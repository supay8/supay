package siat

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// testSignerAndCert genera una llave RSA + certificado autofirmado para probar
// el Signer sin depender de un certificado real.
func testSignerAndCert(t *testing.T) (*Signer, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generar llave: %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "supay-test"},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("crear certificado: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parsear certificado: %v", err)
	}
	return &Signer{privateKey: key, certs: []*x509.Certificate{cert}, certBytes: certDER}, cert
}

func TestSigner_SignYValida(t *testing.T) {
	signer, cert := testSignerAndCert(t)

	input := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<facturaElectronicaCompraVenta>
  <cabecera>
    <cuf>CUF-1</cuf>
    <montoTotal>100.00</montoTotal>
  </cabecera>
</facturaElectronicaCompraVenta>`)

	signed, err := signer.SignXML(input)
	if err != nil {
		t.Fatalf("SignXML: %v", err)
	}

	out := string(signed)
	if !strings.Contains(out, "standalone=\"no\"") {
		t.Errorf("la declaración XML no incluye standalone=\"no\":\n%s", out)
	}
	if !strings.Contains(out, "Signature") {
		t.Errorf("el XML firmado no contiene Signature")
	}

	// Validar la firma (RSA-SHA256, enveloped) con goxmldsig.
	ctx := dsig.NewDefaultValidationContext(&dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}})
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(signed); err != nil {
		t.Fatalf("parsear XML firmado: %v", err)
	}
	if _, err := ctx.Validate(doc.Root()); err != nil {
		t.Fatalf("la firma no valida: %v", err)
	}

	// La firma debe ser enveloped: Signature como hijo del documento raíz.
	root := doc.Root()
	found := false
	for _, el := range root.ChildElements() {
		if el.Tag == "Signature" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no se encontró Signature directo bajo la raíz")
	}
}

func TestSigner_Nil(t *testing.T) {
	var s *Signer
	if _, err := s.SignXML([]byte("<a/>")); err == nil {
		t.Fatal("se esperaba error con Signer nil")
	}
	s = &Signer{}
	if _, err := s.SignXML([]byte("<a/>")); err == nil {
		t.Fatal("se esperaba error sin llave privada")
	}
}

func TestNewSignerFromConfig_SinCertificado(t *testing.T) {
	if _, err := NewSignerFromConfig(Config{}); err == nil {
		t.Fatal("se esperaba error sin certificado configurado")
	}
}

func TestEnsureStandaloneNo(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"declaración estándar",
			"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<raiz/>",
			"<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n<raiz/>",
		},
		{
			"sin declaración",
			"<raiz/>",
			"<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n<raiz/>",
		},
		{
			"declaración con otro orden",
			"<?xml version=\"1.0\"?><raiz/>",
			"<?xml version=\"1.0\" standalone=\"no\"?><raiz/>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(ensureStandaloneNo([]byte(tt.in)))
			if got != tt.want {
				t.Fatalf("ensureStandaloneNo\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}
