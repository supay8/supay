package usecase

import (
	"context"
	"errors"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"testing"
	"time"
)

func TestBuildSolicitudFacturaFaltaCuis(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.PointOfSale.Cuis = nil

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("se esperaba error por CUIS ausente")
	}
}

func TestBuildSolicitudFacturaCufdVencido(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CufdRecord.ValidTo = time.Now().Add(-time.Hour)

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("se esperaba error por CUFD vencido")
	}
}

func TestBuildSolicitudFacturaUsaCufdMasReciente(t *testing.T) {
	inv := testInvoice()
	inv.CufdRecord.Cufd = "CUFD-VIEJO"
	inv.CufdRecord.ControlCode = "CC-VIEJO"

	cufdRepo := &fakeCufdRepo{vigente: &domain.Cufd{
		ID:          "cufd-nuevo",
		Cufd:        "CUFD-NUEVO",
		ControlCode: "CC-NUEVO",
		ValidFrom:   time.Now().Add(-time.Hour),
		ValidTo:     time.Now().Add(time.Hour),
		Active:      true,
	}}
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	if req.Cufd != "CUFD-NUEVO" || req.CodigoControl != "CC-NUEVO" {
		t.Errorf("debío usar el CUFD más reciente, se obtuvo Cufd=%q CodigoControl=%q", req.Cufd, req.CodigoControl)
	}
}

func TestBuildSolicitudFacturaCufdVencidoConVigente(t *testing.T) {
	inv := testInvoice()
	inv.CufdRecord.ValidTo = time.Now().Add(-time.Hour)

	cufdRepo := &fakeCufdRepo{vigente: &domain.Cufd{
		ID:          "cufd-nuevo",
		Cufd:        "CUFD-NUEVO",
		ControlCode: "CC-NUEVO",
		ValidFrom:   time.Now().Add(-time.Hour),
		ValidTo:     time.Now().Add(time.Hour),
		Active:      true,
	}}
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura con CUFD de factura vencido pero CUFD nuevo vigente: %v", err)
	}
	if req.Cufd != "CUFD-NUEVO" {
		t.Errorf("Cufd=%q, se esperaba CUFD-NUEVO", req.Cufd)
	}
}

func TestBuildSolicitudFacturaLazyCredenciales(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	provider := &fakeCredentialProvider{
		cufd: &domain.Cufd{ID: "cufd-lazy", Cufd: "CUFD-LAZY", ControlCode: "CTRL-LAZY", Active: true,
			ValidFrom: time.Now().Add(-time.Minute), ValidTo: time.Now().Add(time.Hour),
			Direccion: "AV. PADRON 123"},
	}
	uc.credentials = provider

	inv := testInvoice()
	inv.PointOfSale.Cuis = nil // sin cuis ni cufd vigente

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura con credenciales lazy: %v", err)
	}
	if provider.cuisCalls != 1 || provider.cufdCalls != 1 {
		t.Fatalf("llamadas cuis=%d cufd=%d, se esperaban 1 y 1", provider.cuisCalls, provider.cufdCalls)
	}
	if req.Cuis != "CUIS-LAZY" || req.Cufd != "CUFD-LAZY" || req.CodigoControl != "CTRL-LAZY" {
		t.Fatalf("solicitud con credenciales lazy incorrectas: cuis=%s cufd=%s ctrl=%s", req.Cuis, req.Cufd, req.CodigoControl)
	}
}

func TestBuildSolicitudFacturaSinProviderMantieneError(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.PointOfSale.Cuis = nil

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("sin provider de credenciales el cuis ausente debe seguir siendo error")
	}
}

func TestEmitEsperaCufdAntesDeClaim(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	repo.invoices[inv.ID] = inv
	provider := &fakeCredentialProvider{cufdErr: errors.New("renovación CUFD temporalmente no disponible")}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, &fakeEmissionService{})
	uc.credentials = provider

	if _, err := uc.Emit(invoiceContext(), inv.ID); err == nil {
		t.Fatal("se esperaba que la emisión esperara la renovación del CUFD")
	}
	if repo.claimCalls != 0 {
		t.Fatalf("ClaimForEmission fue llamado %d veces antes de tener CUFD", repo.claimCalls)
	}
	if inv.Status != domain.InvoicePending {
		t.Fatalf("status=%s, la factura debe permanecer PENDING mientras espera CUFD", inv.Status)
	}
}
