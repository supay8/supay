package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
)

type Config struct {
	Port string
	// SIAT y SiatModalidad se mantienen por compatibilidad pero están
	// desacoplados: ya no se valida NIT/Token/Cert al iniciar. El
	// SiatClientProvider resuelve credenciales por CompanyId.
	// Deprecated: no usar en código nuevo; preferir SiatInfra.
	SIAT          siat.Config `json:"-"`
	SiatModalidad int         `json:"-"`

	// SiatInfra contiene solo parámetros de infraestructura compartida.
	SiatInfra     SiatInfraConfig
	BackendSecret string
	// APIKey protege la API HTTP: todas las rutas (excepto /health) exigen el
	// header X-API-Key con este valor. Vacío deshabilita la protección.
	APIKey string
	// EncryptionKey es la llave maestra AES-GCM para cifrar tokens y P12 por empresa.
	EncryptionKey        string
	DeploymentMode       string // selfhosted | cloud
	StorageDriver        string // none | local | r2
	StoragePath          string // base path para driver local
	R2                   R2Config
	AllowCustomIssueDate bool // dev-only: permite POST /invoices con issue_date arbitrario
	Maintenance          MaintenanceConfig
}

// SiatInfraConfig retiene solo infra compartida, sin credenciales por empresa.
type SiatInfraConfig struct {
	BaseURL        string
	CodigoAmbiente int
	Timeout        time.Duration
	TraceId        string
	UserAgent      string
	Modalidad      int // default fallback, override por certificado/empresa
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

// MaintenanceConfig controla el scheduler liviano de la fase 8. Todos los
// intervalos son configurables para poder acelerar pruebas y despliegues.
type MaintenanceConfig struct {
	Enabled               bool
	CredentialInterval    time.Duration
	CertificateInterval   time.Duration
	JobTimeout            time.Duration
	CufdRenewalLead       time.Duration
	CuisRenewalLead       time.Duration
	CuisFallbackValidity  time.Duration
	NotificationTimeout   time.Duration
	CertificateWebhookURL string
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
	BackendSecret := strings.TrimSpace(os.Getenv("BACKEND_SECRET"))
	// Infra compartida: sin validar credenciales por empresa
	modalidad := parseInt(getEnv("SIAT_MODALIDAD", "1"), siat.ModalidadElectronica)
	if modalidad != siat.ModalidadElectronica && modalidad != siat.ModalidadComputarizada {
		modalidad = siat.ModalidadElectronica
	}
	siatInfra := SiatInfraConfig{
		BaseURL:        baseURL,
		CodigoAmbiente: ambiente,
		Timeout:        parseDuration(getEnv("SIAT_TIMEOUT", "45s"), 45*time.Second),
		TraceId:        strings.TrimSpace(os.Getenv("SIAT_TRACE_ID")),
		UserAgent:      strings.TrimSpace(os.Getenv("SIAT_USER_AGENT")),
		Modalidad:      modalidad,
	}

	// Compatibilidad: SIAT legado desde env solo para advertencia, no bloquea arranque
	legacyToken := strings.TrimSpace(os.Getenv("SIAT_TOKEN_DELEGADO"))
	legacyNit := strings.TrimSpace(os.Getenv("SIAT_NIT"))
	legacySistema := strings.TrimSpace(os.Getenv("SIAT_CODIGO_SISTEMA"))
	legacyP12 := strings.TrimSpace(os.Getenv("SIAT_CERT_P12"))
	if legacyToken != "" || legacyNit != "" || legacySistema != "" || legacyP12 != "" {
		log.Printf("⚠️ Variables SIAT_* legacy detectadas (SIAT_NIT/TOKEN/CERT). Serán ignoradas: configure credenciales por empresa via API/certificates (multi-tenant).")
	}
	// SIAT deprecated vacío (solo para no romper callers antiguos que leen cfg.SIAT.BaseURL)
	siatConfig := siat.Config{
		CodigoAmbiente: ambiente,
		BaseURL:        baseURL,
		TraceId:        siatInfra.TraceId,
		UserAgent:      siatInfra.UserAgent,
		Timeout:        siatInfra.Timeout,
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

	allowCustomIssueDate := parseBoolEnv("ALLOW_CUSTOM_ISSUE_DATE", ambiente == siat.AmbientePruebas)
	maintenance := MaintenanceConfig{
		Enabled:               parseBoolEnv("MAINTENANCE_ENABLED", true),
		CredentialInterval:    parseDuration(getEnv("CREDENTIAL_RENEWAL_INTERVAL", "1h"), time.Hour),
		CertificateInterval:   parseDuration(getEnv("CERTIFICATE_CHECK_INTERVAL", "24h"), 24*time.Hour),
		JobTimeout:            parseDuration(getEnv("MAINTENANCE_JOB_TIMEOUT", "5m"), 5*time.Minute),
		CufdRenewalLead:       parseDuration(getEnv("CUFD_RENEWAL_LEAD", "4h"), 4*time.Hour),
		CuisRenewalLead:       parseDuration(getEnv("CUIS_RENEWAL_LEAD", "720h"), 30*24*time.Hour),
		CuisFallbackValidity:  parseDuration(getEnv("CUIS_FALLBACK_VALIDITY", "8760h"), 365*24*time.Hour),
		NotificationTimeout:   parseDuration(getEnv("CERTIFICATE_NOTIFICATION_TIMEOUT", "10s"), 10*time.Second),
		CertificateWebhookURL: strings.TrimSpace(os.Getenv("CERTIFICATE_ALERT_WEBHOOK_URL")),
	}

	encryptionKey := strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))
	if encryptionKey == "" {
		encryptionKey = strings.TrimSpace(os.Getenv("MASTER_ENCRYPTION_KEY"))
	}
	if encryptionKey == "" {
		encryptionKey = strings.TrimSpace(os.Getenv("SIAT_ENCRYPTION_KEY"))
	}
	// No panic aquí; el provider validará al instanciar crypto.Service y logueará warning.
	if encryptionKey == "" {
		log.Printf("⚠️ ENCRYPTION_KEY no configurada: el cifrado de credenciales SIAT quedará deshabilitado hasta configurarla (requerida para multi-tenant seguro)")
	}

	return Config{
		Port:                 getEnv("PORT", "8081"),
		SIAT:                 siatConfig,
		SiatModalidad:        modalidad,
		SiatInfra:            siatInfra,
		APIKey:               strings.TrimSpace(os.Getenv("API_KEY")),
		EncryptionKey:        encryptionKey,
		DeploymentMode:       deploymentMode,
		StorageDriver:        storageDriver,
		StoragePath:          storagePath,
		R2:                   r2Cfg,
		BackendSecret:        BackendSecret,
		AllowCustomIssueDate: allowCustomIssueDate,
		Maintenance:          maintenance,
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

func parseBoolEnv(key string, fallback bool) bool {
	if raw, ok := os.LookupEnv(key); ok {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		if trimmed == "true" || trimmed == "1" || trimmed == "yes" {
			return true
		}
		if trimmed == "false" || trimmed == "0" || trimmed == "no" {
			return false
		}
	}
	return fallback
}
