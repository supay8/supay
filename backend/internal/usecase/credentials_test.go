package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"gorm.io/gorm"
)

// --- fakes locales del servicio de credenciales ---

type fakeCredPOSStore struct {
	updated []*domain.PointOfSale
}

func (f *fakeCredPOSStore) Update(pos *domain.PointOfSale) error {
	f.updated = append(f.updated, pos)
	return nil
}

type fakeCredCufdStore struct {
	existing *domain.Cufd
	created  []*domain.Cufd
}

func (f *fakeCredCufdStore) Create(c *domain.Cufd) error {
	f.created = append(f.created, c)
	return nil
}

func (f *fakeCredCufdStore) GetActiveByPos(string) (*domain.Cufd, error) {
	if f.existing == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.existing, nil
}

type fakeCredClient struct {
	cuisCalls int
	cufdCalls int
	cuis      *siat.RespuestaCuis
	cufd      *siat.RespuestaCufd
	cuisErr   error
	cufdErr   error
}

func (f *fakeCredClient) RequestCUIS(context.Context, ports.CredentialRequest) (ports.CuisResult, error) {
	f.cuisCalls++
	if f.cuisErr != nil {
		return ports.CuisResult{}, f.cuisErr
	}
	return ports.CuisResult{
		Codigo:        f.cuis.Codigo,
		FechaVigencia: f.cuis.FechaVigencia.Time,
		Transaccion:   f.cuis.Transaccion,
		Mensajes:      convertMensajes(f.cuis.Mensajes),
	}, nil
}

func (f *fakeCredClient) RequestCUFD(context.Context, ports.CredentialRequest) (ports.CufdResult, error) {
	f.cufdCalls++
	if f.cufdErr != nil {
		return ports.CufdResult{}, f.cufdErr
	}
	return ports.CufdResult{
		Codigo:        f.cufd.Codigo,
		CodigoControl: f.cufd.CodigoControl,
		Direccion:     f.cufd.Direccion,
		FechaVigencia: f.cufd.FechaVigencia.Time,
		Transaccion:   f.cufd.Transaccion,
		Mensajes:      convertMensajes(f.cufd.Mensajes),
	}, nil
}

func (f *fakeCredClient) Emit(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
	return ports.FiscalResult{}, nil
}
func (f *fakeCredClient) VerifyStatus(context.Context, ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	return ports.FiscalDocumentResult{}, nil
}
func (f *fakeCredClient) Annul(context.Context, ports.FiscalDocumentQuery, int) (ports.FiscalDocumentResult, error) {
	return ports.FiscalDocumentResult{}, nil
}
func (f *fakeCredClient) RevertAnnul(context.Context, ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	return ports.FiscalDocumentResult{}, nil
}
func (f *fakeCredClient) RegisterSignificantEvent(context.Context, ports.FiscalEvent) (ports.FiscalEventResult, error) {
	return ports.FiscalEventResult{}, nil
}
func (f *fakeCredClient) SendPackage(context.Context, ports.FiscalPackage) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}
func (f *fakeCredClient) ValidatePackage(context.Context, ports.FiscalPackage, string) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}
func (f *fakeCredClient) SendBulk(context.Context, ports.FiscalBulk) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}
func (f *fakeCredClient) ValidateBulk(context.Context, ports.FiscalBulk, string) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}
func (f *fakeCredClient) SendPurchases(context.Context, ports.FiscalPurchase) (ports.FiscalPurchaseResult, error) {
	return ports.FiscalPurchaseResult{}, nil
}
func (f *fakeCredClient) SignXML(context.Context, ports.FiscalSignRequest) (ports.FiscalSignResult, error) {
	return ports.FiscalSignResult{}, nil
}
func (f *fakeCredClient) EmitAdjustment(context.Context, ports.FiscalAdjustment) (ports.FiscalAdjustmentResult, error) {
	return ports.FiscalAdjustmentResult{}, nil
}
func (f *fakeCredClient) Synchronize(context.Context, ports.FiscalSyncRequest, ports.FiscalSyncOperation) (ports.FiscalSyncResult, error) {
	return ports.FiscalSyncResult{}, nil
}

