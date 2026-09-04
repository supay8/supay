package usecase

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/adapters/siat"
)

func TestBuildDatosSectorNotaDescuentoCompletaCampos(t *testing.T) {
	cuf := "CUF-ORIGINAL-1234567890123456789012345678901234567890123456789012345678901234567AA"
	ref := &domain.Invoice{
		Cuf:       &cuf,
		IssueDate: time.Date(2026, 8, 24, 15, 30, 0, 0, siat.LaPaz),
		Total:     100,
	}
	items := []CreateInvoiceItemRequest{{Quantity: 1, UnitPrice: 100}}

	raw, err := buildDatosSectorNotaDescuento(ref, nil, items)
	if err != nil {
		t.Fatalf("buildDatosSectorNotaDescuento: %v", err)
	}
	var datos map[string]any
	if err := json.Unmarshal(raw, &datos); err != nil {
		t.Fatal(err)
	}
	if datos["numero_autorizacion_cuf"] != cuf {
		t.Fatalf("cuf=%v", datos["numero_autorizacion_cuf"])
	}
	if datos["fecha_emision_factura"] != "2026-08-24" {
		t.Fatalf("fecha=%v", datos["fecha_emision_factura"])
	}
	if datos["monto_total_original"] != float64(100) {
		t.Fatalf("monto_total_original=%v", datos["monto_total_original"])
	}
	if datos["monto_total_devuelto"] != float64(100) {
		t.Fatalf("monto_total_devuelto=%v", datos["monto_total_devuelto"])
	}
	if datos["monto_efectivo_credito_debito"] != float64(13) {
		t.Fatalf("monto_efectivo_credito_debito=%v, se esperaba 13 (13%% de 100)", datos["monto_efectivo_credito_debito"])
	}
}

func TestAutofillDocumentoAjusteDescuentoCopiaItemsYDatosSector(t *testing.T) {
	cuf := "CUF-ORIGINAL"
	refID := "ref-inv-1"
	repo := newFakeInvoiceRepo()
	repo.invoices[refID] = &domain.Invoice{
		ID:            refID,
		CompanyId:     "comp-1",
		Cuf:           &cuf,
		InvoiceNumber: 42,
		IssueDate:     time.Date(2026, 8, 24, 10, 0, 0, 0, siat.LaPaz),
		Total:         200,
		Subtotal:      200,
		Items: []domain.InvoiceItem{
			{
				Code:              "P-001",
				Description:       "Servicio",
				CodigoActividad:   strPtr("620100"),
				CodigoProductoSin: strPtr("83141"),
				UnitCode:          intPtr(1),
				Quantity:          2,
				UnitPrice:         100,
				Subtotal:          200,
			},
		},
	}
	uc := &InvoiceUsecase{invoiceRepo: repo}
	refIDCopy := refID
	req := CreateInvoiceRequest{
		CompanyId:             "comp-1",
		CodigoDocumentoSector: sectorNotaCreditoDebitoDescuentos,
		ReferenciaFacturaId:   &refIDCopy,
	}

	ref, err := uc.autofillDocumentoAjusteDescuento(&req, "comp-1")
	if err != nil {
		t.Fatalf("autofill: %v", err)
	}
	if ref == nil || ref.ID != refID {
		t.Fatalf("ref=%+v", ref)
	}
	if len(req.Items) != 1 || req.Items[0].Code != "P-001" {
		t.Fatalf("items no copiados: %+v", req.Items)
	}

	var datos map[string]any
	if err := json.Unmarshal(req.DatosSector, &datos); err != nil {
		t.Fatal(err)
	}
	if datos["monto_total_original"] != float64(200) {
		t.Fatalf("monto_total_original=%v", datos["monto_total_original"])
	}
	if datos["monto_efectivo_credito_debito"] != float64(26) {
		t.Fatalf("monto_efectivo_credito_debito=%v", datos["monto_efectivo_credito_debito"])
	}
}

func TestAutofillDocumentoAjusteDescuentoRespetaCamposProveidos(t *testing.T) {
	cuf := "CUF-ORIGINAL"
	refID := "ref-inv-2"
	repo := newFakeInvoiceRepo()
	repo.invoices[refID] = &domain.Invoice{
		ID:            refID,
		CompanyId:     "comp-1",
		Cuf:           &cuf,
		InvoiceNumber: 10,
		IssueDate:     time.Date(2026, 8, 24, 10, 0, 0, 0, siat.LaPaz),
		Total:         100,
	}
	uc := &InvoiceUsecase{invoiceRepo: repo}
	refIDCopy := refID
	datosExistentes := json.RawMessage(`{"monto_total_devuelto":50,"monto_efectivo_credito_debito":6.5}`)
	req := CreateInvoiceRequest{
		CompanyId:             "comp-1",
		CodigoDocumentoSector: sectorNotaCreditoDebitoDescuentos,
		ReferenciaFacturaId:   &refIDCopy,
		DatosSector:           datosExistentes,
		Items: []CreateInvoiceItemRequest{
			{Code: "X", Description: "Item", Quantity: 1, UnitPrice: 50},
		},
	}

	if _, err := uc.autofillDocumentoAjusteDescuento(&req, "comp-1"); err != nil {
		t.Fatalf("autofill: %v", err)
	}
	var datos map[string]any
	if err := json.Unmarshal(req.DatosSector, &datos); err != nil {
		t.Fatal(err)
	}
	if datos["monto_total_devuelto"] != float64(50) {
		t.Fatalf("monto_total_devuelto=%v", datos["monto_total_devuelto"])
	}
	if datos["monto_efectivo_credito_debito"] != float64(6.5) {
		t.Fatalf("monto_efectivo_credito_debito=%v", datos["monto_efectivo_credito_debito"])
	}
	if datos["numero_autorizacion_cuf"] != cuf {
		t.Fatalf("cuf no autocompletado: %v", datos["numero_autorizacion_cuf"])
	}
}
