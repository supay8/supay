package usecase

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
)

const (
	sectorNotaCreditoDebitoDescuentos = 47
	sectorNotaCreditoDebitoIce        = 48
	tasaIVACreditoFiscal              = 0.13
)

// autofillDocumentoAjusteDescuento completa ítems y datos_sector de una nota
// sector 24/29/47/48 a partir de la factura referenciada.
// Devuelve la factura original cargada para reutilizar validaciones posteriores.
// Las líneas repetidas son operaciones independientes y no se deduplican.
func (uc *InvoiceUsecase) autofillDocumentoAjusteDescuento(req *CreateInvoiceRequest, companyID string) (*domain.Invoice, error) {
	if req == nil || req.ReferenciaFacturaId == nil || strings.TrimSpace(*req.ReferenciaFacturaId) == "" {
		return nil, nil
	}
	if !esSectorAjuste(req.CodigoDocumentoSector) {
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
		profile, profileErr := siat.PerfilSectorLayout(req.CodigoDocumentoSector, req.Layout)
		if profileErr != nil {
			return nil, domain.NewBadRequestError(profileErr.Error())
		}
		for i := range req.Items {
			data, err := datosDetalleOriginal(profile, req.Items[i].SectorData)
			if err != nil {
				return nil, err
			}
			req.Items[i].SectorData = data
		}
	} else if err := hydrateAdjustmentItems(req.Items, ref.Items); err != nil {
		return nil, err
	}

	datos, err := buildDatosSectorNotaDescuento(ref, req.DatosSector, req.Items)
	if err != nil {
		return nil, err
	}
	if req.CodigoDocumentoSector == 29 {
		var values map[string]any
		if err := json.Unmarshal(datos, &values); err != nil {
			return nil, err
		}
		if _, ok := values["monto_total_conciliado"]; !ok {
			values["monto_total_conciliado"] = sumItemSubtotals(req.Items)
		}
		delete(values, "monto_total_devuelto")
		delete(values, "monto_efectivo_credito_debito")
		datos, err = json.Marshal(values)
		if err != nil {
			return nil, err
		}
	}
	req.DatosSector = datos

	return ref, nil
}

func esSectorAjuste(sector int) bool {
	return sector == 24 || sector == 29 || sector == 47 || sector == 48
}

// Keep only fields supported by the adjustment layout. Original sales may
// contain unrelated sector data (student, hotel, etc.). ICE fields are retained.
func datosDetalleOriginal(profile *siat.SectorProfile, data json.RawMessage) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, domain.NewConflictError("datos de detalle inválidos en factura original")
		}
	}
	out := map[string]json.RawMessage{}
	for _, field := range profile.CamposDetalle {
		if value, ok := values[field.JSON]; ok {
			out[field.JSON] = value
		}
	}
	return json.Marshal(out)
}

func sameFiscalLine(a, b *domain.InvoiceItem) bool {
	return cadenaOpcional(a.CodigoActividad) == cadenaOpcional(b.CodigoActividad) && cadenaOpcional(a.CodigoProductoSin) == cadenaOpcional(b.CodigoProductoSin) && ((a.UnitCode == nil && b.UnitCode == nil) || (a.UnitCode != nil && b.UnitCode != nil && *a.UnitCode == *b.UnitCode))
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

	expectedCUF := strings.TrimSpace(*ref.Cuf)
	if value, ok := valores["numero_autorizacion_cuf"]; ok && value != expectedCUF {
		return nil, domain.NewBadRequestError("numero_autorizacion_cuf no coincide con la factura referenciada")
	}
	valores["numero_autorizacion_cuf"] = expectedCUF
	expectedDate := ref.IssueDate.In(siat.LaPaz).Format("2006-01-02")
	if value, ok := valores["fecha_emision_factura"]; ok {
		text, isString := value.(string)
		matches := false
		if isString {
			for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
				date, err := time.ParseInLocation(layout, text, siat.LaPaz)
				if err == nil && date.In(siat.LaPaz).Format("2006-01-02") == expectedDate {
					matches = true
					break
				}
			}
		}
		if !matches {
			return nil, domain.NewBadRequestError("fecha_emision_factura no coincide con la factura referenciada")
		}
	}
	valores["fecha_emision_factura"] = expectedDate

	montoOriginal := ref.Total
	if v, ok := valores["monto_total_original"]; ok {
		amount, numeric := v.(float64)
		if !numeric || round2(amount) != round2(montoOriginal) {
			return nil, domain.NewBadRequestError("monto_total_original no coincide con la factura referenciada")
		}
	}
	valores["monto_total_original"] = round2(montoOriginal)

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

// hydrateAdjustmentItems copia la identidad fiscal desde la línea original.
// El request solo selecciona la línea por código y aporta cantidades/importes.
func hydrateAdjustmentItems(items []CreateInvoiceItemRequest, originals []domain.InvoiceItem) error {
	for index := range items {
		code := strings.TrimSpace(items[index].Code)
		if code == "" {
			code = strings.TrimSpace(items[index].SKU)
		}
		var match *domain.InvoiceItem
		for originalIndex := range originals {
			candidate := &originals[originalIndex]
			if candidate.Code != code {
				continue
			}
			if match != nil && !sameFiscalLine(match, candidate) {
				return domain.NewConflictError(fmt.Sprintf("el ítem %d coincide con líneas originales de distinta configuración fiscal", index+1))
			}
			match = candidate
		}
		if match == nil {
			return domain.NewBadRequestError(fmt.Sprintf("el producto del ítem %d no pertenece a la factura referenciada", index+1))
		}
		items[index].Code = match.Code
		items[index].SKU = match.Code
		items[index].Description = match.Description
		items[index].CodigoActividad = match.CodigoActividad
		items[index].CodigoProductoSin = match.CodigoProductoSin
		items[index].UnitCode = match.UnitCode
	}
	return nil
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
