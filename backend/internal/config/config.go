package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/siat"
)

type Config struct {
	Port          string
	SIAT          siat.Config
	SiatModalidad int
}

func Load() Config {
	ambiente := parseInt(getEnv("SIAT_AMBIENTE", "2"), siat.AmbientePruebas)
	if ambiente != siat.AmbienteProduccion {
		ambiente = siat.AmbientePruebas
	}

	baseURL := strings.TrimSpace(os.Getenv("SIAT_BASE_URL"))
	if baseURL == "" {
		if ambiente == siat.AmbienteProduccion {
			baseURL = "https://siat.impuestos.gob.bo/v2"
		} else {
			baseURL = "https://pilotosiatservicios.impuestos.gob.bo/v2"
		}
	}

	siatConfig := siat.Config{
		Token:          strings.TrimSpace(os.Getenv("SIAT_TOKEN_DELEGADO")),
		Nit:            parseInt64(getEnv("SIAT_NIT", ""), 0),
		CodigoSistema:  strings.TrimSpace(os.Getenv("SIAT_CODIGO_SISTEMA")),
		CodigoAmbiente: ambiente,
		BaseURL:        baseURL,
		TraceId:        strings.TrimSpace(os.Getenv("SIAT_TRACE_ID")),
		UserAgent:      strings.TrimSpace(os.Getenv("SIAT_USER_AGENT")),
		Timeout:        parseDuration(getEnv("SIAT_TIMEOUT", "45s"), 45*time.Second),
		CertPemCert:    strings.TrimSpace(os.Getenv("SIAT_CERT_PEM_CERT")),
		CertPemKey:     strings.TrimSpace(os.Getenv("SIAT_CERT_PEM_KEY")),
		CertP12:        strings.TrimSpace(os.Getenv("SIAT_CERT_P12")),
		CertP12Pass:    strings.TrimSpace(os.Getenv("SIAT_CERT_P12_PASSWORD")),
	}

	modalidad := parseInt(getEnv("SIAT_MODALIDAD", "1"), siat.ModalidadElectronica)

	return Config{
		Port:          getEnv("PORT", "8081"),
		SIAT:          siatConfig,
		SiatModalidad: modalidad,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func parseInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