func credFixtures() (*domain.Company, *domain.PointOfSale) {
	company := &domain.Company{ID: "c1", Nit: "1020304050", CodigoSistema: "SIS", Ambiente: domain.EnvironmentPiloto}
	cuis := "CUIS-1"
	pos := &domain.PointOfSale{ID: "pos-1", CompanyId: "c1", CodigoSucursal: 0, Cuis: &cuis}
	return company, pos
}

func vigenteCufd() *siat.RespuestaCufd {
	return &siat.RespuestaCufd{
		Codigo:        "CUFD-2",
		CodigoControl: "CTRL-2",
		Direccion:     "AV. TEST 123",
		FechaVigencia: siat.XMLDateTime{Time: time.Now().Add(24 * time.Hour)},
		Transaccion:   true,
	}
}

func TestEnsureCuisYaPresenteNoLlamaSIAT(t *testing.T) {
	client := &fakeCredClient{cuis: &siat.RespuestaCuis{Codigo: "NUEVO", Transaccion: true}}
	posStore := &fakeCredPOSStore{}
	svc := NewCredentialService(posStore, &fakeCredCufdStore{}, client, 0)
	company, pos := credFixtures()

	if err := svc.EnsureCuis(context.Background(), company, pos); err != nil {
		t.Fatalf("EnsureCuis: %v", err)
	}
	if client.cuisCalls != 0 {
		t.Fatalf("se llamó al SIAT %d veces teniendo cuis vigente", client.cuisCalls)
	}
	if *pos.Cuis != "CUIS-1" {
		t.Fatalf("cuis pisado: %s", *pos.Cuis)
	}
}

func TestEnsureCuisAusenteSolicitaYPersiste(t *testing.T) {
	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	client := &fakeCredClient{cuis: &siat.RespuestaCuis{Codigo: "CUIS-NUEVO", FechaVigencia: siat.XMLDateTime{Time: expiresAt}, Transaccion: true}}
	posStore := &fakeCredPOSStore{}
	svc := NewCredentialService(posStore, &fakeCredCufdStore{}, client, 0)
	company, pos := credFixtures()
	pos.Cuis = nil

	if err := svc.EnsureCuis(context.Background(), company, pos); err != nil {
		t.Fatalf("EnsureCuis: %v", err)
	}
	if client.cuisCalls != 1 {
		t.Fatalf("cuisCalls=%d, se esperaba 1", client.cuisCalls)
	}
	if pos.Cuis == nil || *pos.Cuis != "CUIS-NUEVO" {
		t.Fatalf("cuis no asignado: %v", pos.Cuis)
	}
	if pos.CuisCreatedAt == nil {
		t.Fatal("cuis_created_at no asignado")
	}
	if pos.CuisExpiresAt == nil || !pos.CuisExpiresAt.Equal(expiresAt) {
		t.Fatalf("cuis_expires_at=%v, se esperaba %v", pos.CuisExpiresAt, expiresAt)
	}
	if len(posStore.updated) != 1 {
		t.Fatalf("persistencias=%d, se esperaba 1", len(posStore.updated))
	}
}

func TestEnsureCuisProximoAVencerSeRenueva(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	newExpiry := time.Now().Add(365 * 24 * time.Hour)
	client := &fakeCredClient{cuis: &siat.RespuestaCuis{
		Codigo: "CUIS-RENOVADO", FechaVigencia: siat.XMLDateTime{Time: newExpiry}, Transaccion: true,
	}}
	posStore := &fakeCredPOSStore{}
	svc := NewCredentialService(posStore, &fakeCredCufdStore{}, client, 0)
	company, pos := credFixtures()
	pos.CuisExpiresAt = &expiresAt

	if err := svc.EnsureCuis(context.Background(), company, pos); err != nil {
		t.Fatalf("EnsureCuis: %v", err)
	}
	if client.cuisCalls != 1 || pos.Cuis == nil || *pos.Cuis != "CUIS-RENOVADO" {
		t.Fatalf("el CUIS próximo a vencer no se renovó: calls=%d cuis=%v", client.cuisCalls, pos.Cuis)
	}
}

