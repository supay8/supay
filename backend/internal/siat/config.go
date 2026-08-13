package siat

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
)

// Alias de constantes del SDK go-siat para uso del resto de la aplicación.
const (
	AmbienteProduccion     = goSiat.AmbienteProduccion
	AmbientePruebas        = goSiat.AmbientePruebas
	ModalidadElectronica   = goSiat.ModalidadElectronica
	ModalidadComputarizada = goSiat.ModalidadComputarizada
)

// Config agrupa la identidad global del contribuyente y la configuración de
// conexión usada por el adaptador sobre el SDK go-siat.
type Config struct {
	// Token de autenticación proporcionado por el SIAT (obligatorio).
	Token string

	// Nit del contribuyente emisor (obligatorio).
	Nit int64

	// CodigoSistema autorizado por el SIN (obligatorio).
	CodigoSistema string

	// CodigoAmbiente: 1 = producción, 2 = piloto/pruebas.
	CodigoAmbiente int

	// BaseURL del SIAT (p.ej. https://pilotosiatservicios.impuestos.gob.bo/v2).
	BaseURL string

	// TraceId para correlacionar solicitudes (opcional).
	TraceId string

	// UserAgent personalizado (opcional).
	UserAgent string

	// HTTPClient personalizado (opcional).
	HTTPClient *http.Client

	// Timeout usado si no se provee un HTTPClient (opcional, default 45s).
	Timeout time.Duration

	// Credenciales de firma digital (PEM cert/key o P12). Opcional: se
	// requieren solo para emitir facturas en modalidad electrónica.
	CertPemCert string
	CertPemKey  string
	CertP12     string
	CertP12Pass string
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("siat config: Token es obligatorio")
	}
	if c.Nit <= 0 {
		return fmt.Errorf("siat config: Nit es obligatorio y debe ser mayor a cero")
	}
	if strings.TrimSpace(c.CodigoSistema) == "" {
		return fmt.Errorf("siat config: CodigoSistema es obligatorio")
	}
	if c.CodigoAmbiente != AmbienteProduccion && c.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat config: CodigoAmbiente inválido (%d)", c.CodigoAmbiente)
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("siat config: BaseURL es obligatorio")
	}
	return nil
}
