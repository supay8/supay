package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
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

func (f *fakeCredClient) SolicitarCUIS(context.Context, siat.SolicitudCuis) (*siat.RespuestaCuis, error) {
	f.cuisCalls++
	return f.cuis, f.cuisErr
}

func (f *fakeCredClient) SolicitarCUFD(context.Context, siat.SolicitudCufd) (*siat.RespuestaCufd, error) {
	f.cufdCalls++
	return f.cufd, f.cufdErr
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
	client := &fakeCredClient{cuis: &siat.RespuestaCuis{Codigo: "CUIS-NUEVO", Transaccion: true}}
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
	if len(posStore.updated) != 1 {
		t.Fatalf("persistencias=%d, se esperaba 1", len(posStore.updated))
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