func TestEnsureCuisSinServicioEsNoDisponible(t *testing.T) {
	svc := NewCredentialService(&fakeCredPOSStore{}, &fakeCredCufdStore{}, nil, 0)
	company, pos := credFixtures()
	pos.Cuis = nil

	if err := svc.EnsureCuis(context.Background(), company, pos); err != ErrSiatNoDisponible {
		t.Fatalf("err=%v, se esperaba ErrSiatNoDisponible", err)
	}
}

func TestEnsureCufdExistenteNoLlamaSIAT(t *testing.T) {
	existing := &domain.Cufd{ID: "cufd-1", Cufd: "CUFD-1", Active: true, ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour)}
	client := &fakeCredClient{}
	svc := NewCredentialService(&fakeCredPOSStore{}, &fakeCredCufdStore{existing: existing}, client, 0)
	company, pos := credFixtures()

	cufd, err := svc.EnsureCufd(context.Background(), company, pos)
	if err != nil {
		t.Fatalf("EnsureCufd: %v", err)
	}
	if cufd.ID != "cufd-1" {
		t.Fatalf("cufd=%s, se esperaba el existente", cufd.ID)
	}
	if client.cufdCalls != 0 {
		t.Fatalf("se llamó al SIAT %d veces teniendo cufd vigente", client.cufdCalls)
	}
}

func TestEnsureCufdAusenteSolicitaYPersiste(t *testing.T) {
	client := &fakeCredClient{cufd: vigenteCufd()}
	cufdStore := &fakeCredCufdStore{}
	svc := NewCredentialService(&fakeCredPOSStore{}, cufdStore, client, 0)
	company, pos := credFixtures()

	cufd, err := svc.EnsureCufd(context.Background(), company, pos)
	if err != nil {
		t.Fatalf("EnsureCufd: %v", err)
	}
	if client.cufdCalls != 1 {
		t.Fatalf("cufdCalls=%d, se esperaba 1", client.cufdCalls)
	}
	if cufd.Cufd != "CUFD-2" || !cufd.Active {
		t.Fatalf("cufd persistido incorrecto: %+v", cufd)
	}
	if len(cufdStore.created) != 1 {
		t.Fatalf("persistencias=%d, se esperaba 1", len(cufdStore.created))
	}
}

func TestEnsureCufdEncadenaCuisCuandoFalta(t *testing.T) {
	client := &fakeCredClient{
		cuis: &siat.RespuestaCuis{Codigo: "CUIS-RESUELTO", Transaccion: true},
		cufd: vigenteCufd(),
	}
	posStore := &fakeCredPOSStore{}
	svc := NewCredentialService(posStore, &fakeCredCufdStore{}, client, 0)
	company, pos := credFixtures()
	pos.Cuis = nil

	if _, err := svc.EnsureCufd(context.Background(), company, pos); err != nil {
		t.Fatalf("EnsureCufd: %v", err)
	}
	if client.cuisCalls != 1 || client.cufdCalls != 1 {
		t.Fatalf("llamadas cuis=%d cufd=%d, se esperaba 1 y 1", client.cuisCalls, client.cufdCalls)
	}
	if pos.Cuis == nil || *pos.Cuis != "CUIS-RESUELTO" {
		t.Fatalf("cuis no resuelto: %v", pos.Cuis)
	}
}

func TestEnsureCufdRechazoSiatEsConflicto(t *testing.T) {
	client := &fakeCredClient{cufd: &siat.RespuestaCufd{Transaccion: false}}
	svc := NewCredentialService(&fakeCredPOSStore{}, &fakeCredCufdStore{}, client, 0)
	company, pos := credFixtures()

	_, err := svc.EnsureCufd(context.Background(), company, pos)
	if err == nil {
		t.Fatal("se esperaba error por respuesta sin transacción")
	}
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err=%v, se esperaba ConflictError", err)
	}
}

func convertMensajes(in []siat.Mensaje) []ports.FiscalMessage {
	out := make([]ports.FiscalMessage, len(in))
	for i, m := range in {
		out[i] = ports.FiscalMessage{Codigo: m.Codigo, Descripcion: m.Descripcion}
	}
	return out
}
