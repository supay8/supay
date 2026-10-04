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

// CertificateInput contiene únicamente el material de firma digital recibido
// vía multipart/form-data. La configuración fiscal vive en el tenant.
type CertificateInput struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // P12
	P12Password string `json:"p12_password"`
	// P12Bytes son los bytes planos del .p12 recibidos via multipart p12_file (no JSON)
	P12Bytes []byte `json:"-"`
}

type CertificateUsecase struct {
	certRepo             domain.CertificateRepository
	companyRepo          domain.CompanyRepository
	crypto               *crypto.Service
	storage              storage.CertStorage
	invalidateSiatClient func(companyID string)
}

func NewCertificateUsecase(certRepo domain.CertificateRepository, companyRepo domain.CompanyRepository, cryptoSvc *crypto.Service, storage storage.CertStorage, invalidators ...func(string)) *CertificateUsecase {
	uc := &CertificateUsecase{certRepo: certRepo, companyRepo: companyRepo, crypto: cryptoSvc, storage: storage}
	if len(invalidators) > 0 {
		uc.invalidateSiatClient = invalidators[0]
	}
	return uc
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
	if len(in.P12Bytes) == 0 {
		return nil, domain.NewBadRequestError("p12_file es obligatorio (multipart/form-data, campo p12_file)")
	}
	if len(in.P12Bytes) > 5<<20 {
		return nil, domain.NewBadRequestError("p12_file excede 5MB")
	}
	if uc.crypto == nil {
		return nil, domain.NewConflictError("cifrado no configurado: defina ENCRYPTION_KEY")
	}
	if _, err := uc.companyRepo.GetByID(companyID); err != nil {
		return nil, domain.NewNotFoundError("empresa no encontrada")
	}
	encPass := ""
	if strings.TrimSpace(in.P12Password) != "" {
		var err error
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
		EncryptedP12Password: encPass,
	}
	if in.Type != "" {
		cert.Type = strings.ToUpper(strings.TrimSpace(in.Type))
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
	// Guardar el P12 cifrado bajo la empresa, igual que las facturas.
	key := fmt.Sprintf("companies/%s/cert/%s.p12.enc", companyID, cert.ID)
	ref, err := uc.storage.Put(context.Background(), key, []byte(encP12))
	if err != nil {
		// No revertir cert pero reportar
		return nil, fmt.Errorf("certificado creado pero no se pudo persistir P12: %w", err)
	}
	cert.P12StorageRef = ref
	if err := uc.certRepo.Update(cert); err != nil {
		return nil, err
	}
	if uc.invalidateSiatClient != nil {
		uc.invalidateSiatClient(companyID)
	}
	return cert, nil
}

func (uc *CertificateUsecase) List(companyID string) ([]*domain.Certificate, error) {
	return uc.certRepo.ListByCompany(companyID)
}

func (uc *CertificateUsecase) GetActive(companyID string) (*domain.Certificate, error) {
	return uc.certRepo.GetActiveByCompany(companyID)
}

func (uc *CertificateUsecase) Update(id string, companyID string, in CertificateInput) (*domain.Certificate, error) {
	cert, err := uc.certRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if cert.CompanyId != companyID {
		return nil, domain.NewBadRequestError("certificado no pertenece a la empresa")
	}
	if len(in.P12Bytes) > 5<<20 {
		return nil, domain.NewBadRequestError("p12_file excede 5MB")
	}
	if len(in.P12Bytes) > 0 && uc.crypto == nil {
		return nil, domain.NewConflictError("cifrado no configurado: defina ENCRYPTION_KEY")
	}
	if len(in.P12Bytes) > 0 && uc.storage == nil {
		return nil, fmt.Errorf("cert storage no configurado")
	}
	var rawPassword string
	if in.P12Password != "" {
		rawPassword = strings.TrimSpace(in.P12Password)
		encPass, err := uc.crypto.EncryptString(rawPassword)
		if err != nil {
			return nil, fmt.Errorf("no se pudo cifrar password P12: %w", err)
		}
		cert.EncryptedP12Password = encPass
	}
	if in.Name != "" {
		cert.Name = strings.TrimSpace(in.Name)
	}
	if in.Type != "" {
		cert.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	}
	var newStorageRef string
	oldStorageRef := cert.P12StorageRef
	if len(in.P12Bytes) > 0 {
		encP12, err := uc.crypto.Encrypt(in.P12Bytes)
		if err != nil {
			return nil, fmt.Errorf("no se pudo cifrar P12: %w", err)
		}
		key := fmt.Sprintf("companies/%s/cert/%s-%d.p12.enc", companyID, cert.ID, time.Now().UTC().UnixNano())
		ref, err := uc.storage.Put(context.Background(), key, []byte(encP12))
		if err != nil {
			return nil, fmt.Errorf("certificado actualizado pero no se pudo persistir P12: %w", err)
		}
		newStorageRef = ref
		cert.P12StorageRef = ref
	}
	if err := uc.certRepo.Update(cert); err != nil {
		if newStorageRef != "" {
			_ = uc.storage.Delete(context.Background(), newStorageRef)
		}
		return nil, err
	}
	if newStorageRef != "" && oldStorageRef != "" && oldStorageRef != newStorageRef {
		_ = uc.storage.Delete(context.Background(), oldStorageRef)
	}
	if uc.invalidateSiatClient != nil {
		uc.invalidateSiatClient(companyID)
	}
	return cert, nil
}

func (uc *CertificateUsecase) Delete(id string) error {
	// Los certificados son historial fiscal: revocar conserva tanto el registro
	// como el material cifrado necesario para auditoría de facturas históricas.
	cert, err := uc.certRepo.GetByID(id)
	if err != nil {
		return err
	}
	if err := uc.certRepo.Delete(id); err != nil {
		return err
	}
	if uc.invalidateSiatClient != nil {
		uc.invalidateSiatClient(cert.CompanyId)
	}
	return nil
}
