package siat

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brandsrx/supay/internal/crypto"
	"github.com/brandsrx/supay/internal/domain"
	"golang.org/x/sync/singleflight"
)

// CertStorageReader es la porción de storage.CertStorage que necesita el provider (evita import cycle).
type CertStorageReader interface {
	Get(ctx context.Context, ref string) ([]byte, error)
}

// ProviderInfra contiene solo parámetros de infraestructura compartida
// (sin credenciales por empresa) - evita ciclo con internal/config.
type ProviderInfra struct {
	BaseURL        string
	CodigoAmbiente int
	CodigoSistema  string
	Timeout        time.Duration
	TraceId        string
	UserAgent      string
	Modalidad      int
}

// SiatClientProvider resuelve un *Service configurado por empresa bajo demanda.
// Implementa cache en memoria con TTL y descifrado AES-GCM de credenciales.
type SiatClientProvider interface {
	GetForCompany(ctx context.Context, companyID string) (*Service, error)
	GetForContext(ctx context.Context) (*Service, error)
	Invalidate(companyID string)
}

type provider struct {
	companyRepo domain.CompanyRepository
	certRepo    domain.CertificateRepository
	crypto      *crypto.Service
	certStorage CertStorageReader
	infra       ProviderInfra

	mu    sync.RWMutex
	cache map[string]*cachedEntry
	ttl   time.Duration
	sf    singleflight.Group
}

type cachedEntry struct {
	svc       *Service
	expiresAt time.Time
	companyID string
}

func NewSiatClientProvider(
	companyRepo domain.CompanyRepository,
	certRepo domain.CertificateRepository,
	cryptoSvc *crypto.Service,
	infra ProviderInfra,
) SiatClientProvider {
	if infra.Timeout <= 0 {
		infra.Timeout = 45 * time.Second
	}
	return &provider{
		companyRepo: companyRepo,
		certRepo:    certRepo,
		crypto:      cryptoSvc,
		infra:       infra,
		cache:       make(map[string]*cachedEntry),
		ttl:         10 * time.Minute,
	}
}

// NewSiatClientProviderWithStorage crea el provider con storage abstracto para .p12 (local/r2/memory).
func NewSiatClientProviderWithStorage(
	companyRepo domain.CompanyRepository,
	certRepo domain.CertificateRepository,
	cryptoSvc *crypto.Service,
	certStorage CertStorageReader,
	infra ProviderInfra,
) SiatClientProvider {
	if infra.Timeout <= 0 {
		infra.Timeout = 45 * time.Second
	}
	return &provider{
		companyRepo: companyRepo,
		certRepo:    certRepo,
		crypto:      cryptoSvc,
		certStorage: certStorage,
		infra:       infra,
		cache:       make(map[string]*cachedEntry),
		ttl:         10 * time.Minute,
	}
}

// SetCertStorage inyecta el storage después de construir (para wiring sin ciclo).
func (p *provider) SetCertStorage(s CertStorageReader) { p.certStorage = s }

// GetForContext resuelve usando el CompanyId inyectado en el ctx por el middleware.
func (p *provider) GetForContext(ctx context.Context) (*Service, error) {
	companyID, ok := CompanyIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("siat provider: CompanyId no presente en contexto (middleware no configurado)")
	}
	return p.GetForCompany(ctx, companyID)
}

func (p *provider) GetForCompany(ctx context.Context, companyID string) (*Service, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, fmt.Errorf("siat provider: companyID vacío")
	}
	// Fast path cache
	p.mu.RLock()
	if e, ok := p.cache[companyID]; ok && time.Now().Before(e.expiresAt) && e.svc != nil {
		p.mu.RUnlock()
		return e.svc, nil
	}
	p.mu.RUnlock()

	// singleflight para evitar thundering herd
	v, err, _ := p.sf.Do(companyID, func() (interface{}, error) {
		// double-check after waiting
		p.mu.RLock()
		if e, ok := p.cache[companyID]; ok && time.Now().Before(e.expiresAt) && e.svc != nil {
			p.mu.RUnlock()
			return e.svc, nil
		}
		p.mu.RUnlock()

		svc, err := p.buildServiceForCompany(ctx, companyID)
		if err != nil {
			return nil, err
		}
		p.mu.Lock()
		p.cache[companyID] = &cachedEntry{svc: svc, expiresAt: time.Now().Add(p.ttl), companyID: companyID}
		p.mu.Unlock()
		return svc, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Service), nil
}

