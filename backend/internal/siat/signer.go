package siat

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"golang.org/x/crypto/pkcs12"
)

// Signer firma documentos XML con XMLDSig (RSA-SHA256, firma enveloped,
// canonicalización C14N 1.0 con comentarios) tal como exige el SIN para la
// modalidad Electrónica en Línea.
type Signer struct {
	privateKey *rsa.PrivateKey
	certs      []*x509.Certificate
	certBytes  []byte
}

// NewSignerFromConfig construye un Signer a partir de la configuración SIAT.
// Soporta PKCS#12 (.p12/.pfx) vía CertPath+CertPassword o PEM vía
// CertPEMCert+CertPEMKey.
func NewSignerFromConfig(cfg Config) (*Signer, error) {
	if cfg.CertPath != "" {
		return NewSignerFromPKCS12(cfg.CertPath, cfg.CertPassword)
	}
	if cfg.CertPEMCert != "" && cfg.CertPEMKey != "" {
		return NewSignerFromPEMFiles(cfg.CertPEMCert, cfg.CertPEMKey)
	}
	return nil, fmt.Errorf("siat signer: no se configuró certificado (SIAT_CERT_PATH o SIAT_CERT_PEM_CERT/SIAT_CERT_PEM_KEY)")
}

// NewSignerFromPKCS12 carga un contenedor PKCS#12 (.p12/.pfx).
func NewSignerFromPKCS12(path, password string) (*Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("siat signer: leer p12: %w", err)
	}
	priv, cert, err := pkcs12.Decode(data, password)
	if err != nil {
		return nil, fmt.Errorf("siat signer: decodificar p12: %w", err)
	}
	rsaKey, ok := priv.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("siat signer: la llave del p12 no es RSA")
	}
	return &Signer{privateKey: rsaKey, certs: []*x509.Certificate{cert}, certBytes: cert.Raw}, nil
}

// NewSignerFromPEMFiles carga certificado + llave privada en formato PEM.
func NewSignerFromPEMFiles(certPath, keyPath string) (*Signer, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("siat signer: leer certificado: %w", err)
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("siat signer: leer llave: %w", err)
	}

	blockCert, _ := pem.Decode(certData)
	if blockCert == nil {
		return nil, fmt.Errorf("siat signer: certificado PEM inválido")
	}
	cert, err := x509.ParseCertificate(blockCert.Bytes)
	if err != nil {
		return nil, fmt.Errorf("siat signer: parsear certificado: %w", err)
	}

	blockKey, _ := pem.Decode(keyData)
	if blockKey == nil {
		return nil, fmt.Errorf("siat signer: llave PEM inválida")
	}
	var key crypto.PrivateKey
	switch blockKey.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(blockKey.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(blockKey.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(blockKey.Bytes)
	default:
		return nil, fmt.Errorf("siat signer: tipo de llave no soportado %q", blockKey.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("siat signer: parsear llave: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("siat signer: se requiere llave RSA")
	}
	return &Signer{privateKey: rsaKey, certs: []*x509.Certificate{cert}, certBytes: cert.Raw}, nil
}

// SignXML firma un documento XML (enveloped, RSA-SHA256) y devuelve el XML con
// la firma incrustada y la declaración standalone="no" (patrón del Firmador
// oficial del SIN).
func (s *Signer) SignXML(xmlBytes []byte) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("siat signer: nil")
	}
	if s.privateKey == nil {
		return nil, fmt.Errorf("siat signer: no hay llave privada configurada")
	}

	ks := &signerKeyStore{privateKey: s.privateKey, certBytes: s.certBytes}
	ctx := dsig.NewDefaultSigningContext(ks)
	ctx.Canonicalizer = dsig.MakeC14N10WithCommentsCanonicalizer()
	ctx.SetSignatureMethod(dsig.RSASHA256SignatureMethod)

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		return nil, fmt.Errorf("siat signer: parsear xml: %w", err)
	}
	if doc.Root() == nil {
		return nil, fmt.Errorf("siat signer: xml sin elemento raíz")
	}

	signed, err := ctx.SignEnveloped(doc.Root())
	if err != nil {
		return nil, fmt.Errorf("siat signer: firmar xml: %w", err)
	}

	out := etree.NewDocument()
	out.SetRoot(signed)

	var buf bytes.Buffer
	if _, err := out.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("siat signer: serializar xml: %w", err)
	}

	return ensureStandaloneNo(buf.Bytes()), nil
}

// signerKeyStore adapta la llave/certificado a la interfaz de goxmldsig.
type signerKeyStore struct {
	privateKey *rsa.PrivateKey
	certBytes  []byte
}

func (ks *signerKeyStore) GetKeyPair() (*rsa.PrivateKey, []byte, error) {
	return ks.privateKey, ks.certBytes, nil
}

// ensureStandaloneNo fuerza la declaración XML standalone="no", tal como la
// produce el Firmador del SIN. La declaración no forma parte del contenido
// firmado, por lo que no invalida la firma.
func ensureStandaloneNo(xmlBytes []byte) []byte {
	decl := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	if bytes.HasPrefix(xmlBytes, decl) {
		return append([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n"), xmlBytes[len(decl):]...)
	}
	s := string(xmlBytes)
	if strings.Contains(s, "<?xml") {
		return []byte(strings.Replace(s, "?>", " standalone=\"no\"?>", 1))
	}
	return append([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n"), xmlBytes...)
}
