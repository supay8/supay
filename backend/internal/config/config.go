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
	// APIKey protege la API HTTP: todas las rutas (excepto /health) exigen el
	// header X-API-Key con este valor. Vacío deshabilita la protección.
	APIKey         string
	DeploymentMode string // selfhosted | cloud
	StorageDriver  string // none | local | r2
	StoragePath    string // base path para driver local
	R2             R2Config
}

// R2Config agrupa credenciales de Cloudflare R2 (solo en modo cloud + r2).
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicURL       string
	Endpoint        string // override opcional, por defecto https://<account>.r2.cloudflarestorage.com
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
	if modalidad != siat.ModalidadElectronica && modalidad != siat.ModalidadComputarizada {
		modalidad = siat.ModalidadElectronica
	}

	if err := siatConfig.Validate(); err != nil {
		panic("configuración SIAT inválida: " + err.Error())
	}

	deploymentMode := parseDeploymentMode(os.Getenv("DEPLOYMENT_MODE"), os.Getenv("SELF_HOSTED"))
	rawStorageDriver := strings.TrimSpace(os.Getenv("STORAGE_DRIVER"))
	var storageDriver string
	if rawStorageDriver == "" {
		// Default condicional: selfhosted -> local (disco), cloud -> r2.
		if deploymentMode == "selfhosted" {
			storageDriver = "local"
		} else {
			storageDriver = "r2"
		}
	} else {
		storageDriver = parseStorageDriver(rawStorageDriver)
	}
	storagePath := strings.TrimSpace(os.Getenv("STORAGE_PATH"))
	if storagePath == "" {
		storagePath = "./storage/pdfs"
	}

	// Self-hosted no debe exigir R2: forzar local si se pide r2 en ese modo.
	if deploymentMode == "selfhosted" && storageDriver == "r2" {
		storageDriver = "local"
	}

	r2Cfg := R2Config{
		AccountID:       strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		Bucket:          strings.TrimSpace(os.Getenv("R2_BUCKET")),
		PublicURL:       strings.TrimSpace(os.Getenv("R2_PUBLIC_URL")),
		Endpoint:        strings.TrimSpace(os.Getenv("R2_ENDPOINT")),
	}
	if r2Cfg.Endpoint == "" && r2Cfg.AccountID != "" {
		r2Cfg.Endpoint = "https://" + r2Cfg.AccountID + ".r2.cloudflarestorage.com"
	}

	return Config{
		Port:           getEnv("PORT", "8081"),
		SIAT:           siatConfig,
		SiatModalidad:  modalidad,
		APIKey:         strings.TrimSpace(os.Getenv("API_KEY")),
		DeploymentMode: deploymentMode,
		StorageDriver:  storageDriver,
		StoragePath:    storagePath,
		R2:             r2Cfg,
	}
}

func parseDeploymentMode(raw, legacySelfHosted string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		// Legacy SELF_HOSTED=true compat.
		if strings.EqualFold(strings.TrimSpace(legacySelfHosted), "true") || strings.EqualFold(strings.TrimSpace(legacySelfHosted), "1") {
			return "selfhosted"
		}
		return "selfhosted"
	}
	if v == "cloud" {
		return "cloud"
	}
	return "selfhosted"
}

func parseStorageDriver(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "local":
		return "local"
	case "r2":
		return "r2"
	case "none":
		return "none"
	case "":
		return "none"
	default:
		return "none"
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
