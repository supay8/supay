package usecase

import (
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
