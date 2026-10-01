package siat

import "testing"

func TestTotalesFixtureBaseCoincidenConTotalFiscal(t *testing.T) {
	items := []ItemFactura{
		{Cantidad: 1, PrecioUnitario: 100.25, SubTotal: 100.25},
		{Cantidad: 2, PrecioUnitario: 25.125, SubTotal: 50.25},
	}

	total, sujetoIVA := CalcularTotales(items, false)
	if total != 150.50 || sujetoIVA != 150.50 {
		t.Fatalf("total=%0.2f sujeto_iva=%0.2f; want 150.50", total, sujetoIVA)
	}
}

func TestCalcularSubtotalEliminaRuidoBinario(t *testing.T) {
	if got := CalcularSubtotal(3, 0.1, nil); got != 0.30 {
		t.Fatalf("got %.17f, want 0.30", got)
	}
}
