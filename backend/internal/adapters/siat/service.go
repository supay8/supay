package siat

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
	"golang.org/x/crypto/pkcs12"
	pkcs12Modern "software.sslmate.com/src/go-pkcs12"
)

// Service es un adaptador delgado sobre el SDK go-siat v2 que expone solo la
// gestión de códigos (CUIS y CUFD) usada por la aplicación.
type Service struct {
	sdk *goSiat.SiatServices
}

// CodigoSistema expone el CodigoSistema configurado (para usecase sincronización).
func (s *Service) CodigoSistema() string {
	if s == nil || s.sdk == nil {
		return ""
	}
	return s.sdk.Config().CodigoSistema
}

// CodigoAmbiente expone el ambiente configurado.
func (s *Service) CodigoAmbiente() int {
	if s == nil || s.sdk == nil {
		return 0
	}
	return s.sdk.Config().CodigoAmbiente
}

// NewService construye el adaptador validando la configuración global.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 45 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	sdk, err := goSiat.New(goSiat.Config{
		Token:          cfg.Token,
		Nit:            cfg.Nit,
		CodigoSistema:  cfg.CodigoSistema,
		CodigoAmbiente: cfg.CodigoAmbiente,
		BaseURL:        cfg.BaseURL,
		TraceId:        cfg.TraceId,
		UserAgent:      cfg.UserAgent,
		HTTPClient:     httpClient,
		CredentialSign: buildCredentialSign(cfg),
	})
	if err != nil {
		return nil, fmt.Errorf("siat: %w", err)
	}

	return &Service{sdk: sdk}, nil
}

// VerificarNit verifica un NIT contra el SIAT antes de emitir facturas.
// Devuelve true si el NIT es válido, false si no.
func (s *Service) VerificarNit(ctx context.Context, nit string, cuis string, codigoAmbiente, codigoSucursal, codigoModalidad int) (bool, error) {
	if s.sdk == nil {
		return false, fmt.Errorf("siat verificar nit: servicio SIAT no inicializado")
	}
	nitInt := parseNit(nit)
	if nitInt <= 0 {
		return false, fmt.Errorf("siat verificar nit: NIT inválido %q", nit)
	}

	request := models.NewVerificarNitBuilder().
		WithCodigoSucursal(codigoSucursal).
		WithCuis(cuis).
		WithNitParaVerificacion(nitInt).
		WithCodigoModalidad(codigoModalidad).
		Build()

	resp, err := s.sdk.Codigos().VerificarNit(ctx, request)
	if err != nil {
		return false, fmt.Errorf("siat verificar nit: %w", err)
	}

	// Verificar la respuesta
	if resp == nil {
		return false, fmt.Errorf("siat verificar nit: respuesta vacía del servicio")
	}
	return resp.Body.Content.RespuestaVerificarNit.Transaccion, nil
}

