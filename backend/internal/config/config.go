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
	environment := siat.Environment(strings.ToUpper(strings.TrimSpace(getEnv("SIAT_ENVIRONMENT", ""))))
	if environment != siat.EnvironmentProduccion {
		environment = siat.EnvironmentPiloto
	}
	// SIAT_AMBIENTE (1 = producción, 2 = piloto) tiene prioridad sobre
	// SIAT_ENVIRONMENT.
	switch strings.TrimSpace(getEnv("SIAT_AMBIENTE", "2")) {
	case "1":
		environment = siat.EnvironmentProduccion
	case "2":
		environment = siat.EnvironmentPiloto
	}

	siATConfig := siat.DefaultConfig(environment)
	siATConfig.Timeout = parseDuration(getEnv("SIAT_TIMEOUT", "30s"), 30*time.Second)
	siATConfig.Headers = map[string]string{}

	if token := strings.TrimSpace(os.Getenv("SIAT_TOKEN_DELEGADO")); token != "" {
		siATConfig.Headers["apikey"] = "TokenApi " + token
	}

	if wsdlURL := strings.TrimSpace(os.Getenv("SIAT_WSDL_URL")); wsdlURL != "" {
		siATConfig.WSDLURL = wsdlURL
	}
	if endpointURL := strings.TrimSpace(os.Getenv("SIAT_ENDPOINT_URL")); endpointURL != "" {
		siATConfig.EndpointURL = endpointURL
	}

	return Config{
		Port:          getEnv("PORT", "8080"),
		SIAT:          siATConfig,
		SiatModalidad: parseInt(getEnv("SIAT_MODALIDAD", "1"), 1),
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
