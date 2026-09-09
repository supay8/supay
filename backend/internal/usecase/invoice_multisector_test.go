package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
)

func requiredSectorData(t *testing.T, fields []siat.CampoSector) json.RawMessage {
	t.Helper()
	values := map[string]any{}
	for _, field := range fields {
		if !field.Requerido {
			continue
		}
		switch field.Tipo {
		case "int":
			values[field.JSON] = 1
		case "float":
			values[field.JSON] = 1.0
		case "fecha":
			values[field.JSON] = "2026-08-24"
		case "json":
			values[field.JSON] = map[string]any{}
		case "bool":
			values[field.JSON] = true
		default:
			values[field.JSON] = "PRUEBA"
		}
	}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func seedOriginalInvoice(repo *fakeInvoiceRepo) string {
	id := "original-1"
	repo.invoices[id] = &domain.Invoice{
		ID: id, CompanyId: "comp-1", CustomerId: "cust-1", InvoiceNumber: 42,
		Cuf: strPtr("CUF-ORIGINAL"), IssueDate: time.Date(2026, 8, 24, 10, 0, 0, 0, siat.LaPaz),
		Total: 2500, Subtotal: 2500, CodigoDocumentoSector: 1, Status: domain.InvoiceAccepted,
		Items: []domain.InvoiceItem{{ProductID: strPtr("product-1"), Code: "SKU-001", Description: "Producto original",
			CodigoActividad: strPtr("101010"), CodigoProductoSin: strPtr("5113100"), UnitCode: intPtr(58),
			Quantity: 2, UnitPrice: 1250, Subtotal: 2500}},
	}
	return id
}

func TestV1PreviewAndCreateAllSDKProfiles(t *testing.T) {
	for _, p := range siat.PerfilesSector() {
		if !p.HasBuilder() {
			continue
		}
		for _, mode := range []int{siat.ModalidadElectronica, siat.ModalidadComputarizada} {
			if p.ValidarModalidad(mode) != nil {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s/%d", p.Codigo, p.Layout, mode), func(t *testing.T) {
				uc, repo, _, productRepo := createTestUsecaseBuilder()
				uc.modalidad = mode
				input := minimalTestInvoice()
				input.Sector, input.Layout = strconv.Itoa(p.Codigo), p.Layout
				input.Data = requiredSectorData(t, p.Campos)
				input.Items[0].Data = requiredSectorData(t, p.CamposDetalle)
				productRepo.product.Mappings[0].CodigoDocumentoSector = p.Codigo
				if p.EsAjuste() {
					ref := seedOriginalInvoice(repo)
					input.ReferenceInvoiceID = &ref
					var values map[string]any
					_ = json.Unmarshal(input.Data, &values)
					for _, key := range []string{"numero_autorizacion_cuf", "fecha_emision_factura", "monto_total_original"} {
						delete(values, key)
					}
					input.Data, _ = json.Marshal(values)
					productRepo.product.Mappings = nil // no remapping required for notes
				}
				if p.Codigo == 30 {
					total := 24.0
					input.Items = nil
					input.Total = &total
				}
				before := len(repo.invoices)
				preview, err := uc.PreviewSimplified(t.Context(), input)
				if err != nil {
					t.Fatal(err)
				}
				if len(repo.invoices) != before {
					t.Fatal("preview persisted an invoice")
				}
				invoice, err := uc.CreateSimplified(t.Context(), input, "")
				if err != nil {
					t.Fatal(err)
				}
				if invoice.Total != preview.Total || invoice.CodigoDocumentoSector != p.Codigo || invoice.Modalidad != mode {
					t.Fatalf("preview/create mismatch: preview=%+v invoice=%+v", preview, invoice)
				}
			})
		}
	}
}

func TestV1Sector47UsesOriginalFiscalSnapshot(t *testing.T) {
	uc, repo, _, products := createTestUsecaseBuilder()
	ref := seedOriginalInvoice(repo)
	products.product.Mappings = nil
	input := minimalTestInvoice()
	input.Sector = "47"
	input.ReferenceInvoiceID = &ref
	input.Items = []MinimalInvoiceItem{{SKU: "SKU-001", Quantity: 1, Price: 1250}}
	preview, err := uc.PreviewSimplified(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(preview.Data, &data); err != nil {
		t.Fatal(err)
	}
	if preview.Total != 1250 || data["monto_total_original"] != float64(2500) || data["monto_total_devuelto"] != float64(1250) || data["monto_efectivo_credito_debito"] != 162.5 {
		t.Fatalf("partial return values: %+v / %s", preview, preview.Data)
	}
	if preview.Items[0].Description != "Producto original" {
		t.Fatal("used current catalog instead of snapshot")
	}
	invoice, err := uc.CreateSimplified(t.Context(), input, "adjustment-47")
	if err != nil {
		t.Fatal(err)
	}
	if invoice.AjustaFacturaId == nil || *invoice.AjustaFacturaId != ref {
		t.Fatal("reference lost")
	}
}

func TestV1RejectsInvalidSectorInputsAsValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*MinimalInvoiceRequest)
		message string
	}{
		{"missing_reference", func(r *MinimalInvoiceRequest) { r.Sector = "47" }, "reference_invoice_id"},
		{"missing_layout", func(r *MinimalInvoiceRequest) { r.Sector = "24" }, "layout"},
		{"required_header", func(r *MinimalInvoiceRequest) { r.Sector = "11" }, "nombre_estudiante"},
		{"single_item", func(r *MinimalInvoiceRequest) { r.Sector = "23"; r.Items = append(r.Items, r.Items[0]) }, "exactamente un"},
		{"payment", func(r *MinimalInvoiceRequest) { r.Payment = &MinimalInvoicePayment{} }, "payment"},
		{"unknown_data", func(r *MinimalInvoiceRequest) { r.Data = json.RawMessage(`{"typo":true}`) }, "no reconocidas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, _, _, products := createTestUsecaseBuilder()
			input := minimalTestInvoice()
			tc.change(&input)
			sector, _ := resolveSectorAlias(input.Sector)
			if sector > 0 {
				products.product.Mappings[0].CodigoDocumentoSector = sector
			}
			_, err := uc.PreviewSimplified(t.Context(), input)
			var bad *domain.BadRequestError
			if !errors.As(err, &bad) || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected validation %s, got %v", tc.message, err)
			}
		})
	}
}