func (p *provider) Invalidate(companyID string) {
	p.mu.Lock()
	delete(p.cache, companyID)
	p.mu.Unlock()
}

func (p *provider) buildServiceForCompany(ctx context.Context, companyID string) (*Service, error) {
	company, err := p.companyRepo.GetByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("siat provider: empresa no encontrada %s: %w", companyID, err)
	}
	if strings.TrimSpace(company.Nit) == "" {
		return nil, fmt.Errorf("siat provider: empresa %s sin NIT", companyID)
	}
	if strings.TrimSpace(p.infra.CodigoSistema) == "" {
		return nil, fmt.Errorf("siat provider: SIAT_CODIGO_SISTEMA no configurado")
	}

	// Certificado activo (si existe) aporta token/p12 cifrados y overrides modalidad/ambiente
	var cert *domain.Certificate
	if p.certRepo != nil {
		if c, err := p.certRepo.GetActiveByCompany(companyID); err == nil && c != nil {
			cert = c
		}
	}

	// Resolver token delegado: prioriza cert cifrado, fallback a vacío (error)
	var token string
	if cert != nil && strings.TrimSpace(cert.EncryptedToken) != "" {
		if p.crypto == nil {
			return nil, fmt.Errorf("siat provider: crypto no configurado pero certificado tiene token cifrado")
		}
		plain, err := p.crypto.DecryptString(cert.EncryptedToken)
		if err != nil {
			return nil, fmt.Errorf("siat provider: no se pudo descifrar token delegado: %w", err)
		}
		token = strings.TrimSpace(plain)
	}
	if token == "" {
		// Sin token por empresa: no se puede emitir en electrónica. Se permite
		// construir servicio sin token solo para operaciones que no requieren firma?
		// Por ahora exigir token.
		return nil, fmt.Errorf("siat provider: empresa %s no tiene token delegado configurado (configure via certificates)", companyID)
	}

	// Resolver P12: descifrar password y cargar bytes desde storage ref o cert
	var p12Bytes []byte
	var p12Pass string
	if cert != nil {
		if strings.TrimSpace(cert.EncryptedP12Password) != "" {
			if p.crypto == nil {
				return nil, fmt.Errorf("siat provider: crypto no configurado para password P12")
			}
			pass, err := p.crypto.DecryptString(cert.EncryptedP12Password)
			if err != nil {
				return nil, fmt.Errorf("siat provider: no se pudo descifrar password P12: %w", err)
			}
			p12Pass = pass
		}
		// Cargar bytes P12 desde storageRef o ConfigPath (abstracto local/r2/memory)
		ref := strings.TrimSpace(cert.P12StorageRef)
		if ref == "" {
			ref = strings.TrimSpace(cert.ConfigPath)
		}
		if ref != "" {
			b, err := p.loadP12Bytes(ctx, ref)
			if err != nil {
				slog.Warn("siat provider: no se pudo cargar P12, se intenta sin cert", "company_id", companyID, "ref", ref, "error", err)
			} else {
				encryptedRef := strings.HasSuffix(strings.ToLower(strings.TrimSpace(ref)), ".enc")
				if p.crypto != nil {
					dec, derr := p.crypto.Decrypt(strings.TrimSpace(string(b)))
					if derr == nil {
						b = dec
					} else if encryptedRef {
						return nil, fmt.Errorf("siat provider: no se pudo descifrar P12: %w", derr)
					}
				} else if encryptedRef {
					return nil, fmt.Errorf("siat provider: crypto no configurado para descifrar P12")
				}

				// Los registros legacy pueden contener el P12 directamente en Base64.
				// El SDK debe recibir los bytes DER, nunca el Base64 como string.
				if decoded, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b))); derr == nil && len(decoded) > 0 {
					b = decoded
				}
				p12Bytes = b
			}
		}
	}

	// Ambiente y modalidad: cert override > company > infra default
	codigoAmbiente := company.Ambiente.CodigoAmbiente()
	if cert != nil && cert.Ambiente != nil && strings.TrimSpace(*cert.Ambiente) != "" {
		if strings.EqualFold(strings.TrimSpace(*cert.Ambiente), "PRODUCCION") {
			codigoAmbiente = 1
		} else {
			codigoAmbiente = 2
		}
	} else if p.infra.CodigoAmbiente == 1 || p.infra.CodigoAmbiente == 2 {
		// si company ambiente es vacío (no debería), usar infra
		if codigoAmbiente == 0 {
			codigoAmbiente = p.infra.CodigoAmbiente
		}
	}

	modalidad := company.Modalidad
	if modalidad == 0 {
		modalidad = 1
	}
	if cert != nil && cert.Modalidad != nil && *cert.Modalidad != 0 {
		modalidad = *cert.Modalidad
	} else if p.infra.Modalidad != 0 && modalidad == 0 {
		modalidad = p.infra.Modalidad
	}

	nitInt, err := strconv.ParseInt(strings.TrimSpace(company.Nit), 10, 64)
	if err != nil || nitInt <= 0 {
		return nil, fmt.Errorf("siat provider: NIT inválido para empresa %s: %q", companyID, company.Nit)
	}

	cfg := Config{
		Token:          token,
		Nit:            nitInt,
		CodigoSistema:  p.infra.CodigoSistema,
		CodigoAmbiente: codigoAmbiente,
		BaseURL:        p.infra.BaseURL,
		TraceId:        p.infra.TraceId,
		UserAgent:      p.infra.UserAgent,
		Timeout:        p.infra.Timeout,
		CertP12Bytes:   p12Bytes,
		CertP12Pass:    p12Pass,
	}
	// Nota: modalidad no va en siat.Config global, se maneja por request en usecase
	_ = modalidad

	svc, err := NewService(cfg)
	if err != nil {
		return nil, fmt.Errorf("siat provider: no se pudo crear siat.Service para empresa %s: %w", companyID, err)
	}
	return svc, nil
}

