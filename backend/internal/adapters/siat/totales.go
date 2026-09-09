package siat

import (
	"encoding/json"
	"fmt"
	"log/slog"
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

// CalcularTotales suma los subtotales estrictos de todos los ítems.
// Si esTasaCero == true, montoSujetoIva retorna 0 (sector tasa cero).
// Auto-corrección: los callers deben usar el retorno como fuente de verdad
// para cabecera.montoTotal / montoTotalSujetoIva.
func CalcularTotales(items []ItemFactura, esTasaCero bool) (montoTotal, montoSujetoIva float64) {
	var sum float64
	for _, it := range items {
		// Preferir SubTotal ya almacenado si es consistente, pero recalcular
		// estricto para evitar valores quemados (ej. 100.00 fijo).
		// Si SubTotal viene 0 pero cantidad*precio >0, el recalculo lo corrige.
		esperado := CalcularSubtotal(it.Cantidad, it.PrecioUnitario, it.MontoDescuento)
		sub := it.SubTotal
		if sub == 0 && esperado != 0 {
			sub = esperado
		} else {
			// Detectar desfase por redondeo o frontend erróneo
			if round2(sub) != esperado {
				slog.Warn("totales: subtotal desalineado, auto-corregido",
					"descripcion", it.Descripcion,
					"subtotal_enviado", sub,
					"subtotal_esperado", esperado,
				)
				sub = esperado
			} else {
				sub = round2(sub)
			}
		}
		sum += sub
	}
	sum = round2(sum)
	if esTasaCero {
		return sum, 0
	}
	return sum, sum
}

// NormalizarTotales corrige req.MontoTotal y cada item.SubTotal al cálculo
// estricto, emitiendo Warn si hubo desfase. Retorna el req corregido.
func NormalizarTotales(req SolicitudFactura, esTasaCero bool) SolicitudFactura {
	if len(req.Items) == 0 {
		return req
	}
	// Normalizar ítems primero
	for i := range req.Items {
		esperado := CalcularSubtotal(req.Items[i].Cantidad, req.Items[i].PrecioUnitario, req.Items[i].MontoDescuento)
		if round2(req.Items[i].SubTotal) != esperado {
			slog.Warn("totales: subtotal item auto-corregido",
				"nro", i+1,
				"descripcion", req.Items[i].Descripcion,
				"anterior", req.Items[i].SubTotal,
				"corregido", esperado,
			)
			req.Items[i].SubTotal = esperado
		} else {
			req.Items[i].SubTotal = round2(req.Items[i].SubTotal)
		}
	}
	total, _ := CalcularTotales(req.Items, false)
	if round2(req.MontoTotal) != total {
		slog.Warn("totales: montoTotal auto-corregido",
			"montoTotal_previo", req.MontoTotal,
			"montoTotal_corregido", total,
			"items", len(req.Items),
		)
		req.MontoTotal = total
	}
	return req
}