func TestV1AdjustmentRejectsOtherTenantAndUnrelatedProduct(t *testing.T) {
	for _, otherTenant := range []bool{false, true} {
		uc, repo, _, _ := createTestUsecaseBuilder()
		ref := seedOriginalInvoice(repo)
		input := minimalTestInvoice()
		input.Sector = "47"
		input.ReferenceInvoiceID = &ref
		if otherTenant {
			repo.invoices[ref].CompanyId = "other-company"
		} else {
			input.Items[0].SKU = "OTHER"
		}
		if _, err := uc.CreateSimplified(t.Context(), input, ""); err == nil {
			t.Fatal("accepted unrelated reference/item")
		}
		if len(repo.invoices) != 1 {
			t.Fatal("invalid note persisted")
		}
	}
}

func TestV1PaymentAndDiscountPersistConsistently(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()
	input := minimalTestInvoice()
	input.Payment = &MinimalInvoicePayment{MethodCode: 2, CurrencyCode: 2, ExchangeRate: 6.96}
	input.Data = json.RawMessage(`{"descuento_adicional":4,"monto_gift_card":2}`)
	preview, err := uc.PreviewSimplified(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := uc.CreateSimplified(t.Context(), input, "")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Total != 20 || invoice.Total != 20 || invoice.CodigoMetodoPago != 2 || invoice.TipoCambio != 6.96 {
		t.Fatalf("inconsistent amounts: %+v / %+v", preview, invoice)
	}
}

func TestV1AdjustmentCopiesCompatibleDataAndKeepsDuplicateLines(t *testing.T) {
	uc, repo, _, _ := createTestUsecaseBuilder()
	ref := seedOriginalInvoice(repo)
	repo.invoices[ref].Items[0].SectorData = json.RawMessage(`{"detalle_huespedes":[{"nombre":"HUESPED"}]}`)
	input := minimalTestInvoice()
	input.Sector, input.ReferenceInvoiceID, input.Items = "47", &ref, nil
	preview, err := uc.PreviewSimplified(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Total != 2500 || len(preview.Items) != 1 {
		t.Fatalf("bad inherited invoice: %+v", preview)
	}
	input.Items = []MinimalInvoiceItem{{SKU: "SKU-001", Quantity: 1, Price: 100}, {SKU: "SKU-001", Quantity: 1, Price: 100}}
	invoice, err := uc.CreateSimplified(t.Context(), input, "")
	if err != nil {
		t.Fatal(err)
	}
	if invoice.Total != 200 || len(invoice.Items) != 2 {
		t.Fatal("identical lines silently collapsed")
	}
}

func TestV1AdjustmentRejectsAlteredOriginalMetadata(t *testing.T) {
	for _, data := range []string{
		`{"numero_autorizacion_cuf":"OTHER"}`,
		`{"monto_total_original":123}`,
		`{"fecha_emision_factura":"2026-08-24Tinvalid"}`,
	} {
		uc, repo, _, _ := createTestUsecaseBuilder()
		ref := seedOriginalInvoice(repo)
		input := minimalTestInvoice()
		input.Sector = "47"
		input.ReferenceInvoiceID = &ref
		input.Data = json.RawMessage(data)
		if _, err := uc.PreviewSimplified(t.Context(), input); err == nil {
			t.Fatalf("accepted altered reference: %s", data)
		}
	}
}

func TestV1BoletoIndividualFailsWithoutCreatingDraft(t *testing.T) {
	uc, repo, _, _ := createTestUsecaseBuilder()
	input := minimalTestInvoice()
	input.Sector = "30"
	if _, err := uc.EmitSimplified(t.Context(), input, ""); err == nil || !strings.Contains(err.Error(), "masiva") {
		t.Fatalf("expected bulk-only error: %v", err)
	}
	if len(repo.invoices) != 0 {
		t.Fatal("created draft for unsupported emission operation")
	}
}