// buildCredentialSign construye la credencial de firma digital a partir de la
// configuración. Prioriza P12 si se provee; en caso contrario usa el par
// PEM cert/key. Devuelve una credencial vacía si no hay datos.
//
// Fix definitivo para "pkcs12: expected exactly two safe bags": el SDK
// go-siat usa x/crypto/pkcs12.Decode() que exige exactamente 2 bags
// (1 key + 1 cert). Muchos emisores entregan P12 con cadena (3+ bags) y
// fallan. Aquí convertimos cualquier P12 válido (N bags) a PEM vía
// pkcs12.ToPEM() y entregamos PEM al SDK, que no tiene esa restricción.
func buildCredentialSign(cfg Config) goSiat.CredentialSign {
	if len(cfg.CertP12Bytes) > 0 {
		data := normalizeP12DER(cfg.CertP12Bytes)
		if certPEM, keyPEM, err := flexibleP12ToPEM(data, cfg.CertP12Pass); err == nil {
			return goSiat.NewPEMCredential(certPEM, keyPEM)
		}
		// Fallback: deja que el SDK reporte el error original (password, formato)
		return goSiat.NewP12Credential(data, cfg.CertP12Pass)
	}
	if p12 := strings.TrimSpace(cfg.CertP12); p12 != "" {
		// go-siat interpreta un string como ruta de archivo y []byte como el
		// contenido del P12. Aceptar Base64 aquí evita que el SDK intente abrir
		// ese contenido como si fuera un nombre de archivo.
		if decoded, err := base64.StdEncoding.DecodeString(p12); err == nil && len(decoded) > 0 {
			data := normalizeP12DER(decoded)
			if certPEM, keyPEM, err := flexibleP12ToPEM(data, cfg.CertP12Pass); err == nil {
				return goSiat.NewPEMCredential(certPEM, keyPEM)
			}
			return goSiat.NewP12Credential(data, cfg.CertP12Pass)
		}
		// p12 es ruta de archivo: intentar flexible leyendo el archivo
		if data, err := os.ReadFile(p12); err == nil {
			data = normalizeP12DER(data)
			if certPEM, keyPEM, err := flexibleP12ToPEM(data, cfg.CertP12Pass); err == nil {
				return goSiat.NewPEMCredential(certPEM, keyPEM)
			}
		}
		return goSiat.NewP12Credential(p12, cfg.CertP12Pass)
	}
	if strings.TrimSpace(cfg.CertPemCert) != "" && strings.TrimSpace(cfg.CertPemKey) != "" {
		return goSiat.NewPEMCredential(strings.TrimSpace(cfg.CertPemCert), strings.TrimSpace(cfg.CertPemKey))
	}
	return goSiat.CredentialSign{}
}

// flexibleP12ToPEM convierte cualquier P12 válido (1 key + N certs) a PEM
// usando pkcs12.ToPEM que no exige 2 bags. Retorna cert PEM + key PEM
// listos para NewPEMCredential. Si hay cadena, elige el cert cuyo public key
// coincide con la private key; si no, usa el primero.
//
// Soporta tanto P12 legacy (3DES/RC2, x/crypto) como modernos (PBES2/AES-256
// generados por OpenSSL 3+), probando x/crypto primero y fallback a
// go-pkcs12 (sslmate) que implementa AES.
func flexibleP12ToPEM(p12Data []byte, password string) ([]byte, []byte, error) {
	blocks, err := pkcs12.ToPEM(p12Data, password)
	if err != nil {
		// Fallback moderno: OpenSSL 3+ usa PBES2/AES-256 que x/crypto no soporta
		if blocks2, err2 := pkcs12Modern.ToPEM(p12Data, password); err2 == nil {
			blocks = blocks2
			err = nil
		} else {
			// Si ambos fallan, retornar el error original (más reconocido)
			// pero si el moderno tuvo éxito parcial, ya lo usamos
			if err2 != nil && err != nil {
				// Preferir error de x/crypto si es "expected exactly two safe bags" es más claro,
				// pero para AES es "unknown digest" - en ese caso el fallback ya habría sucedido
				return nil, nil, err
			}
			return nil, nil, err
		}
	}
	var certBlocks []*pem.Block
	var keyBlocks []*pem.Block
	for _, b := range blocks {
		switch b.Type {
		case "CERTIFICATE":
			certBlocks = append(certBlocks, b)
		case "PRIVATE KEY":
			keyBlocks = append(keyBlocks, b)
		}
	}
	if len(keyBlocks) == 0 {
		return nil, nil, fmt.Errorf("pkcs12: private key missing")
	}
	if len(certBlocks) == 0 {
		return nil, nil, fmt.Errorf("pkcs12: certificate missing")
	}

	keyPEM := pem.EncodeToMemory(keyBlocks[0])

	// Si hay cadena, intentar matchear cert con la key
	certBlock := certBlocks[0]
	if len(certBlocks) > 1 {
		if matched := selectMatchingCert(keyBlocks[0], certBlocks); matched != nil {
			certBlock = matched
		}
	}
	certPEM := pem.EncodeToMemory(certBlock)

	// Validación temprana: que el cert no esté expirado (mismo check que hace el SDK)
	if cert, err := x509.ParseCertificate(certBlock.Bytes); err == nil {
		_ = cert // el SDK valida expiración en SignXMLBytes; no fallamos aquí
	}

	return certPEM, keyPEM, nil
}

