package siat

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Environment string

const (
	EnvironmentPiloto     Environment = "PILOTO"
	EnvironmentProduccion Environment = "PRODUCCION"
)

// Service identifica los servicios SOAP del SIAT.
type Service string

const (
	ServiceCodigos        Service = "FacturacionCodigos"
	ServiceOperaciones    Service = "FacturacionOperaciones"
	ServiceSincronizacion Service = "FacturacionSincronizacion"
)

func (s Service) String() string { return string(s) }

type Config struct {
	Environment Environment
	WSDLURL     string
	EndpointURL string
	Timeout     time.Duration
	Headers     map[string]string
}

func DefaultConfig(environment Environment) Config {
	baseURL := "https://pilotosiatservicios.impuestos.gob.bo/v2"
	if environment == EnvironmentProduccion {
		baseURL = "https://siat.impuestos.gob.bo/v2"
	}

	return Config{
		Environment: environment,
		WSDLURL:     baseURL + "/" + ServiceCodigos.String() + "?wsdl",
		EndpointURL: baseURL + "/" + ServiceCodigos.String(),
		Timeout:     30 * time.Second,
		Headers:     map[string]string{},
	}
}

// ServiceEndpoint devuelve la URL base (endpoint SOAP) para un servicio del SIAT
// según el ambiente configurado.
func (c Config) ServiceEndpoint(service Service) string {
	if c.Environment == EnvironmentProduccion {
		return "https://siat.impuestos.gob.bo/v2/" + string(service)
	}
	return "https://pilotosiatservicios.impuestos.gob.bo/v2/" + string(service)
}

func (c Config) Validate() error {
	if c.WSDLURL == "" && c.EndpointURL == "" {
		return fmt.Errorf("siat config: WSDLURL or EndpointURL must be provided")
	}

	if c.EndpointURL != "" {
		if _, err := url.ParseRequestURI(c.EndpointURL); err != nil {
			return fmt.Errorf("siat config: invalid endpoint URL: %w", err)
		}
	}

	if c.WSDLURL != "" {
		if _, err := url.ParseRequestURI(c.WSDLURL); err != nil {
			return fmt.Errorf("siat config: invalid WSDL URL: %w", err)
		}
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("siat config: timeout must be greater than zero")
	}

	return nil
}

func (c Config) EffectiveEndpoint() string {
	if strings.TrimSpace(c.EndpointURL) != "" {
		return c.EndpointURL
	}

	if strings.HasSuffix(c.WSDLURL, "?wsdl") {
		return strings.TrimSuffix(c.WSDLURL, "?wsdl")
	}

	return c.WSDLURL
}

func (c Config) CloneHeaders() map[string]string {
	cloned := make(map[string]string, len(c.Headers))
	for key, value := range c.Headers {
		cloned[key] = value
	}
	return cloned
}
