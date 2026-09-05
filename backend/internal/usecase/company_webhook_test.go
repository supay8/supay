package usecase

import (
	"errors"
	"testing"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type webhookCompanyRepo struct {
	company *domain.Company
	updated bool
}

func (f *webhookCompanyRepo) Create(company *domain.Company) error {
	f.company = company
	return nil
}

func (f *webhookCompanyRepo) GetByNit(string) (*domain.Company, error) {
	if f.company == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.company, nil
}

func (f *webhookCompanyRepo) GetByID(string) (*domain.Company, error) {
	if f.company == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.company, nil
}

func (f *webhookCompanyRepo) Update(company *domain.Company) error {
	f.company = company
	f.updated = true
	return nil
}

func (f *webhookCompanyRepo) Delete(string) error { return nil }

func TestCompanyUpdateConfiguresCertificateWebhook(t *testing.T) {
	repo := &webhookCompanyRepo{company: &domain.Company{ID: "tenant-1", Nit: "123", Ambiente: domain.EnvironmentPiloto}}
	service := NewCompanyUsecase(repo)
	webhookURL := "https://tenant.example.test/supay-alerts"

	company, err := service.Update(UpdateCompanyRequest{CertificateWebhookURL: &webhookURL}, "tenant-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !repo.updated || company.CertificateWebhookURL != webhookURL {
		t.Fatalf("webhook no persistido: updated=%v value=%q", repo.updated, company.CertificateWebhookURL)
	}
}

func TestCompanyRejectsInvalidCertificateWebhook(t *testing.T) {
	repo := &webhookCompanyRepo{company: &domain.Company{ID: "tenant-1", Nit: "123", Ambiente: domain.EnvironmentPiloto}}
	service := NewCompanyUsecase(repo)
	webhookURL := "file:///etc/passwd"

	_, err := service.Update(UpdateCompanyRequest{CertificateWebhookURL: &webhookURL}, "tenant-1")
	var badRequest *domain.BadRequestError
	if !errors.As(err, &badRequest) {
		t.Fatalf("err=%v, se esperaba BadRequestError", err)
	}
	if repo.updated {
		t.Fatal("el webhook inválido no debe persistirse")
	}
}
