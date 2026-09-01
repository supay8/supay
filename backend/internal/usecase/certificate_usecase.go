package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/crypto"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/storage"
)

// CertificateInput es el payload para configurar credenciales fiscales por empresa vía multipart/form-data.
// Solo multipart: p12_file binario + campos texto (token, p12_password, etc). No JSON base64.
type CertificateInput struct {
	Name        string     `json:"name"`
	Type        string     `json:"type"` // P12
	Token       string     `json:"token"` // token delegado SIAT (se cifra)
	P12Password string     `json:"p12_password"`
	Modalidad   *int       `json:"modalidad,omitempty"`
	Ambiente    *string    `json:"ambiente,omitempty"` // PILOTO | PRODUCCION
	NotBefore   *time.Time `json:"not_before,omitempty"`
	NotAfter    *time.Time `json:"not_after,omitempty"`
	// P12Bytes son los bytes planos del .p12 recibidos via multipart p12_file (no JSON)
	P12Bytes []byte `json:"-"`
}

type CertificateUsecase struct {
	certRepo    domain.CertificateRepository
	companyRepo domain.CompanyRepository
	crypto      *crypto.Service
	storage     storage.CertStorage
}

func NewCertificateUsecase(certRepo domain.CertificateRepository, companyRepo domain.CompanyRepository, cryptoSvc *crypto.Service, storage storage.CertStorage) *CertificateUsecase {
	return &CertificateUsecase{certRepo: certRepo, companyRepo: companyRepo, crypto: cryptoSvc, storage: storage}
}

// NewCertificateUsecaseWithPath legado para tests con path directo (crea Local storage).
func NewCertificateUsecaseWithPath(certRepo domain.CertificateRepository, companyRepo domain.CompanyRepository, cryptoSvc *crypto.Service, storagePath string) *CertificateUsecase {
	st, _ := storage.NewLocalCertStorage(storagePath)
	return NewCertificateUsecase(certRepo, companyRepo, cryptoSvc, st)
}

func (uc *CertificateUsecase) Create(companyID string, in CertificateInput) (*domain.Certificate, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	if strings.TrimSpace(in.Token) == "" {
		return nil, domain.NewBadRequestError("token delegado es obligatorio")
	}
	if len(in.P12Bytes) == 0 {
		return nil, domain.NewBadRequestError("p12_file es obligatorio (multipart/form-data, campo p12_file)")
	}
	if len(in.P12Bytes) > 5<<20 {
		return nil, domain.NewBadRequestError("p12_file excede 5MB")
	}
	if uc.crypto == nil {
		return nil, domain.NewConflictError("cifrado no configurado: defina ENCRYPTION_KEY")
	}
	company, err := uc.companyRepo.GetByID(companyID)
	if err != nil {
		return nil, domain.NewNotFoundError("empresa no encontrada")
	}
	encToken, err := uc.crypto.EncryptString(strings.TrimSpace(in.Token))
	if err != nil {
		return nil, fmt.Errorf("no se pudo cifrar token: %w", err)
	}
	encPass := ""
	if strings.TrimSpace(in.P12Password) != "" {
		encPass, err = uc.crypto.EncryptString(strings.TrimSpace(in.P12Password))
		if err != nil {
			return nil, fmt.Errorf("no se pudo cifrar password P12: %w", err)
		}
	}
	// Cifrar bytes planos del .p12 inmediatamente (nunca toca disco plano)
	encP12, err := uc.crypto.Encrypt(in.P12Bytes)
	if err != nil {
		return nil, fmt.Errorf("no se pudo cifrar P12: %w", err)
	}
	if uc.storage == nil {
		return nil, fmt.Errorf("cert storage no configurado")
	}
	now := time.Now()
	cert := &domain.Certificate{
		CompanyId:            companyID,
		Name:                 in.Name,
		Type:                 "P12",
		Status:               domain.CertificateActive,
		NotBefore:            now,
		NotAfter:             now.Add(365 * 24 * time.Hour),
		EncryptedToken:       encToken,
		EncryptedP12Password: encPass,
		Modalidad:            in.Modalidad,
		Ambiente:             in.Ambiente,
		Nit:                  company.Nit,
	}
	if in.Type != "" {
		cert.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	}
	if in.NotBefore != nil {
		cert.NotBefore = *in.NotBefore
	}
	if in.NotAfter != nil {
		cert.NotAfter = *in.NotAfter
	}
	if cert.Name == "" {
		short := companyID
		if len(short) > 8 {
			short = short[:8]
		}
		cert.Name = fmt.Sprintf("cert-%s-%s", short, now.Format("20060102"))
	}
	if err := uc.certRepo.Create(cert); err != nil {
		return nil, err
	}
	// Guardar P12 cifrado en storage abstracto; key jerárquica certs/<companyId>/<certId>.p12.enc
	key := fmt.Sprintf("certs/%s/%s.p12.enc", companyID, cert.ID)
	ref, err := uc.storage.Put(context.Background(), key, []byte(encP12))
	if err != nil {
		// No revertir cert pero reportar
		return nil, fmt.Errorf("certificado creado pero no se pudo persistir P12: %w", err)
	}
	cert.P12StorageRef = ref
	if err := uc.certRepo.Update(cert); err != nil {
		return nil, err
	}
	return cert, nil
}

func (uc *CertificateUsecase) List(companyID string) ([]*domain.Certificate, error) {
	return uc.certRepo.ListByCompany(companyID)
}

func (uc *CertificateUsecase) GetActive(companyID string) (*domain.Certificate, error) {
	return uc.certRepo.GetActiveByCompany(companyID)
}

func (uc *CertificateUsecase) Delete(id string) error {
	cert, err := uc.certRepo.GetByID(id)
	if err != nil {
		return err
	}
	if cert.P12StorageRef != "" && uc.storage != nil {
		_ = uc.storage.Delete(context.Background(), cert.P12StorageRef)
	}
	return uc.certRepo.Delete(id)
}
