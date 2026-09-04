package usecase

import (
	"encoding/json"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/adapters/siat"
)

const (
	sectorNotaCreditoDebitoDescuentos = 47
	sectorNotaCreditoDebitoIce        = 48
	tasaIVACreditoFiscal              = 0.13
)

// autofillDocumentoAjusteDescuento completa ítems y datos_sector de una nota
// sector 47/48 a partir de la factura referenciada cuando el cliente no los envía.
// Devuelve la factura original cargada para reutilizar validaciones posteriores.
// Incluye deduplicación legacy: si el cliente envió 2 ítems idénticos para
// bypassear el XSD minOccurs=2, se colapsan a 1 ítem lógico antes de construir
// datos_sector, ya que builder_reflex generará el par 1/2 automáticamente.
func (uc *InvoiceUsecase) autofillDocumentoAjusteDescuento(req *CreateInvoiceRequest, companyID string) (*domain.Invoice, error) {
	if req == nil || req.ReferenciaFacturaId == nil || strings.TrimSpace(*req.ReferenciaFacturaId) == "" {
		return nil, nil
	}
	if req.CodigoDocumentoSector != sectorNotaCreditoDebitoDescuentos && req.CodigoDocumentoSector != sectorNotaCreditoDebitoIce {
		return nil, nil
	}

	refID := strings.TrimSpace(*req.ReferenciaFacturaId)
	ref, err := uc.invoiceRepo.GetByID(refID)
	if err != nil {
		return nil, domain.NewNotFoundError("la factura referenciada no existe")
	}
	if ref.CompanyId != companyID {
		return nil, domain.NewBadRequestError("la factura referenciada pertenece a otra empresa")
	}
	if ref.Cuf == nil || strings.TrimSpace(*ref.Cuf) == "" {
		return nil, domain.NewConflictError("la factura referenciada aún no tiene cuf; emítala antes de ajustarla")
	}
	if ref.InvoiceNumber <= 0 {
		return nil, domain.NewConflictError("la factura referenciada no tiene número correlativo válido; no se puede crear el ajuste")
	}

	if len(req.Items) == 0 {
		req.Items = cloneItemsFromReferencia(ref.Items)
	} else {
		// Deduplicación legacy para sectores con DetallePar (47/48): si el cliente
		// envió el par manual (2 ítems idénticos), colapsar a ítems lógicos.
		if dedup := deduplicarItemsPar(req.Items); len(dedup) < len(req.Items) {
			req.Items = dedup
		}
	}

	datos, err := buildDatosSectorNotaDescuento(ref, req.DatosSector, req.Items)
	if err != nil {
		return nil, err
	}
	req.DatosSector = datos

	return ref, nil
}

func cloneItemsFromReferencia(items []domain.InvoiceItem) []CreateInvoiceItemRequest {
	out := make([]CreateInvoiceItemRequest, 0, len(items))
	for _, it := range items {
		item := CreateInvoiceItemRequest{
			Code:        it.Code,
			Description: it.Description,
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
			Discount:    it.Discount,
			SectorData:  it.SectorData,
		}
		if it.ProductID != nil {
			item.ProductID = *it.ProductID
		}
		item.CodigoActividad = it.CodigoActividad
		item.CodigoProductoSin = it.CodigoProductoSin
		item.UnitCode = it.UnitCode
		out = append(out, item)
	}
	return out
}

func buildDatosSectorNotaDescuento(ref *domain.Invoice, existing json.RawMessage, items []CreateInvoiceItemRequest) (json.RawMessage, error) {
	valores := map[string]any{}
	if len(existing) > 0 && string(existing) != "null" {
		if err := json.Unmarshal(existing, &valores); err != nil {
			return nil, domain.NewBadRequestError("datos_sector no es un objeto JSON válido")
		}
	}

	if _, ok := valores["numero_autorizacion_cuf"]; !ok {
		valores["numero_autorizacion_cuf"] = strings.TrimSpace(*ref.Cuf)
	}
	if _, ok := valores["fecha_emision_factura"]; !ok {
		valores["fecha_emision_factura"] = ref.IssueDate.In(siat.LaPaz).Format("2006-01-02")
	}

	montoOriginal := ref.Total
	if v, ok := valores["monto_total_original"]; ok {
		montoOriginal = jsonFloat(v)
	} else {
		valores["monto_total_original"] = round2(montoOriginal)
	}

	montoDevuelto := round2(montoOriginal)
	if v, ok := valores["monto_total_devuelto"]; ok {
		montoDevuelto = round2(jsonFloat(v))
	} else if len(items) > 0 {
		montoDevuelto = round2(sumItemSubtotals(items))
		valores["monto_total_devuelto"] = montoDevuelto
	} else {
		valores["monto_total_devuelto"] = montoDevuelto
	}

	if _, ok := valores["monto_efectivo_credito_debito"]; !ok {
		valores["monto_efectivo_credito_debito"] = round2(montoDevuelto * tasaIVACreditoFiscal)
	}

	return json.Marshal(valores)
}

func sumItemSubtotals(items []CreateInvoiceItemRequest) float64 {
	var total float64
	for _, it := range items {
		total += round2(it.Quantity*it.UnitPrice - it.Discount)
	}
	return round2(total)
}

func jsonFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func deduplicarItemsPar(items []CreateInvoiceItemRequest) []CreateInvoiceItemRequest {
	if len(items) < 2 {
		return items
	}
	// Solo deduplica cuando el número de ítems es par y cada par consecutivo
	// son idénticos (bypass legacy minOccurs=2). Ej: [A,A] -> [A], [A,A,B,B] -> [A,B].
	if len(items)%2 != 0 {
		return items
	}
	dedup := make([]CreateInvoiceItemRequest, 0, len(items)/2)
	for i := 0; i < len(items); i += 2 {
		a, b := items[i], items[i+1]
		if !itemsIguales(a, b) {
			return items
		}
		dedup = append(dedup, a)
	}
	return dedup
}

func itemsIguales(a, b CreateInvoiceItemRequest) bool {
	if a.ProductID != b.ProductID || a.SKU != b.SKU || a.Code != b.Code || a.Description != b.Description {
		return false
	}
	if a.Quantity != b.Quantity || a.UnitPrice != b.UnitPrice || a.Discount != b.Discount {
		return false
	}
	if (a.CodigoActividad == nil) != (b.CodigoActividad == nil) {
		return false
	}
	if a.CodigoActividad != nil && b.CodigoActividad != nil && *a.CodigoActividad != *b.CodigoActividad {
		return false
	}
	if (a.CodigoProductoSin == nil) != (b.CodigoProductoSin == nil) {
		return false
	}
	if a.CodigoProductoSin != nil && b.CodigoProductoSin != nil && *a.CodigoProductoSin != *b.CodigoProductoSin {
		return false
	}
	if (a.UnitCode == nil) != (b.UnitCode == nil) {
		return false
	}
	if a.UnitCode != nil && b.UnitCode != nil && *a.UnitCode != *b.UnitCode {
		return false
	}
	// SectorData: comparar raw bytes (nil == empty)
	ad := string(a.SectorData)
	bd := string(b.SectorData)
	if ad == "" {
		ad = "null"
	}
	if bd == "" {
		bd = "null"
	}
	return ad == bd
}
