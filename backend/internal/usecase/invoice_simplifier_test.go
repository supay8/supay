package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/brandsrx/supay/internal/adapters/siat"
)

func TestMinimalInvoiceRequestTieneSeisCamposPublicos(t *testing.T) {
	if got := reflect.TypeOf(MinimalInvoiceRequest{}).NumField(); got != 6 {
		t.Fatalf("MinimalInvoiceRequest tiene %d campos; se esperaban 6", got)
	}
}

func minimalTestInvoice() MinimalInvoiceRequest {
	return MinimalInvoiceRequest{
		PointOfSaleID: "pos-1",
		Customer: MinimalInvoiceCustomer{
			DocumentType:   "cédula",
			DocumentNumber: "1234567",
		},
		Items:       []MinimalInvoiceItem{{SKU: "SKU-001", Quantity: 2, Price: 12.50, Discount: 1}},
		InvoiceType: "venta",
		Sector:      "auto",
	}
}

func TestInvoiceRequestSimplifierAutocompletaClienteProductoYSiat(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()
	result, err := NewInvoiceRequestSimplifier(uc).Simplify(context.Background(), minimalTestInvoice())
	if err != nil {
		t.Fatalf("Simplify() error: %v", err)
	}

	if result.Request.CustomerId != "cust-1" || result.Preview.Customer.Name != "Juan Perez" {
		t.Fatalf("cliente no autocompletado: request=%q preview=%+v", result.Request.CustomerId, result.Preview.Customer)
	}
	if result.Preview.CodigoDocumentoSector != siat.SectorCompraVenta {
		t.Fatalf("sector=%d", result.Preview.CodigoDocumentoSector)
	}
	if len(result.Preview.Items) != 1 {
		t.Fatalf("items=%d", len(result.Preview.Items))
	}
	item := result.Preview.Items[0]
	if item.ProductID != "product-1" || item.Description != "Producto de prueba mapeado" || item.CodigoProductoSin != 5113100 || item.UnidadMedida != 58 {
		t.Fatalf("producto/SIAT no autocompletado: %+v", item)
	}
	if result.Preview.Total != 24 {
		t.Fatalf("total=%v, se esperaba 24", result.Preview.Total)
	}
}

func TestInvoiceRequestSimplifierNoPersisteDurantePreview(t *testing.T) {
	uc, repo, customers, _ := createTestUsecaseBuilder()
	request := minimalTestInvoice()
	request.Customer.DocumentNumber = "nuevo-1"
	request.Customer.Name = "Cliente Nuevo"

	preview, err := uc.PreviewSimplified(context.Background(), request)
	if err != nil {
		t.Fatalf("PreviewSimplified() error: %v", err)
	}
	if preview.Customer.ID != "" || len(customers.created) != 0 || len(repo.invoices) != 0 {
		t.Fatalf("preview produjo escrituras: customer=%q customers=%d invoices=%d", preview.Customer.ID, len(customers.created), len(repo.invoices))
	}
}

func TestCreateSimplifiedPersisteSolicitudNormalizada(t *testing.T) {
	uc, repo, _, _ := createTestUsecaseBuilder()
	invoice, err := uc.CreateSimplified(context.Background(), minimalTestInvoice(), "order-v1-1")
	if err != nil {
		t.Fatalf("CreateSimplified() error: %v", err)
	}
	if invoice.IdempotencyKey == nil || *invoice.IdempotencyKey != "order-v1-1" {
		t.Fatalf("idempotency key no propagada: %+v", invoice.IdempotencyKey)
	}
	if len(repo.invoices) != 1 || len(invoice.Items) != 1 || invoice.Items[0].Description == "" {
		t.Fatalf("factura normalizada no persistida: invoices=%d invoice=%+v", len(repo.invoices), invoice)
	}
}

func TestInvoiceAliases(t *testing.T) {
	if got, err := normalizeDocumentType("pasaporte"); err != nil || got != "PAS" {
		t.Fatalf("document alias=%q err=%v", got, err)
	}
	if got, err := normalizeInvoiceType("educación"); err != nil || got != "education" {
		t.Fatalf("invoice alias=%q err=%v", got, err)
	}
	if got, err := resolveSectorAlias("educación"); err != nil || got != 11 {
		t.Fatalf("sector alias=%d err=%v", got, err)
	}
}
