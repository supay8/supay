package siat

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// Este archivo implementa el aplicador genérico de builders del SDK go-siat.
// Los ~50 builders de factura no comparten una interfaz común (cada uno define
// sus métodos With* con tipos que varían: *int vs *int64, etc.), por lo que la
// construcción se hace por reflexión convirtiendo cada argumento al tipo exacto
// que declara el método. Un campo cuyo método no existe en ese builder se omite
// (los XSD de notas y boletos no tienen montoTotal, moneda, etc.).

// llamarMetodo invoca metodo sobre builder con args convertidos al tipo exacto
// de los parámetros declarados. tolerante=true omite en silencio cuando el
// método no existe en el builder (campo sin equivalente en ese XSD); con
// tolerante=false devuelve error.
func llamarMetodo(builder any, metodo string, tolerante bool, args ...any) {
	if metodo == "" {
		return
	}
	m := reflect.ValueOf(builder).MethodByName(metodo)
	if !m.IsValid() {
		if tolerante {
			return
		}
		panic(fmt.Sprintf("el builder %T no expone el método %s", builder, metodo))
	}
	mt := m.Type()
	nFijos := mt.NumIn()
	if mt.IsVariadic() {
		nFijos--
	}
	if (!mt.IsVariadic() && mt.NumIn() != len(args)) || (mt.IsVariadic() && len(args) < nFijos) {
		panic(fmt.Sprintf("%T.%s espera %d argumentos, se pasaron %d", builder, metodo, mt.NumIn(), len(args)))
	}
	convertidos := make([]reflect.Value, len(args))
	for i, arg := range args {
		param := mt.In(i)
		if mt.IsVariadic() && i >= nFijos {
			// Los argumentos variádicos se convierten al tipo del elemento y
			// viajan individuales (reflect.Call no acepta el slice).
			param = param.Elem()
		}
		cv, ok := convertirArg(param, arg)
		if !ok {
			panic(fmt.Sprintf("%T.%s: no se pudo convertir %T (%v) al parámetro %s",
				builder, metodo, arg, arg, param))
		}
		convertidos[i] = cv
	}
	m.Call(convertidos)
}

// convertirArg adapta un valor (string/int/int64/float64/time.Time o punteros a
// ellos) al tipo exacto del parámetro del builder: string, int, int64, float64,
// time.Time o punteros a cualquiera de ellos. Devuelve ok=false para omitir el
// campo (valor nil o incompatible).
func convertirArg(param reflect.Type, valor any) (reflect.Value, bool) {
	if valor == nil {
		return reflect.Value{}, false
	}
	v := reflect.ValueOf(valor)
	if v.Type() == param {
		return v, true
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, false
		}
		valor = v.Elem().Interface()
	}
	if param.Kind() == reflect.Pointer {
		elem, ok := convertirArg(param.Elem(), valor)
		if !ok {
			return reflect.Value{}, false
		}
		p := reflect.New(param.Elem())
		p.Elem().Set(elem)
		return p, true
	}
	// Algunos builders (p.ej. CompraVentaTasas) reciben los detalles como
	// slice: se envuelve el detalle individual en un slice de un elemento.
	if param.Kind() == reflect.Slice {
		elem, ok := convertirArg(param.Elem(), valor)
		if !ok {
			return reflect.Value{}, false
		}
		s := reflect.MakeSlice(param, 1, 1)
		s.Index(0).Set(elem)
		return s, true
	}
	switch param.Kind() {
	case reflect.String:
		switch n := valor.(type) {
		case string:
			return reflect.ValueOf(n).Convert(param), true
		case int64:
			return reflect.ValueOf(strconv.FormatInt(n, 10)).Convert(param), true
		case float64:
			return reflect.ValueOf(strconv.FormatFloat(n, 'f', -1, 64)).Convert(param), true
		}
	case reflect.Int, reflect.Int32:
		n, ok := aEntero(valor)
		if !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(int(n)).Convert(param), true
	case reflect.Int64:
		if n, ok := aEntero(valor); ok {
			return reflect.ValueOf(n).Convert(param), true
		}
		if s, esStr := valor.(string); esStr {
			if parsed, err := strconv.ParseInt(s, 10, 64); err == nil {
				return reflect.ValueOf(parsed).Convert(param), true
			}
		}
		return reflect.Value{}, false
	case reflect.Float64, reflect.Float32:
		f, ok := aFlotante(valor)
		if !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(f).Convert(param), true
	case reflect.Struct:
		if param == reflect.TypeOf(time.Time{}) {
			if t, ok := valor.(time.Time); ok {
				return reflect.ValueOf(t), true
			}
		}
	case reflect.Bool:
		if b, ok := valor.(bool); ok {
			return reflect.ValueOf(b), true
		}
	}
	return reflect.Value{}, false
}

func aEntero(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
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

func tieneMetodo(v any, metodo string) bool {
	return reflect.ValueOf(v).MethodByName(metodo).IsValid()
}

// esPunteroNil reporta si el valor es nil o un puntero tipado a nil
// ((*string)(nil), etc.), caso en el que el campo debe omitirse.
func esPunteroNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// construirCabecera crea el builder de cabecera del sector, aplica los campos
// comunes (omitiendo los que ese XSD no tiene), los datos_sector específicos y
// devuelve la cabecera construida lista para WithCabecera.
func construirCabecera(p *SectorProfile, req SolicitudFactura, cuf string, valores map[string]any) any {
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
		{"WithCafc", req.Cafc},
		{"WithCodigoMetodoPago", req.CodigoMetodoPago},
		{"WithMontoTotal", req.MontoTotal},
		{"WithMontoTotalSujetoIva", montoTotalSujetoIva(p, req)},
		{"WithCodigoMoneda", req.CodigoMoneda},
		{"WithTipoCambio", req.TipoCambio},
		{"WithMontoTotalMoneda", req.MontoTotal},
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
		llamarMetodo(cab, campo.Metodo, false, valor)
	}

	return llamarBuild(cab)
}

func montoTotalSujetoIva(p *SectorProfile, req SolicitudFactura) float64 {
	if p.MontoSujetoIvaCero {
		return 0
	}
	return req.MontoTotal
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
			llamarMetodo(root, "AddDetalle", true, d1)
			nro++
			d2 := construirDetalleConCodigo(p, items[i], nro, 2)
			llamarMetodo(root, "AddDetalle", true, d2)
			nro++
		}
		return
	}
	for i := range items {
		detalle := construirDetalle(p, items[i], i+1)
		llamarMetodo(root, "AddDetalle", true, detalle)
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
	// Campos sectoriales de detalle: se aplican mediante reflexión NO tolerante.
	// Si Supay declara un campo en CamposDetalle pero el builder no tiene el
	// método With* correspondiente, llamarMetodo con tolerante=false produce un
	// error explícito (panic que buildFacturaSDK convierte en error).
	aplicarCamposDetalle(p, det, item.DatosSector)
	return llamarBuild(det)
}

// aplicarCamposDetalle valida y aplica los datos sectoriales del item sobre el
// builder de detalle usando la misma reflexión que los campos de cabecera. Un
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
	construirDetalles(p, root, req.Items)
	return llamarBuild(root)
}

// llamarBuild invoca Build() sobre cualquier builder del SDK.
func llamarBuild(builder any) any {
	v := reflect.ValueOf(builder).MethodByName("Build")
	if !v.IsValid() {
		panic(fmt.Sprintf("el builder %T no expone Build()", builder))
	}
	out := v.Call(nil)
	if len(out) == 0 {
		panic(fmt.Sprintf("Build() de %T no devolvió nada", builder))
	}
	return out[0].Interface()
}
