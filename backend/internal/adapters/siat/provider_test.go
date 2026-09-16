package siat

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/brandsrx/supay/internal/crypto"
	"github.com/brandsrx/supay/internal/domain"
)

// testMemoryStorage mock evita import cycle con internal/storage
type testMemoryStorage struct {
	data map[string][]byte
}

func newTestMemoryStorage() *testMemoryStorage {
	return &testMemoryStorage{data: make(map[string][]byte)}
}
func (m *testMemoryStorage) Put(_ context.Context, key string, data []byte) (string, error) {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[key] = cp
	return "memory://" + key, nil
}
func (m *testMemoryStorage) Get(_ context.Context, ref string) ([]byte, error) {
	if v, ok := m.data[ref]; ok {
		cp := make([]byte, len(v))
		copy(cp, v)
		return cp, nil
	}
	// try without prefix
	key := ref
	if len(ref) > 9 && ref[:9] == "memory://" {
		key = ref[9:]
	}
	if v, ok := m.data[key]; ok {
		cp := make([]byte, len(v))
		copy(cp, v)
		return cp, nil
	}
	return nil, context.Canceled
}
func (m *testMemoryStorage) Delete(_ context.Context, ref string) error {
	delete(m.data, ref)
	if len(ref) > 9 && ref[:9] == "memory://" {
		delete(m.data, ref[9:])
	}
	return nil
}

type fakeCompanyRepo struct {
	company domain.Company
}

func (f *fakeCompanyRepo) Create(*domain.Company) error             { return nil }
func (f *fakeCompanyRepo) GetByNit(string) (*domain.Company, error) { return &f.company, nil }
func (f *fakeCompanyRepo) GetByID(string) (*domain.Company, error)  { return &f.company, nil }
func (f *fakeCompanyRepo) Update(*domain.Company) error             { return nil }
func (f *fakeCompanyRepo) Delete(string) error                      { return nil }

type fakeCertRepo struct {
	cert *domain.Certificate
	err  error
}

func (f *fakeCertRepo) Create(*domain.Certificate) error            { return nil }
func (f *fakeCertRepo) GetByID(string) (*domain.Certificate, error) { return f.cert, f.err }
func (f *fakeCertRepo) GetActiveByCompany(string) (*domain.Certificate, error) {
	if f.cert == nil {
		return nil, f.err
	}
	return f.cert, nil
}
func (f *fakeCertRepo) ListByCompany(string) ([]*domain.Certificate, error) { return nil, nil }
func (f *fakeCertRepo) Update(*domain.Certificate) error                    { return nil }
func (f *fakeCertRepo) Delete(string) error                                 { return nil }

func TestProviderDecryptAndBuildService(t *testing.T) {
	// Crypto with known master
	cr := crypto.MustNew("test-master-key-12345678901234567890123456789012")
	// Prepare encrypted token and p12
	token := "TOKEN_DELEGADO_TEST_123"
	p12Raw := []byte("fake-p12-bytes-content-12345")
	encToken, _ := cr.EncryptString(token)
	encPass, _ := cr.EncryptString("p12pass")
	encP12, _ := cr.Encrypt(p12Raw)

	// Storage memory with encrypted p12
	mem := newTestMemoryStorage()
	key := "certs/comp-1/cert-123.p12.enc"
	ref, _ := mem.Put(context.Background(), key, []byte(encP12))

	company := domain.Company{
		ID:                     "comp-1",
		Nit:                    "123456789",
		Ambiente:               domain.EnvironmentPiloto,
		Modalidad:              1,
		BusinessName:           "Test Company",
		Municipio:              "LA PAZ",
		Direccion:              "AV TEST 123",
		EncryptedTokenDelegado: encToken,
	}
	cert := &domain.Certificate{
		ID:                   "cert-123",
		CompanyId:            "comp-1",
		Status:               domain.CertificateActive,
		EncryptedP12Password: encPass,
		P12StorageRef:        ref,
	}

	companyRepo := &fakeCompanyRepo{company: company}
	certRepo := &fakeCertRepo{cert: cert}
	infra := ProviderInfra{
		BaseURL:        "https://pilotosiatservicios.impuestos.gob.bo/v2",
		CodigoAmbiente: 2,
		CodigoSistema:  "SYS123456",
		Modalidad:      1,
	}
	provider := NewSiatClientProviderWithStorage(companyRepo, certRepo, cr, mem, infra)
	svc, err := provider.GetForCompany(context.Background(), "comp-1")
	if err != nil {
		t.Fatalf("GetForCompany: %v", err)
	}
	if svc == nil || svc.sdk == nil {
		t.Fatalf("service or sdk nil")
	}
	credential := svc.sdk.Config().CredentialSign
	if !bytes.Equal(credential.P12Bytes, p12Raw) {
		t.Fatalf("P12 no llegó descifrado al SDK: got=%d bytes want=%d", len(credential.P12Bytes), len(p12Raw))
	}
	if credential.P12Password != "p12pass" {
		t.Fatalf("password P12 no llegó descifrado al SDK")
	}
	// Second call should be cached (same pointer)
	svc2, err := provider.GetForCompany(context.Background(), "comp-1")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if svc != svc2 {
		t.Fatalf("expected cached same instance")
	}
	// Invalidate and re-fetch should still work
	provider.Invalidate("comp-1")
	svc3, err := provider.GetForCompany(context.Background(), "comp-1")
	if err != nil {
		t.Fatalf("after invalidate: %v", err)
	}
	if svc3 == nil {
		t.Fatalf("svc3 nil")
	}
}

