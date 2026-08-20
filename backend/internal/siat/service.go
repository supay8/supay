package siat

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
)

// Service es un adaptador delgado sobre el SDK go-siat v2 que expone solo la
// gestión de códigos (CUIS y CUFD) usada por la aplicación.
type Service struct {
	sdk *goSiat.SiatServices
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
	if resp == nil || resp.Body.Content.RespuestaVerificarNit.Transaccion {
		return true, nil
	}
	return false, nil
}

// buildCredentialSign construye la credencial de firma digital a partir de la
// configuración. Prioriza P12 si se provee; en caso contrario usa el par
// PEM cert/key. Devuelve una credencial vacía si no hay datos.
func buildCredentialSign(cfg Config) goSiat.CredentialSign {
	if strings.TrimSpace(cfg.CertP12) != "" {
		return goSiat.NewP12Credential(strings.TrimSpace(cfg.CertP12), cfg.CertP12Pass)
	}
	if strings.TrimSpace(cfg.CertPemCert) != "" && strings.TrimSpace(cfg.CertPemKey) != "" {
		return goSiat.NewPEMCredential(strings.TrimSpace(cfg.CertPemCert), strings.TrimSpace(cfg.CertPemKey))
	}
	return goSiat.CredentialSign{}
}

// SolicitarCUIS solicita un CUIS al SIAT usando el SDK go-siat.
func (s *Service) SolicitarCUIS(ctx context.Context, req SolicitudCuis) (*RespuestaCuis, error) {
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
	if err := goSiat.Verify(resp.Body.Content.RespuestaCuis); err != nil {
		return nil, fmt.Errorf("siat cuis: %w", err)
	}

	result := resp.Body.Content.RespuestaCuis
	return &RespuestaCuis{
		Codigo:        result.Codigo,
		FechaVigencia: XMLDateTime{Time: SIATWallClockToInstant(result.FechaVigencia)},
		Transaccion:   result.Transaccion,
		Mensajes:      toMensajes(result.MensajesList),
	}, nil
}

// SolicitarCUFD solicita un CUFD al SIAT usando el SDK go-siat.
func (s *Service) SolicitarCUFD(ctx context.Context, req SolicitudCufd) (*RespuestaCufd, error) {
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
	cfg := base
	if ambiente > 0 {
		cfg.CodigoAmbiente = ambiente
	}
	if strings.TrimSpace(sistema) != "" {
		cfg.CodigoSistema = sistema
	}
	if strings.TrimSpace(nit) != "" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(nit), 10, 64); err == nil && parsed > 0 {
			cfg.Nit = parsed
		}
	}
	return goSiat.WithDynamicConfig(ctx, cfg)
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