// loadP12Bytes usa el storage abstracto si está configurado; fallback a lectura local directa.
// Para R2 el storage debe ser R2CertStorage con bucket privado supay-certs (SSE-S3 automático, sin auto-creación).
func (p *provider) loadP12Bytes(ctx context.Context, ref string) ([]byte, error) {
	clean := strings.TrimSpace(ref)
	if clean == "" {
		return nil, fmt.Errorf("ref vacío")
	}
	// Si es base64 directo (sin archivo), retornarlo
	if _, err := base64.StdEncoding.DecodeString(clean); err == nil && len(clean) > 100 {
		return []byte(clean), nil
	}
	if p.certStorage != nil {
		if data, err := p.certStorage.Get(ctx, clean); err == nil {
			return data, nil
		} else {
			// fallback a try con ref tal cual (memory://, r2://)
			if data2, err2 := p.certStorage.Get(ctx, ref); err2 == nil {
				return data2, nil
			}
			// si falla, propagar error original para log
			slog.Warn("siat provider: certStorage.Get falló, intentando fallback local", "ref", ref, "error", err)
		}
	}
	// Fallback legacy: nunca debería usarse en cloud (sin disco persistente), pero mantiene compatibilidad self-host sin storage inyectado
	// Acepta cualquier base64 válido como P12 directo (incluye p12 cortos de test)
	if _, err := base64.StdEncoding.DecodeString(clean); err == nil {
		return []byte(clean), nil
	}
	if _, err := base64.RawStdEncoding.DecodeString(clean); err == nil {
		return []byte(clean), nil
	}
	return nil, fmt.Errorf("cert storage no configurado o ref no encontrado %q", ref)
}
