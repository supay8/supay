package usecase

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/brandsrx/supay/internal/crypto"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/storage"
	"gorm.io/gorm"
)

type certificateRepoStub struct {
	item *domain.Certificate
}

func (r *certificateRepoStub) Create(cert *domain.Certificate) error {
	cert.ID = "cert-1"
	r.item = cert
	return nil
}
func (r *certificateRepoStub) GetByID(id string) (*domain.Certificate, error) {
	if r.item == nil || r.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}
func (r *certificateRepoStub) GetActiveByCompany(string) (*domain.Certificate, error) {
	return r.item, nil
}
func (r *certificateRepoStub) ListByCompany(string) ([]*domain.Certificate, error) {
	return []*domain.Certificate{r.item}, nil
}
func (r *certificateRepoStub) Update(cert *domain.Certificate) error { r.item = cert; return nil }
func (r *certificateRepoStub) Delete(string) error                   { return nil }

func TestCertificateCreateOnlyRequiresSignatureMaterial(t *testing.T) {
	certRepo := &certificateRepoStub{}
	companyRepo := &phase12CompanyRepo{items: map[string]*domain.Company{
		"company-1": {ID: "company-1", Nit: "123", Ambiente: domain.EnvironmentPiloto},
	}}
	invalidated := ""
	uc := NewCertificateUsecase(
		certRepo,
		companyRepo,
		crypto.MustNew("test-master-key-12345678901234567890123456789012"),
		storage.NewMemoryCertStorage(),
		func(companyID string) { invalidated = companyID },
	)

	cert, err := uc.Create("company-1", CertificateInput{
		Name:     "principal",
		Type:     "P12",
		P12Bytes: []byte("p12-signature-material"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cert.P12StorageRef == "" || invalidated != "company-1" {
		t.Fatalf("certificado no persistido o cliente no invalidado: cert=%+v invalidated=%q", cert, invalidated)
	}
}

func TestCertificateUpdateReplacesP12StorageRef(t *testing.T) {
	for _, driver := range []string{"memory", "local"} {
		t.Run(driver, func(t *testing.T) {
			testCertificateUpdateReplacesP12StorageRef(t, driver)
		})
	}
}

func testCertificateUpdateReplacesP12StorageRef(t *testing.T, driver string) {
	certRepo := &certificateRepoStub{}
	companyRepo := &phase12CompanyRepo{items: map[string]*domain.Company{
		"company-1": {ID: "company-1", Nit: "123", Ambiente: domain.EnvironmentPiloto},
	}}
	var certStorage storage.CertStorage = storage.NewMemoryCertStorage()
	base := t.TempDir()
	if driver == "local" {
		local, err := storage.NewLocalCertStorage(base)
		if err != nil {
			t.Fatal(err)
		}
		certStorage = local
	}
	cryptoSvc := crypto.MustNew("test-master-key-12345678901234567890123456789012")
	uc := NewCertificateUsecase(
		certRepo,
		companyRepo,
		cryptoSvc,
		certStorage,
	)

	cert, err := uc.Create("company-1", CertificateInput{
		Name:     "principal",
		Type:     "P12",
		P12Bytes: []byte("old-p12-signature-material"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	oldRef := cert.P12StorageRef
	prefix := "memory://companies/company-1/cert/"
	if driver == "local" {
		prefix = filepath.Join(base, "companies", "company-1", "cert") + string(filepath.Separator)
	}
	if !strings.HasPrefix(oldRef, prefix) {
		t.Fatalf("unexpected create path: %q", oldRef)
	}

	updated, err := uc.Update(cert.ID, "company-1", CertificateInput{
		Name:     "principal actualizado",
		P12Bytes: []byte("new-p12-signature-material"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.P12StorageRef == "" || updated.P12StorageRef == oldRef {
		t.Fatalf("P12StorageRef no fue reemplazado: old=%q new=%q", oldRef, updated.P12StorageRef)
	}
	if !strings.HasPrefix(updated.P12StorageRef, prefix) {
		t.Fatalf("unexpected update path: %q", updated.P12StorageRef)
	}
	if _, err := certStorage.Get(nil, oldRef); err == nil {
		t.Fatalf("old P12 storage ref should be deleted")
	}
	data, err := certStorage.Get(nil, updated.P12StorageRef)
	if err != nil {
		t.Fatalf("new P12 storage ref should exist: %v", err)
	}
	plain, err := cryptoSvc.Decrypt(string(data))
	if err != nil || string(plain) != "new-p12-signature-material" {
		t.Fatalf("incorrect updated P12 content: %v", err)
	}
}
