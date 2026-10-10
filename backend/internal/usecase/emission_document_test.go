package usecase

import (
	"context"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"strings"
	"testing"
)

func TestBuildSolicitudFactura(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}

	if req.CodigoAmbiente != 2 {
		t.Errorf("CodigoAmbiente=%d, se esperaba 2 (piloto)", req.CodigoAmbiente)
	}
	if req.Nit != "9971522011" {
		t.Errorf("Nit=%q", req.Nit)
	}
	if req.NumeroFactura != 1 {
		t.Errorf("NumeroFactura=%d", req.NumeroFactura)
	}
	if req.Cuis != "D17EEF19" {
		t.Errorf("Cuis=%q", req.Cuis)
	}
	if req.Cufd != "CUFD-XYZ" || req.CodigoControl != "CC-123" {
		t.Errorf("Cufd/CodigoControl=%q/%q", req.Cufd, req.CodigoControl)
	}
	if req.CodigoSucursal != 0 || req.CodigoPuntoVenta != 3 {
		t.Errorf("sucursal/PV=%d/%d", req.CodigoSucursal, req.CodigoPuntoVenta)
	}
	if req.Cliente.CodigoTipoDocumentoIdentidad != 1 {
		t.Errorf("tipoDocumento=%d", req.Cliente.CodigoTipoDocumentoIdentidad)
	}
	custID := ""
	if req.Cliente.CodigoCliente != nil {
		custID = *req.Cliente.CodigoCliente
	}
	if req.Cliente.NumeroDocumento != "1234567" || custID != "CI1234567" {
		t.Errorf("cliente documento/codigo=%q/%q", req.Cliente.NumeroDocumento, custID)
	}
	if len(req.Items) != 1 {
		t.Fatalf("items=%d", len(req.Items))
	}
	it := req.Items[0]
	if it.CodigoProductoSin != 5113100 {
		t.Errorf("CodigoProductoSin=%d", it.CodigoProductoSin)
	}
	if it.UnidadMedida != 58 {
		t.Errorf("UnidadMedida=%d", it.UnidadMedida)
	}
	if it.ActividadEconomica != "101010" {
		t.Errorf("ActividadEconomica=%q", it.ActividadEconomica)
	}
	if it.Cantidad != 2 || it.PrecioUnitario != 100 || it.SubTotal != 200 {
		t.Errorf("cantidad/precio/subtotal=%v/%v/%v", it.Cantidad, it.PrecioUnitario, it.SubTotal)
	}
	if req.MontoTotal != 200 {
		t.Errorf("MontoTotal=%v", req.MontoTotal)
	}
	if req.RazonSocialEmisor != "EMPRESA PILOTO SRL" {
		t.Errorf("RazonSocialEmisor=%q", req.RazonSocialEmisor)
	}
}

func TestBuildSolicitudFacturaUsaSiatCode(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	siatsCode := 7
	inv.PointOfSale.SiatCode = &siatsCode

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	if req.CodigoPuntoVenta != 7 {
		t.Errorf("CodigoPuntoVenta=%d, se esperaba 7 (SiatCode)", req.CodigoPuntoVenta)
	}
}

func TestBuildSolicitudFacturaSinCodigoProductoSin(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.Items[0].CodigoProductoSin = nil

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil || !strings.Contains(err.Error(), "codigo_producto_sin") {
		t.Fatalf("se esperaba error de codigo_producto_sin, se obtuvo: %v", err)
	}
}

func TestBuildSolicitudFacturaTipoDocInvalido(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.Customer.DocumentType = "XYZ"

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("se esperaba error por tipo de documento inválido")
	}
}

func TestBuildSolicitudFacturaFSEDU(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CodigoDocumentoSector = 11
	inv.CodigoTipoFactura = 1
	nombre := "María Fernanda Quispe"
	periodo := "GESTION 1/2026"
	inv.NombreEstudiante = &nombre
	inv.PeriodoFacturado = &periodo

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura FSEDU: %v", err)
	}
	if req.CodigoDocumentoSector != 11 {
		t.Errorf("CodigoDocumentoSector=%d, se esperaba 11", req.CodigoDocumentoSector)
	}
	if req.NombreEstudiante != nombre {
		t.Errorf("NombreEstudiante=%q", req.NombreEstudiante)
	}
	if req.PeriodoFacturado != periodo {
		t.Errorf("PeriodoFacturado=%q", req.PeriodoFacturado)
	}
}

func TestBuildSolicitudFacturaNoArrastraCamposEducativosFueraDeSector11(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CodigoDocumentoSector = 1
	nombre := "No debe viajar"
	periodo := "2026-1"
	inv.NombreEstudiante = &nombre
	inv.PeriodoFacturado = &periodo

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	// Nuevo contrato: el usecase siempre pasa los campos legados y la capa siat
	// los ignora fuera de los sectores educativos (11/46): los datos específicos
	// resultantes deben quedar vacíos para compraventa.
	perfil, err := siat.PerfilSector(1)
	if err != nil {
		t.Fatalf("PerfilSector(1): %v", err)
	}
	valores, err := perfil.PrepararDatosSector(toSiatSolicitudFactura(*req))
	if err != nil {
		t.Fatalf("PrepararDatosSector: %v", err)
	}
	if len(valores) != 0 {
		t.Errorf("datos_sector normalizados=%v, se esperaba vacío fuera del sector 11", valores)
	}
}

func TestBuildSolicitudFacturaDireccionPadron(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CufdRecord.Direccion = "URBANIZACION: LA FLORIDA, AVENIDA: AROMA, NRO.: 364"

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	if req.Direccion != inv.CufdRecord.Direccion {
		t.Errorf("Direccion=%q, se esperaba la del padrón (CUFD)", req.Direccion)
	}
}