func TestProviderRejectsEncryptedP12WithWrongKey(t *testing.T) {
	cr := crypto.MustNew("correct-master-key-123456789012345678901234")
	otherCrypto := crypto.MustNew("wrong-master-key-12345678901234567890123456")
	encToken, _ := cr.EncryptString("TOKEN123")
	encP12, _ := otherCrypto.Encrypt([]byte("fake-p12"))

	mem := newTestMemoryStorage()
	ref, _ := mem.Put(context.Background(), "certs/comp-1/cert-123.p12.enc", []byte(encP12))
	company := domain.Company{ID: "comp-1", Nit: "123456789", Ambiente: domain.EnvironmentPiloto, Modalidad: 1, EncryptedTokenDelegado: encToken}
	cert := &domain.Certificate{
		ID: "cert-123", CompanyId: "comp-1", Status: domain.CertificateActive,
		P12StorageRef: ref,
	}
	provider := NewSiatClientProviderWithStorage(
		&fakeCompanyRepo{company: company},
		&fakeCertRepo{cert: cert},
		cr,
		mem,
		ProviderInfra{BaseURL: "https://example.com", CodigoAmbiente: 2, CodigoSistema: "SYS123"},
	)

	_, err := provider.GetForCompany(context.Background(), "comp-1")
	if err == nil || !strings.Contains(err.Error(), "no se pudo descifrar P12") {
		t.Fatalf("expected explicit P12 decrypt error, got %v", err)
	}
}

func TestBuildCredentialSignRemovesP12TrailingPadding(t *testing.T) {
	// SEQUENCE de 3 bytes seguida por relleno externo que no pertenece al DER.
	padded := []byte{0x30, 0x03, 0x02, 0x01, 0x03, 0x00, 0x00, '\n'}
	want := padded[:5]

	credential := buildCredentialSign(Config{CertP12Bytes: padded, CertP12Pass: "secret"})
	if !bytes.Equal(credential.P12Bytes, want) {
		t.Fatalf("P12 padding was not removed: got=%x want=%x", credential.P12Bytes, want)
	}
	if credential.P12Password != "secret" {
		t.Fatalf("P12 password changed during normalization")
	}
}

func TestBuildCredentialSignPreservesNonPaddingTrailingData(t *testing.T) {
	withData := []byte{0x30, 0x03, 0x02, 0x01, 0x03, 0xff}
	credential := buildCredentialSign(Config{CertP12Bytes: withData})
	if !bytes.Equal(credential.P12Bytes, withData) {
		t.Fatalf("non-padding trailing data must not be removed")
	}
}

func TestProviderMissingTokenError(t *testing.T) {
	cr := crypto.MustNew("test-master-key-12345678901234567890123456789012")
	mem := newTestMemoryStorage()
	company := domain.Company{ID: "comp-1", Nit: "123", Ambiente: domain.EnvironmentPiloto, Modalidad: 1}
	cert := &domain.Certificate{ID: "cert-123", CompanyId: "comp-1", Status: domain.CertificateActive}
	provider := NewSiatClientProviderWithStorage(&fakeCompanyRepo{company: company}, &fakeCertRepo{cert: cert}, cr, mem, ProviderInfra{BaseURL: "https://example.com", CodigoAmbiente: 2, CodigoSistema: "SYS"})
	_, err := provider.GetForCompany(context.Background(), "comp-1")
	if err == nil {
		t.Fatalf("expected error for missing token")
	}
}

func TestProviderLocalWithoutStorageFallback(t *testing.T) {
	// Ensure provider without storage still handles base64 direct ref
	cr := crypto.MustNew("test-master-key-12345678901234567890123456789012")
	p12B64 := base64.StdEncoding.EncodeToString([]byte("plain-p12"))
	encToken, _ := cr.EncryptString("TOKEN123")
	company := domain.Company{ID: "comp-1", Nit: "123456789", Ambiente: domain.EnvironmentPiloto, Modalidad: 1, EncryptedTokenDelegado: encToken}
	cert := &domain.Certificate{
		ID: "cert-123", CompanyId: "comp-1", Status: domain.CertificateActive,
		P12StorageRef: p12B64, // direct base64, no storage
	}
	provider := NewSiatClientProvider(&fakeCompanyRepo{company: company}, &fakeCertRepo{cert: cert}, cr, ProviderInfra{BaseURL: "https://example.com", CodigoAmbiente: 2, CodigoSistema: "SYS123"})
	svc, err := provider.GetForCompany(context.Background(), "comp-1")
	if err != nil {
		t.Fatalf("direct base64 p12 should work: %v", err)
	}
	if svc == nil {
		t.Fatalf("svc nil")
	}
	if got := svc.sdk.Config().CredentialSign.P12Bytes; !bytes.Equal(got, []byte("plain-p12")) {
		t.Fatalf("direct base64 P12 was not decoded: got=%q", got)
	}
}