func selectMatchingCert(keyBlock *pem.Block, certBlocks []*pem.Block) *pem.Block {
	// Parsear la private key para obtener la public key y compararla con cada cert
	privKey, err := parsePrivateKeyForMatch(keyBlock.Bytes)
	if err != nil || privKey == nil {
		return nil
	}
	for _, cb := range certBlocks {
		cert, err := x509.ParseCertificate(cb.Bytes)
		if err != nil {
			continue
		}
		if publicKeysEqual(privKey, cert.PublicKey) {
			return cb
		}
	}
	return nil
}

func parsePrivateKeyForMatch(der []byte) (interface{}, error) {
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k, nil
	}
	return nil, fmt.Errorf("unknown private key type")
}

func publicKeysEqual(priv interface{}, pub interface{}) bool {
	switch k := priv.(type) {
	case *x509.Certificate:
		return false
	default:
		// Comparar DER de la public key
		_ = k
	}
	// Método genérico: marshalar ambas public keys y comparar
	privPub := extractPublicKey(priv)
	if privPub == nil || pub == nil {
		return false
	}
	a, err1 := x509.MarshalPKIXPublicKey(privPub)
	b, err2 := x509.MarshalPKIXPublicKey(pub)
	if err1 != nil || err2 != nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func extractPublicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case interface{ Public() interface{} }:
		return k.Public()
	default:
		return nil
	}
}

// normalizeP12DER elimina únicamente bytes de relleno ubicados después del
// objeto ASN.1 exterior del PFX. Algunos proveedores entregan archivos P12
// rellenados con NUL hasta un tamaño fijo; OpenSSL los tolera, pero el parser
// estricto usado por go-siat devuelve "pkcs12: trailing data found".
func normalizeP12DER(data []byte) []byte {
	if len(data) < 2 || data[0] != 0x30 { // PFX ::= SEQUENCE
		return data
	}

	headerLen := 2
	contentLen := int(data[1])
	if data[1]&0x80 != 0 {
		lengthBytes := int(data[1] & 0x7f)
		if lengthBytes == 0 || lengthBytes > 4 || len(data) < 2+lengthBytes {
			return data
		}
		headerLen += lengthBytes
		contentLen = 0
		for _, b := range data[2:headerLen] {
			contentLen = contentLen<<8 | int(b)
		}
	}

	totalLen := headerLen + contentLen
	if totalLen <= 0 || totalLen >= len(data) {
		return data
	}
	for _, b := range data[totalLen:] {
		if b != 0x00 && b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			return data
		}
	}

	normalized := make([]byte, totalLen)
	copy(normalized, data[:totalLen])
	return normalized
}

// cuisYaVigente detecta el mensaje 980 del SIAT ("EXISTE UN CUIS VIGENTE PARA
// LA SUCURSAL O PUNTO DE VENTA"). En ese caso el SIAT responde
// transaccion=false pero incluye el CUIS vigente en <codigo>: no es un fallo,
// es la reemisión del código existente.
func cuisYaVigente(err error) bool {
	var siatErr *goSiat.SiatError
	return errors.As(err, &siatErr) && siatErr.SiatCode == goSiat.CodeExisteCuisVigente
}

