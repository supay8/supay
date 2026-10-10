package siat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

//go:generate go run ./internal/buildergen

// SDK builder types are private. Typed closures infer their types at compile
// time and expose only this internal implementation detail to the serializer.
type typedSDKBuilder struct {
	setters map[string]func(any)
	build   func() any
}

func invocarCtor(ctor func() *typedSDKBuilder) any { return ctor() }
func tieneMetodo(builder any, method string) bool {
	_, ok := builder.(*typedSDKBuilder).setters[method]
	return ok
}
func llamarBuild(builder any) any { return builder.(*typedSDKBuilder).build() }
func llamarMetodo(builder any, method string, optional bool, args ...any) {
	if method == "" {
		return
	}
	setter, ok := builder.(*typedSDKBuilder).setters[method]
	if !ok {
		if optional {
			return
		}
		panic(fmt.Sprintf("campo fiscal no soportado: %s", method))
	}
	if len(args) != 1 {
		panic("SDK setter requires one argument")
	}
	setter(args[0])
}
func setBuilderValue[T, B any](setter func(T) B, value any) { setter(builderArg[T](value)) }
func setBuilderSlice[T, B any](setter func([]T) B, value any) {
	if item, ok := value.(T); ok {
		setter([]T{item})
		return
	}
	if item, ok := value.(*T); ok && item != nil {
		setter([]T{*item})
		return
	}
	setter(builderSliceArg[[]T](value))
}
func setBuilderVariadic[T, B any](setter func(...T) B, value any) {
	if item, ok := value.(T); ok {
		setter(item)
		return
	}
	if item, ok := value.(*T); ok && item != nil {
		setter(*item)
		return
	}
	setter(builderSliceArg[[]T](value)...)
}
func builderArg[T any](value any) T {
	if typed, ok := value.(T); ok {
		return typed
	}
	if ptr, ok := value.(*T); ok && ptr != nil {
		return *ptr
	}
	var out T
	data, err := json.Marshal(value)
	if text, ok := value.(string); ok {
		switch any(out).(type) {
		case int64, *int64, int, *int:
			if _, e := strconv.ParseInt(text, 10, 64); e == nil {
				data = []byte(text)
			}
		}
	}
	switch any(out).(type) {
	case string, *string:
		switch n := value.(type) {
		case int64:
			data, _ = json.Marshal(strconv.FormatInt(n, 10))
		case float64:
			data, _ = json.Marshal(strconv.FormatFloat(n, 'f', -1, 64))
		}
	}
	if err == nil {
		err = json.Unmarshal(data, &out)
	}
	if err != nil {
		panic(fmt.Errorf("valor %T incompatible con el campo fiscal: %w", value, err))
	}
	return out
}
func builderSliceArg[T any](value any) T {
	if typed, ok := value.(T); ok {
		return typed
	}
	var out T
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &out); err == nil {
		return out
	}
	if err = json.Unmarshal(append(append([]byte{'['}, data...), ']'), &out); err != nil {
		panic(err)
	}
	return out
}
func aEntero(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case float32:
		return int64(n), true
	}
	return 0, false
}
func aFlotante(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
func esPunteroNil(v any) bool {
	switch p := v.(type) {
	case nil:
		return true
	case *string:
		return p == nil
	case *int:
		return p == nil
	case *int64:
		return p == nil
	case *float64:
		return p == nil
	case *time.Time:
		return p == nil
	}
	return false
}

// construirCabecera crea el builder de cabecera del sector, aplica los campos
// comunes (omitiendo los que ese XSD no tiene), los datos_sector específicos y
// devuelve la cabecera construida lista para WithCabecera.
func construirCabecera(p *SectorProfile, req SolicitudFactura, cuf string, valores map[string]any) any {
	// Fix 920: telefono y complemento deben estar presentes en la secuencia XSD.
	// Si vienen nil (empresa sin teléfono, cliente sin complemento) el SDK
	// emite xsi:nil que el SIAT rechaza al quitarlo con regex; en su lugar
	// enviamos valores neutros para que serialice <telefono>0000000</telefono>
	// y <complemento></complemento>.
	if req.Telefono == nil {
		def := "0000000"
		req.Telefono = &def
	}
	if req.Cliente.Complemento == nil {
		def := ""
		req.Cliente.Complemento = &def
	}
	if req.Cliente.CodigoCliente == nil {
		def := ""
		req.Cliente.CodigoCliente = &def
	}
	cab := p.builders.cabecera()

	comunes := []struct {
		metodo string
		valor  any
	}{
		{"WithNitEmisor", parseNit(req.Nit)},
		{"WithRazonSocialEmisor", req.RazonSocialEmisor},
		{"WithMunicipio", req.Municipio},
		{"WithTelefono", req.Telefono},
		{"WithCuf", cuf},
		{"WithCufd", req.Cufd},
		{"WithCafc", req.Cafc},
		{"WithCodigoSucursal", req.CodigoSucursal},
		{"WithDireccion", req.Direccion},
		{"WithCodigoPuntoVenta", req.CodigoPuntoVenta},
		{"WithFechaEmision", req.FechaEmision},
		{"WithNombreRazonSocial", req.Cliente.NombreRazonSocial},
		{"WithCodigoTipoDocumentoIdentidad", req.Cliente.CodigoTipoDocumentoIdentidad},
		{"WithNumeroDocumento", req.Cliente.NumeroDocumento},
		{"WithComplemento", req.Cliente.Complemento},
		{"WithCodigoCliente", req.Cliente.CodigoCliente},
		{"WithNumeroTarjeta", zeroInt64},
		{"WithMontoGiftCard", zeroFloat},
		{"WithDescuentoAdicional", zeroFloat},
		{"WithCodigoExcepcion", zeroInt},
		{"WithCodigoMetodoPago", req.CodigoMetodoPago},
		{"WithMontoTotal", req.MontoTotal},
		{"WithMontoTotalSujetoIva", montoTotalSujetoIva(p, req)},
		{"WithCodigoMoneda", req.CodigoMoneda},
		{"WithTipoCambio", req.TipoCambio},
		{"WithMontoTotalMoneda", montoTotalMoneda(req)},
		{"WithLeyenda", req.Leyenda},
		{"WithUsuario", req.Usuario},
		{"WithCodigoDocumentoSector", p.Codigo},
	}
	for _, c := range comunes {
		if esPunteroNil(c.valor) || !tieneMetodo(cab, c.metodo) {
			// Campos opcionales vacíos y campos que el XSD del sector no define
			// (montoTotal en notas, municipio en boletos) se omiten: el
			// constructor ya dejó esos punteros en nil / el nodo fuera del XML.
			continue
		}
		llamarMetodo(cab, c.metodo, false, c.valor)
	}

	// Los documentos de ajuste del SDK tienen dos correlativos: el número de
	// la nota y el número de la factura original. Ambos son obligatorios para
	// el XSD del sector 24; no debe elegirse uno descartando el otro.
	numeroFacturaOriginal := req.NumeroFacturaOriginal
	if numeroFacturaOriginal <= 0 {
		// Compatibilidad para consumidores de bajo nivel que todavía solo
		// proporcionan NumeroFactura.
		numeroFacturaOriginal = req.NumeroFactura
	}
	if tieneMetodo(cab, "WithNumeroFactura") {
		llamarMetodo(cab, "WithNumeroFactura", false, numeroFacturaOriginal)
	}
	if tieneMetodo(cab, "WithNumeroNotaCreditoDebito") {
		llamarMetodo(cab, "WithNumeroNotaCreditoDebito", false, req.NumeroFactura)
	}
	if tieneMetodo(cab, "WithNumeroNotaConciliacion") {
		llamarMetodo(cab, "WithNumeroNotaConciliacion", false, req.NumeroFactura)
	}

	for _, campo := range p.Campos {
		if campo.Metodo == "" {
			continue
		}
		valor, presente := valores[campo.JSON]
		if !presente || esPunteroNil(valor) {
			continue
		}
		if campo.Metodo == "WithNumeroNotaCreditoDebito" || campo.Metodo == "WithNumeroNotaConciliacion" {
			if number, ok := aEntero(valor); !ok || number != req.NumeroFactura {
				panic("el número de nota lo asigna Supay y debe coincidir con el correlativo del CUF")
			}
		}
		llamarMetodo(cab, campo.Metodo, false, valor)
	}

	return llamarBuild(cab)
}

func montoTotalSujetoIva(p *SectorProfile, req SolicitudFactura) float64 {
	if p.MontoSujetoIvaCero {
		return 0
	}
	// The total already includes the header discount. Specialized bases can
	// override this default through the sector's MontoTotalSujetoIva setter.
	values := parseSectorObject(req.DatosSector)
	gift, _ := toFloat(values["monto_gift_card"])
	return round2(req.MontoTotal - gift)
}

func montoTotalMoneda(req SolicitudFactura) float64 {
	if req.TipoCambio <= 0 {
		return req.MontoTotal
	}
	return round2(req.MontoTotal / req.TipoCambio)
}

// construirDetalles construye las líneas de detalle del sector. Para sectores
// prevalorados (detalle único) usa WithDetalle con el primer ítem; para el resto
// AddDetalle con cada ítem. Si DetallePar es true (sectores 47/48), cada ítem
// lógico genera dos nodos <detalle> con el mismo contenido y
// codigoDetalleTransaccion 1 (original) y 2 (devolución/ajuste), con nroItem
// secuencial 1,2,3,4...
func construirDetalles(p *SectorProfile, root any, items []ItemFactura) {
	if !p.ConDetalle || len(items) == 0 {
		return
	}
	if p.DetalleUnico {
		detalle := construirDetalle(p, items[0], 1)
		llamarMetodo(root, "WithDetalle", false, detalle)
		return
	}
	if p.DetallePar {
		nro := 1
		for i := range items {
			d1 := construirDetalleConCodigo(p, items[i], nro, 1)
			llamarMetodo(root, "AddDetalle", false, d1)
			nro++
			d2 := construirDetalleConCodigo(p, items[i], nro, 2)
			llamarMetodo(root, "AddDetalle", false, d2)
			nro++
		}
		return
	}
	for i := range items {
		detalle := construirDetalle(p, items[i], i+1)
		llamarMetodo(root, "AddDetalle", false, detalle)
	}
}

func construirDetalle(p *SectorProfile, item ItemFactura, correlativo int) any {
	return construirDetalleConCodigo(p, item, correlativo, correlativo)
}

func construirDetalleConCodigo(p *SectorProfile, item ItemFactura, nroItem int, codigoTransaccion int) any {
	if p.builders.detalle == nil {
		panic(fmt.Sprintf("el sector %d no admite líneas de detalle", p.Codigo))
	}
	det := p.builders.detalle()
	comunes := []struct {
		metodo string
		valor  any
	}{
		{"WithNroItem", nroItem},
		{"WithActividadEconomica", item.ActividadEconomica},
		{"WithCodigoProductoSin", item.CodigoProductoSin},
		{"WithCodigoProducto", item.CodigoProducto},
		{"WithDescripcion", item.Descripcion},
		{"WithCantidad", item.Cantidad},
		{"WithUnidadMedida", item.UnidadMedida},
		{"WithPrecioUnitario", item.PrecioUnitario},
		{"WithMontoDescuento", descuentoPtr(item.MontoDescuento)},
		{"WithSubTotal", item.SubTotal},
	}
	for _, c := range comunes {
		llamarMetodo(det, c.metodo, true, c.valor)
	}
	// codigoDetalleTransaccion existe solo en los detalles de notas. Para
	// sectores con DetallePar (47/48) es 1 para original y 2 para devolución;
	// para el resto (24, etc.) es el correlativo secuencial.
	llamarMetodo(det, "WithCodigoDetalleTransaccion", true, codigoTransaccion)
	// Campos sectoriales de detalle: se aplican mediante llamadas tipadas NO tolerantes.
	// Si Supay declara un campo en CamposDetalle pero el builder no tiene el
	// método With* correspondiente, llamarMetodo con tolerante=false produce un
	// error explícito (panic que buildFacturaSDK convierte en error).
	aplicarCamposDetalle(p, det, item.DatosSector)
	return llamarBuild(det)
}

// aplicarCamposDetalle valida y aplica los datos sectoriales del item sobre el
// builder de detalle usando las mismas llamadas tipadas que la cabecera. Un
// item sin DatosSector conserva el comportamiento anterior solo si el perfil no
// declara CamposDetalle requeridos. Si el builder no expone el método With*
// declarado, se produce un error explícito.
func aplicarCamposDetalle(p *SectorProfile, det any, datos json.RawMessage) {
	if len(p.CamposDetalle) == 0 && (len(datos) == 0 || string(datos) == "null") {
		return
	}
	valores, err := p.ValidarDatosDetalle(datos)
	if err != nil {
		panic(err)
	}
	for _, campo := range p.CamposDetalle {
		if campo.Metodo == "" {
			continue
		}
		valor, presente := valores[campo.JSON]
		if !presente || esPunteroNil(valor) {
			continue
		}
		llamarMetodo(det, campo.Metodo, false, valor)
	}
}

// construirFactura arma el documento raíz del sector: modalidad + cabecera +
// detalles. Devuelve el struct listo para serializar/firmar.
func construirFactura(p *SectorProfile, req SolicitudFactura, cuf string, valores map[string]any) any {
	root := p.builders.factura(req.Modalidad)
	cabecera := construirCabecera(p, req, cuf, valores)
	llamarMetodo(root, "WithCabecera", false, cabecera)
	switch {
	case p.Codigo == 29:
		originals := req.OriginalItems
		if len(originals) == 0 {
			originals = req.Items
		}
		originalProfile := *p
		originalCore := *p.SectorProfile
		originalProfile.SectorProfile = &originalCore
		originalProfile.CamposDetalle = nil
		originalProfile.builders.detalle = func() any { return newNotaDetalleOriginalBuilder() }
		for i, item := range originals {
			item.DatosSector = nil
			llamarMetodo(root, "AddDetalleOriginal", false, construirDetalle(&originalProfile, item, i+1))
		}
		for i, item := range req.Items {
			llamarMetodo(root, "AddDetalleConciliacion", false, construirDetalle(p, item, i+1))
		}
	case (p.DetallePar || p.Codigo == SectorNotaCreditoDebito) && len(req.OriginalItems) > 0:
		// Original sale and returned lines can have different amounts/counts.
		// Preserve both groups instead of copying the returned amount twice.
		nro := 1
		for _, item := range req.OriginalItems {
			llamarMetodo(root, "AddDetalle", false, construirDetalleConCodigo(p, item, nro, 1))
			nro++
		}
		for _, item := range req.Items {
			llamarMetodo(root, "AddDetalle", false, construirDetalleConCodigo(p, item, nro, 2))
			nro++
		}
	default:
		construirDetalles(p, root, req.Items)
	}
	return llamarBuild(root)
}
