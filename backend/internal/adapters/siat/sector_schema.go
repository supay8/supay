package siat

import (
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
)

// These setters are populated by Supay, never by an arbitrary data key. The
// remaining SDK setters form the sector-specific public schema. Inspecting the
// linked SDK (once at startup) keeps metadata, validation and XML in agreement.
var cabeceraAutomatica = strings.Fields(`NitEmisor RazonSocialEmisor Municipio Telefono
	NumeroFactura NumeroNotaCreditoDebito NumeroNotaConciliacion Cuf Cufd
	CodigoSucursal Direccion CodigoPuntoVenta FechaEmision NombreRazonSocial
	CodigoTipoDocumentoIdentidad NumeroDocumento Complemento CodigoCliente
	CodigoMetodoPago MontoTotal CodigoMoneda TipoCambio MontoTotalMoneda
	Leyenda Usuario CodigoDocumentoSector`)

var detalleAutomatico = strings.Fields(`NroItem ActividadEconomica CodigoProductoSin
	CodigoProducto Descripcion Cantidad UnidadMedida PrecioUnitario MontoDescuento
	SubTotal CodigoDetalleTransaccion`)

func completarEsquemaSDK(p *SectorProfile) {
	if !p.HasBuilder() {
		return
	}
	p.Campos = camposDelBuilder(p.builders.cabecera(), p.Campos, cabeceraAutomatica)
	if p.builders.detalle != nil {
		p.CamposDetalle = camposDelBuilder(p.builders.detalle(), p.CamposDetalle, detalleAutomatico)
	}
	p.Soportado = true // Technical coverage; does not assert SIAT homologation.
}

func camposDelBuilder(builder any, declarados []CampoSector, automaticos []string) []CampoSector {
	campos := append([]CampoSector(nil), declarados...)
	covered := make(map[string]bool)
	for _, name := range automaticos {
		covered["With"+name] = true
	}
	for i, field := range campos {
		if field.Metodo != "" && !tieneMetodo(builder, field.Metodo) {
			panic(fmt.Sprintf("esquema SIAT: %T no expone %s", builder, field.Metodo))
		}
		covered[field.Metodo] = true
		if field.Metodo != "" {
			campos[i].sdkType = reflect.ValueOf(builder).MethodByName(field.Metodo).Type().In(0)
		}
	}
	typ := reflect.TypeOf(builder)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !strings.HasPrefix(method.Name, "With") || covered[method.Name] {
			continue
		}
		// Decimal setters are precision overloads of the same XML field.
		if strings.HasSuffix(method.Name, "Decimal") && tieneMetodo(builder, strings.TrimSuffix(method.Name, "Decimal")) {
			continue
		}
		if method.Type.NumIn() != 2 || method.Type.IsVariadic() {
			panic(fmt.Sprintf("esquema SIAT: firma no cubierta %T.%s", builder, method.Name))
		}
		arg := method.Type.In(1)
		required := arg.Kind() != reflect.Pointer && arg.Kind() != reflect.Map && arg.Kind() != reflect.Slice && arg.Kind() != reflect.Interface
		for arg.Kind() == reflect.Pointer {
			arg = arg.Elem()
		}
		kind := ""
		switch arg.Kind() {
		case reflect.String:
			kind = "string"
		case reflect.Int, reflect.Int32, reflect.Int64:
			kind = "int"
		case reflect.Float32, reflect.Float64:
			kind = "float"
		case reflect.Bool:
			kind = "bool"
		case reflect.Map, reflect.Slice, reflect.Interface:
			kind = "json"
		case reflect.Struct:
			if arg == reflect.TypeOf(time.Time{}) {
				kind = "fecha"
			}
		}
		if kind == "" {
			panic(fmt.Sprintf("esquema SIAT: tipo no cubierto %T.%s (%s)", builder, method.Name, arg))
		}
		// IVA defaults to the line total, but specialized sectors may supply
		// their taxable base. The SDK's non-pointer setter isn't a requirement
		// to repeat a value which Supay already calculates.
		if method.Name == "WithMontoTotalSujetoIva" {
			required = false
		}
		field := campo(snakeSDK(strings.TrimPrefix(method.Name, "With")), method.Name, kind, required)
		field.sdkType = method.Type.In(1)
		campos = append(campos, field)
	}
	return campos
}

func snakeSDK(name string) string {
	runes := []rune(name)
	var out strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			out.WriteByte('_')
		}
		out.WriteRune(unicode.ToLower(r))
	}
	return out.String()
}
