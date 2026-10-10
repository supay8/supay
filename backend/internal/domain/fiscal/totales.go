package fiscal

import (
	"encoding/json"
	"fmt"
	"math"
)

// TotalDocumento applies the header discount once, after the line discounts.
// Shared by preview, persistence and SDK construction.
func TotalDocumento(subtotal float64, data json.RawMessage) (float64, error) {
	values := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &values); err != nil {
			return 0, fmt.Errorf("data debe ser un objeto JSON")
		}
	}
	discount, gift := 0.0, 0.0
	for _, key := range []string{"descuento_adicional", "monto_gift_card", "monto_total_sujeto_iva"} {
		value, present := values[key]
		if !present || value == nil {
			continue
		}
		number, ok := toFloat(value)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
			return 0, fmt.Errorf("data.%s debe ser un número no negativo", key)
		}
		if key == "descuento_adicional" {
			discount = round2(number)
		}
		if key == "monto_gift_card" {
			gift = round2(number)
		}
	}
	total := round2(subtotal - discount)
	if total < 0 {
		return 0, fmt.Errorf("data.descuento_adicional supera el subtotal")
	}
	if gift > total {
		return 0, fmt.Errorf("data.monto_gift_card supera el total")
	}
	return total, nil
}

// round2 redondea a 2 decimales (centavos bolivianos) — autoridad única para
// cálculos monetarios del SIAT.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// CompraVenta's SDK setters preserve five decimals for quantity/price and
// two for discounts. Apply the same precision before calculating totals.
func NormalizarImportesItem(sector int, quantity, price, discount float64) (float64, float64, float64) {
	if sector == SectorCompraVenta {
		return math.Round(quantity*1e5) / 1e5, math.Round(price*1e5) / 1e5, round2(discount)
	}
	return quantity, price, discount
}

// Round2ForCompare exportado para comparación en usecases (mantiene 2 decimales).
func Round2ForCompare(v float64) float64 { return round2(v) }

// CalcularSubtotal calcula el subtotal estricto de un ítem: quantity*unitPrice - discount.
// discount nil se trata como 0. Resultado redondeado a 2 decimales.
func CalcularSubtotal(cantidad, precioUnitario float64, descuento *float64) float64 {
	d := 0.0
	if descuento != nil {
		d = *descuento
	}
	return round2(cantidad*precioUnitario - d)
}
