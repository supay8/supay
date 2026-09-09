package siat

import (
	"encoding/json"
	"math"
)

// genericSectorAdapter mantiene compatibles los perfiles que todavía se
// construyen con el aplicador reflexivo. Su contrato permite migrarlos sin
// cambiar EmitirFactura ni los handlers HTTP.
type genericSectorAdapter struct{}

func (genericSectorAdapter) Prepare(p *SectorProfile, req SolicitudFactura) (SectorDocument, error) {
	datos := fusionarCamposLegados(p, req)
	values, err := p.ValidarDatosSector(datos)
	if err != nil {
		return SectorDocument{}, err
	}
	if p.EsAjuste() && p.Codigo != 29 {
		if _, ok := values["monto_descuento_credito_debito"]; !ok {
			values["monto_descuento_credito_debito"] = float64(0)
		}
	}
	return SectorDocument{
		Request: req,
		Values:  values,
		Present: presentSectorFields(datos),
		Null:    nullSectorFields(datos),
	}, nil
}

func (genericSectorAdapter) Build(p *SectorProfile, doc SectorDocument, cuf string) any {
	return construirFactura(p, doc.Request, cuf, doc.Values)
}

// CompraVentaPayload es el modelo tipado de la factura de compra-venta. Los
// campos comunes se copian aquí para que las reglas monetarias del sector no
// dependan de map[string]any ni del JSON recibido por HTTP.
type CompraVentaPayload struct {
	Cliente      ClienteFactura
	Items        []ItemFactura
	MontoTotal   float64
	CodigoMoneda int
	TipoCambio   float64
}

type compraVentaAdapter struct{}

func (compraVentaAdapter) Prepare(p *SectorProfile, req SolicitudFactura) (SectorDocument, error) {
	values, err := p.ValidarDatosSector(fusionarCamposLegados(p, req))
	if err != nil {
		return SectorDocument{}, err
	}
	normalized := normalizeCompraVenta(req)
	if len(req.Items) > 0 {
		normalized.MontoTotal, err = TotalDocumento(normalized.MontoTotal, req.DatosSector)
		if err != nil {
			return SectorDocument{}, err
		}
	}
	payload := CompraVentaPayload{
		Cliente:      normalized.Cliente,
		Items:        normalized.Items,
		MontoTotal:   normalized.MontoTotal,
		CodigoMoneda: normalized.CodigoMoneda,
		TipoCambio:   normalized.TipoCambio,
	}
	return SectorDocument{
		Request: normalized,
		Values:  values,
		Present: presentSectorFields(req.DatosSector),
		Null:    nullSectorFields(req.DatosSector),
		Payload: payload,
	}, nil
}

func (compraVentaAdapter) Build(p *SectorProfile, doc SectorDocument, cuf string) any {
	return construirFactura(p, doc.Request, cuf, doc.Values)
}

func normalizeCompraVenta(req SolicitudFactura) SolicitudFactura {
	req.TipoCambio = roundMoney(req.TipoCambio)
	req.Items = append([]ItemFactura(nil), req.Items...)
	for i := range req.Items {
		req.Items[i].Cantidad, req.Items[i].PrecioUnitario, _ = NormalizarImportesItem(SectorCompraVenta, req.Items[i].Cantidad, req.Items[i].PrecioUnitario, 0)
		if req.Items[i].MontoDescuento != nil {
			value := roundMoney(*req.Items[i].MontoDescuento)
			req.Items[i].MontoDescuento = &value
		}
		// Subtotal estricto dinámico: quantity*unitPrice - discount (corrige 100.00 quemado)
		req.Items[i].SubTotal = CalcularSubtotal(req.Items[i].Cantidad, req.Items[i].PrecioUnitario, req.Items[i].MontoDescuento)
	}
	req.MontoTotal, _ = CalcularTotales(req.Items, false)
	return req
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func presentSectorFields(datos []byte) map[string]bool {
	values := parseSectorObject(datos)
	present := make(map[string]bool, len(values))
	for key := range values {
		present[key] = true
	}
	return present
}

func nullSectorFields(datos []byte) map[string]bool {
	values := parseSectorObject(datos)
	null := make(map[string]bool)
	for key, value := range values {
		if value == nil {
			null[key] = true
		}
	}
	return null
}

func parseSectorObject(datos []byte) map[string]any {
	values := map[string]any{}
	if len(datos) == 0 {
		return values
	}
	// ValidarDatosSector es la autoridad para errores de JSON. Este helper solo
	// aporta metadata de presencia y por eso ignora un JSON inválido.
	_ = json.Unmarshal(datos, &values)
	return values
}
