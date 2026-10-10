package usecase

import (
	"context"
	"errors"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"testing"
)

func TestInvoiceUsecaseDeniesCrossTenantDirectCalls(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	repo.invoices[inv.ID] = inv
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, &fakeEmissionService{})
	wrongTenant := invoiceContextFor("comp-2")

	operations := map[string]func() error{
		"GetByID": func() error {
			_, err := uc.GetByID(wrongTenant, inv.ID)
			return err
		},
		"Emit": func() error {
			_, err := uc.Emit(wrongTenant, inv.ID)
			return err
		},
		"ProcessEmission": func() error {
			_, err := uc.ProcessEmission(context.Background(), "comp-2", inv.ID)
			return err
		},
		"VerifyStatus": func() error {
			_, err := uc.VerifyStatus(wrongTenant, inv.ID)
			return err
		},
		"Annul": func() error {
			_, err := uc.Annul(wrongTenant, inv.ID, 1)
			return err
		},
		"RevertAnnul": func() error {
			_, err := uc.RevertAnnul(wrongTenant, inv.ID)
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			var notFound *domain.NotFoundError
			if err := operation(); !errors.As(err, &notFound) {
				t.Fatalf("se esperaba not found sin revelar pertenencia, got %T: %v", err, err)
			}
		})
	}
	if repo.claimCalls != 0 || inv.Status != domain.InvoicePending {
		t.Fatalf("una llamada cross-tenant intentó mutar la factura: claims=%d status=%s", repo.claimCalls, inv.Status)
	}
	if _, err := uc.GetByID(context.Background(), inv.ID); !errors.Is(err, domain.ErrMissingCompanyID) {
		t.Fatalf("GetByID sin tenant debe fallar cerrado: %v", err)
	}
}

func TestListInvoices(t *testing.T) {
	repo := newFakeInvoiceRepo()
	uc := NewInvoiceUsecase(repo, nil, nil, nil, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	aceptada := testInvoice()
	aceptada.ID = "inv-ok"
	aceptada.Status = domain.InvoiceAccepted
	_ = repo.Create(aceptada)
	pending := testInvoice()
	pending.ID = "inv-pending"
	pending.Status = domain.InvoicePending
	_ = repo.Create(pending)

	t.Run("sin filtro de estado devuelve todo", func(t *testing.T) {
		list, total, err := uc.ListInvoices(domain.InvoiceListFilter{TenantID: "comp-1", PointOfSaleID: "pos-1"})
		if err != nil || total != 2 || len(list) != 2 {
			t.Fatalf("list=%d total=%d err=%v", len(list), total, err)
		}
	})

	t.Run("filtra por estado", func(t *testing.T) {
		status := domain.InvoiceAccepted
		list, total, err := uc.ListInvoices(domain.InvoiceListFilter{TenantID: "comp-1", PointOfSaleID: "pos-1", Status: &status})
		if err != nil || total != 1 || len(list) != 1 || list[0].ID != "inv-ok" {
			t.Fatalf("list=%d total=%d err=%v", len(list), total, err)
		}
	})

	t.Run("point_of_sale_id obligatorio", func(t *testing.T) {
		if _, _, err := uc.ListInvoices(domain.InvoiceListFilter{}); err == nil {
			t.Fatal("se esperaba error de validación")
		}
	})

	t.Run("estado inválido rechazado", func(t *testing.T) {
		status := domain.InvoiceStatus("NO_EXISTE")
		if _, _, err := uc.ListInvoices(domain.InvoiceListFilter{TenantID: "comp-1", PointOfSaleID: "pos-1", Status: &status}); err == nil {
			t.Fatal("se esperaba error por estado inválido")
		}
	})

	t.Run("paginación respeta limit/offset", func(t *testing.T) {
		list, total, err := uc.ListInvoices(domain.InvoiceListFilter{TenantID: "comp-1", PointOfSaleID: "pos-1", Limit: 1, Offset: 1})
		if err != nil || total != 2 || len(list) != 1 {
			t.Fatalf("list=%d total=%d err=%v", len(list), total, err)
		}
	})
}
