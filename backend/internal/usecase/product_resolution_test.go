package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
)

type fakeProductRepository struct {
	product *domain.Product
}

func (f *fakeProductRepository) Create(*domain.Product) error { return nil }
func (f *fakeProductRepository) GetByID(_, _ string) (*domain.Product, error) {
	if f.product == nil {
		return nil, errors.New("not found")
	}
	return f.product, nil
}
func (f *fakeProductRepository) GetBySKU(_, _ string) (*domain.Product, error) {
	return f.GetByID("", "")
}
func (f *fakeProductRepository) List(string) ([]*domain.Product, error) {
	if f.product == nil {
		return nil, nil
	}
	return []*domain.Product{f.product}, nil
}
func (f *fakeProductRepository) UpsertMapping(string, domain.ProductMapping) error { return nil }

func TestResolveProductMappingsUsesFiscalSnapshot(t *testing.T) {
	product := &domain.Product{
		ID: "product-1", SKU: "SKU-001", Active: true,
		Mappings: []domain.ProductMapping{{
			ProductID: "product-1", CodigoProductoSin: 12345, CodigoActividad: "473000", CodigoDocumentoSector: siat.SectorCompraVenta, UnidadMedida: 1, Active: true, SyncedAt: time.Now(),
		}},
	}
	uc := &InvoiceUsecase{productRepo: &fakeProductRepository{product: product}}
	mappings, products, ids, codes, err := uc.resolveProductMappings(CreateInvoiceRequest{
		CompanyId: "company-1",
		Items:     []CreateInvoiceItemRequest{{ProductID: "product-1", Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mappings[0].CodigoProductoSin != 12345 || mappings[0].CodigoActividad != "473000" {
		t.Fatalf("mapeo inesperado: %+v", mappings[0])
	}
	if products[0] == nil || products[0].ID != "product-1" {
		t.Fatalf("producto no resuelto: %+v", products[0])
	}
	if ids[0] == nil || *ids[0] != "product-1" || codes[0] != "SKU-001" {
		t.Fatalf("identidad del producto no resuelta: ids=%v codes=%v", ids, codes)
	}
}

func TestResolveInvoiceSectorRejectsAmbiguousProducts(t *testing.T) {
	uc := &InvoiceUsecase{}
	_, err := uc.resolveInvoiceSector("sale", map[int]bool{1: true, 11: true}, &domain.Company{ID: "company-1"})
	if err == nil {
		t.Fatal("se esperaba rechazo por sectores ambiguos")
	}
}