// SolicitarCUIS solicita un CUIS al SIAT usando el SDK go-siat.
func (s *Service) SolicitarCUIS(ctx context.Context, req SolicitudCuis) (*RespuestaCuis, error) {
	if err := applyIdentityValues(s.sdk.Config(), &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	request := models.NewCuisBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoModalidad(req.CodigoModalidad).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Codigos().SolicitudCuis(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat cuis: %w", err)
	}

	result := resp.Body.Content.RespuestaCuis

	if err := goSiat.Verify(result); err != nil {
		// 980 con código presente: éxito — el propio SIAT devuelve el CUIS
		// vigente en la misma respuesta; se normaliza como transacción OK y
		// se conservan los mensajes para trazabilidad.
		if !cuisYaVigente(err) || result.Codigo == "" {
			return nil, fmt.Errorf("siat cuis: %w", err)
		}
		result.Transaccion = true
	}

	return &RespuestaCuis{
		Codigo:        result.Codigo,
		FechaVigencia: XMLDateTime{Time: SIATWallClockToInstant(result.FechaVigencia)},
		Transaccion:   result.Transaccion,
		Mensajes:      toMensajes(result.MensajesList),
	}, nil
}

// SolicitarCUFD solicita un CUFD al SIAT usando el SDK go-siat.
func (s *Service) SolicitarCUFD(ctx context.Context, req SolicitudCufd) (*RespuestaCufd, error) {
	if err := applyIdentityValues(s.sdk.Config(), &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	request := models.NewCufdBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoModalidad(req.CodigoModalidad).
		WithCuis(req.Cuis).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Codigos().SolicitudCufd(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat cufd: %w", err)
	}
	if err := goSiat.Verify(resp.Body.Content.RespuestaCufd); err != nil {
		return nil, fmt.Errorf("siat cufd: %w", err)
	}

	result := resp.Body.Content.RespuestaCufd
	return &RespuestaCufd{
		Codigo:        result.Codigo,
		CodigoControl: result.CodigoControl,
		Direccion:     result.Direccion,
		FechaVigencia: XMLDateTime{Time: SIATWallClockToInstant(result.FechaVigencia)},
		Transaccion:   result.Transaccion,
		Mensajes:      toMensajes(result.MensajesList),
	}, nil
}

// withDynamicConfig sobreescribe la identidad del contribuyente (NIT, sistema,
// ambiente) por empresa en el contexto de la petición, sin tocar la config global.
func withDynamicConfig(ctx context.Context, base goSiat.Config, ambiente int, sistema, nit string) context.Context {
	// The SDK identity is configured once in goSiat.Config. Keep the arguments
	// for source compatibility with older callers, but never replace the global
	// identity per request.
	return goSiat.WithDynamicConfig(ctx, base)
}

func applyIdentityValues(cfg goSiat.Config, ambiente *int, sistema *string, nit *string) error {
	if *ambiente == 0 {
		*ambiente = cfg.CodigoAmbiente
	} else if *ambiente != cfg.CodigoAmbiente {
		return fmt.Errorf("siat identidad: codigoAmbiente de la solicitud (%d) no coincide con Config (%d)", *ambiente, cfg.CodigoAmbiente)
	}
	if strings.TrimSpace(*sistema) == "" {
		*sistema = cfg.CodigoSistema
	} else if strings.TrimSpace(*sistema) != strings.TrimSpace(cfg.CodigoSistema) {
		return fmt.Errorf("siat identidad: codigoSistema de la solicitud no coincide con Config")
	}
	if strings.TrimSpace(*nit) == "" {
		*nit = strconv.FormatInt(cfg.Nit, 10)
	} else if parsed := parseNit(*nit); parsed != cfg.Nit {
		return fmt.Errorf("siat identidad: nit de la solicitud no coincide con Config")
	}
	return nil
}

func (s *Service) applyIdentity(req *SolicitudFactura) error {
	cfg := s.sdk.Config()
	return applyIdentityValues(cfg, &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit)
}

func (s *Service) applyDocumentIdentity(req *SolicitudDocumento) error {
	cfg := s.sdk.Config()
	return applyIdentityValues(cfg, &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit)
}

// toMensajes convierte la lista de mensajes del SIAT (tipo interno del SDK) a
// la representación propia de la aplicación mediante reflexión, ya que el
// paquete interno del SDK no puede importarse por nombre.
func toMensajes(msgs any) []Mensaje {
	v := reflect.ValueOf(msgs)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil
	}
	out := make([]Mensaje, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		var m Mensaje
		if f := elem.FieldByName("Codigo"); f.IsValid() {
			m.Codigo = int(f.Int())
		}
		if f := elem.FieldByName("Descripcion"); f.IsValid() {
			m.Descripcion = f.String()
		}
		out = append(out, m)
	}
	return out
}
