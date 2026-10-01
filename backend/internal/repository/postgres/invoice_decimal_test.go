package postgres

import (
	"testing"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/shopspring/decimal"
)

func TestToModelInvoiceRedondeaSegunEscalaFiscal(t *testing.T) {
	inv := &domain.Invoice{
		TipoCambio: 6.965555,
		Subtotal:   100.25 + 2*25.125,
		Discount:   0.005,
		Total:      0.1 + 0.2,
		Items: []domain.InvoiceItem{{
			Quantity:  1.234567,
			UnitPrice: 2.345678,
			Discount:  0.005,
			Subtotal:  2.899,
		}},
	}

	got := toModelInvoice(inv)
	checks := []struct {
		name string
		got  decimal.Decimal
		want string
	}{
		{"tipo de cambio", got.TipoCambio, "6.96556"},
		{"subtotal", got.Subtotal, "150.50"},
		{"descuento", got.Discount, "0.01"},
		{"total sin ruido binario", got.Total, "0.30"},
		{"cantidad", got.Items[0].Quantity, "1.23457"},
		{"precio unitario", got.Items[0].UnitPrice, "2.34568"},
		{"descuento de línea", got.Items[0].Discount, "0.01"},
		{"subtotal de línea", got.Items[0].Subtotal, "2.90"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !check.got.Equal(decimal.RequireFromString(check.want)) {
				t.Fatalf("got %s, want %s", check.got, check.want)
			}
		})
	}
}
