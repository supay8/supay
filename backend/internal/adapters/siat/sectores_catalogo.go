package siat

import (
	"reflect"

	"github.com/ron86i/go-siat/v2/pkg/models/invoices"
)

// catalogoSectores es el registro de todos los documentos-sector soportados,
// construido a partir del catálogo normativo del SIAT y de las fachadas del SDK
// go-siat v2.1.1. Los códigos 25, 26, 27 y 32 no existen en el catálogo vigente;
// el 33 (Tasa Cero IVA Ley N° 1613) existe pero el SDK no trae builder para él.
//
// Clasificación tipoFacturaDocumento según el catálogo de crédito fiscal del SIN:
// Con → 1 (con derecho), Sin → 2 (sin derecho / documento equivalente),
// Ajuste → 3 (notas de crédito/débito y conciliación).
var catalogoSectores = []*SectorProfile{
	sector(1, "Compra y Venta", TipoDocumentoFacturaConCredito, FachadaCompraVenta,
		b(invoices.NewCompraVentaBuilder, invoices.NewCompraVentaCabeceraBuilder, invoices.NewCompraVentaDetalleBuilder)),

	sector(2, "Alquiler de Bienes Inmuebles", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewAlquilerBienInmuebleBuilder, invoices.NewAlquilerBienInmuebleCabeceraBuilder, invoices.NewAlquilerBienInmuebleDetalleBuilder),
		campoE("periodo_facturado", "WithPeriodoFacturado", "string", true, "Período facturado", "2026-08")),

	sector(3, "Comercial de Exportación", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewComercialExportacionBuilder, invoices.NewComercialExportacionCabeceraBuilder, invoices.NewComercialExportacionDetalleBuilder)),

	sector(4, "Libre Consignación", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewLibreConsignacionBuilder, invoices.NewLibreConsignacionCabeceraBuilder, invoices.NewLibreConsignacionDetalleBuilder)),

	sector(5, "Venta en Zona Franca", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewZonaFrancaBuilder, invoices.NewZonaFrancaCabeceraBuilder, invoices.NewZonaFrancaDetalleBuilder)),

	sector(6, "Servicio Turístico y Hospedaje", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewTurismoHospedajeBuilder, invoices.NewTurismoHospedajeCabeceraBuilder, invoices.NewTurismoHospedajeDetalleBuilder),
		campo("fecha_ingreso_hospedaje", "WithFechaIngresoHospedaje", "fecha", false),
		campo("cantidad_habitaciones", "WithCantidadHabitaciones", "int", false),
		campo("cantidad_huespedes", "WithCantidadHuespedes", "int", false),
		campo("cantidad_mayores", "WithCantidadMayores", "int", false),
		campo("cantidad_menores", "WithCantidadMenores", "int", false),
		campo("razon_social_operador_turismo", "WithRazonSocialOperadorTurismo", "string", false)),

	sector(7, "Seguridad Alimentaria y Abastecimiento", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewSeguridadAlimentariaBuilder, invoices.NewSeguridadAlimentariaCabeceraBuilder, invoices.NewSeguridadAlimentariaDetalleBuilder)),

	func() *SectorProfile {
		p := sector(8, "Tasa Cero (libros y transporte internacional de carga)", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
			b(invoices.NewTasaCeroBuilder, invoices.NewTasaCeroCabeceraBuilder, invoices.NewTasaCeroDetalleBuilder))
		p.MontoSujetoIvaCero = true
		return p
	}(),

	sector(9, "Compra y Venta de Moneda Extranjera", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewMonedaExtranjeraBuilder, invoices.NewMonedaExtranjeraCabeceraBuilder, invoices.NewMonedaExtranjeraDetalleBuilder),
		campo("codigo_tipo_operacion", "WithCodigoTipoOperacion", "int", false),
		campo("tipo_cambio_oficial", "WithTipoCambioOficial", "float", false),
		campo("ingreso_diferencia_cambio", "WithIngresoDiferenciaCambio", "float", false)),

	sector(10, "Dutty Free", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewDuttyFreeBuilder, invoices.NewDuttyFreeCabeceraBuilder, invoices.NewDuttyFreeDetalleBuilder)),

	sectorEducativo(11, "Sectores Educativos",
		b(invoices.NewSectorEducativoBuilder, invoices.NewSectorEducativoCabeceraBuilder, invoices.NewSectorEducativoDetalleBuilder)),

	sector(12, "Comercialización de Hidrocarburos", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewComercializacionHidroBuilder, invoices.NewComercializacionHidroCabeceraBuilder, invoices.NewComercializacionHidroDetalleBuilder)),

	sector(13, "Servicios Básicos", TipoDocumentoFacturaConCredito, FachadaServicioBasico,
		b(invoices.NewServicioBasicoBuilder, invoices.NewServicioBasicoCabeceraBuilder, invoices.NewServicioBasicoDetalleBuilder),
		camposServicioBasico()...),

	sector(14, "Productos Alcanzados por el ICE", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewAlcanzadaIceBuilder, invoices.NewAlcanzadaIceCabeceraBuilder, invoices.NewAlcanzadaIceDetalleBuilder)),

	sector(15, "Entidades Financieras", TipoDocumentoFacturaConCredito, FachadaEntidadFinanciera,
		b(invoices.NewEntidadFinancieraBuilder, invoices.NewEntidadFinancieraCabeceraBuilder, invoices.NewEntidadFinancieraDetalleBuilder),
		campo("monto_total_arrendamiento_financiero", "WithMontoTotalArrendamientoFinanciero", "float", false)),

	sector(16, "Hoteles", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewHotelBuilder, invoices.NewHotelCabeceraBuilder, invoices.NewHotelDetalleBuilder),
		camposHospedaje()...),

	sector(17, "Hospitales / Clínicas", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewHospitalClinicaBuilder, invoices.NewHospitalClinicaCabeceraBuilder, invoices.NewHospitalClinicaDetalleBuilder),
		campo("modalidad_servicio", "WithModalidadServicio", "string", false)),

	sector(18, "Juegos de Azar", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewJuegoAzarBuilder, invoices.NewJuegoAzarCabeceraBuilder, invoices.NewJuegoAzarDetalleBuilder)),

	sector(19, "Hidrocarburos Alcanzada IEHD", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewHidrocarburoAlcanzadaIehdBuilder, invoices.NewHidrocarburoAlcanzadaIehdCabeceraBuilder, invoices.NewHidrocarburoAlcanzadaIehdDetalleBuilder)),

	sector(20, "Comercial de Exportación de Minerales", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewComercialExportacionMineraBuilder, invoices.NewComercialExportacionMineraCabeceraBuilder, invoices.NewComercialExportacionMineraDetalleBuilder)),

	sector(21, "Venta de Minerales", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewVentaMineralBuilder, invoices.NewVentaMineralCabeceraBuilder, invoices.NewVentaMineralDetalleBuilder)),

	sector(22, "Telecomunicaciones", TipoDocumentoFacturaConCredito, FachadaTelecomunicaciones,
		b(invoices.NewTelecomunicacionesBuilder, invoices.NewTelecomunicacionesCabeceraBuilder, invoices.NewTelecomunicacionesDetalleBuilder),
		campo("nit_conjunto", "WithNitConjunto", "int", false)),

	func() *SectorProfile {
		p := sector(23, "Prevalorada", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
			b(invoices.NewPrevaloradaBuilder, invoices.NewPrevaloradaCabeceraBuilder, invoices.NewPrevaloradaDetalleBuilder))
		p.DetalleUnico = true
		return p
	}(),

	nota(24, "Nota de Crédito-Débito",
		invoices.NewNotaCreditoDebitoBuilder, invoices.NewNotaCreditoDebitoCabeceraBuilder, invoices.NewNotaDetalleCreditoDebitoBuilder,
		notasCampos()...),

	notaLayout(24, "Nota Fiscal de Crédito-Débito", "nota_fiscal_credito_debito",
		invoices.NewNotaFiscalCreditoDebitoBuilder, invoices.NewNotaFiscalCreditoDebitoCabeceraBuilder, invoices.NewNotaDetalleFiscalCreditoDebitoBuilder,
		notasCampos()...),

	sector(28, "Comercial de Exportación de Servicios", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewComercialExportacionServicioBuilder, invoices.NewComercialExportacionServicioCabeceraBuilder, invoices.NewComercialExportacionServicioDetalleBuilder)),

	sectorAjuste(29, "Nota de Conciliación",
		invoices.NewNotaConciliacionBuilder, invoices.NewNotaConciliacionCabeceraBuilder, invoices.NewNotaDetalleConciliacionBuilder,
		campo("numero_autorizacion_cuf", "WithNumeroAutorizacionCuf", "string", true),
		campo("fecha_emision_factura", "WithFechaEmisionFactura", "fecha", true),
		campo("monto_total_original", "WithMontoTotalOriginal", "float", true),
		campo("monto_total_conciliado", "WithMontoTotalConciliado", "float", true),
		campo("numero_nota_conciliacion", "WithNumeroNotaConciliacion", "int", false),
		campo("credito_fiscal_iva", "WithCreditoFiscalIva", "float", false),
		campo("debito_fiscal_iva", "WithDebitoFiscalIva", "float", false)),

	sector(30, "Boleto Aéreo", TipoDocumentoFacturaSinCredito, FachadaBoletoAereo,
		b(invoices.NewBoletoAereoBuilder, invoices.NewBoletoAereoCabeceraBuilder, nil),
		campo("nombre_pasajero", "WithNombrePasajero", "string", true),
		campo("numero_documento_pasajero", "WithNumeroDocumentoPasajero", "string", true),
		campo("codigo_iata_linea_aerea", "WithCodigoIataLineaAerea", "int", false),
		campo("codigo_iata_agente_viajes", "WithCodigoIataAgenteViajes", "string", false),
		campo("nit_agente_viajes", "WithNitAgenteViajes", "int", false),
		campo("codigo_origen_servicio", "WithCodigoOrigenServicio", "string", false),
		campo("codigo_tipo_transaccion", "WithCodigoTipoTransaccion", "string", false),
		campo("monto_tarifa", "WithMontoTarifa", "float", false)),

	sector(31, "Suministro de Energía", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewSuministroEnergiaBuilder, invoices.NewSuministroEnergiaCabeceraBuilder, invoices.NewSuministroEnergiaDetalleBuilder)),

	sector(33, "Tasa Cero IVA Ley N° 1613", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		buildersSector{}),

	sector(34, "Seguros", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewSegurosBuilder, invoices.NewSegurosCabeceraBuilder, invoices.NewSegurosDetalleBuilder)),

	sector(35, "Compra Venta Bonificaciones", TipoDocumentoFacturaConCredito, FachadaCompraVenta,
		b(invoices.NewCompraVentaBonificacionesBuilder, invoices.NewCompraVentaBonificacionesCabeceraBuilder, invoices.NewCompraVentaBonificacionesDetalleBuilder)),

	func() *SectorProfile {
		p := sector(36, "Prevalorada Sin Derecho a Crédito Fiscal", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
			b(invoices.NewPrevaloradaSinDerechoCreditoFiscalBuilder, invoices.NewPrevaloradaSinDerechoCreditoFiscalCabeceraBuilder, invoices.NewPrevaloradaSinDerechoCreditoFiscalDetalleBuilder))
		p.DetalleUnico = true
		return p
	}(),

	sector(37, "Comercialización de GNV", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewComercializacionGnvBuilder, invoices.NewComercializacionGnvCabeceraBuilder, invoices.NewComercializacionGnvDetalleBuilder)),

	sector(38, "Hidrocarburos No Alcanzada IEHD", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewHidrocarburoNoAlcanzadaIehdBuilder, invoices.NewHidrocarburoNoAlcanzadaIehdCabeceraBuilder, invoices.NewHidrocarburoNoAlcanzadaIehdDetalleBuilder)),

	sector(39, "Comercialización de GN y GLP", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewComercializacionGnGlpBuilder, invoices.NewComercializacionGnGlpCabeceraBuilder, invoices.NewComercializacionGnGlpDetalleBuilder)),

	sector(40, "Servicios Básicos Zona Franca", TipoDocumentoFacturaSinCredito, FachadaServicioBasico,
		b(invoices.NewServicioBasicoZFBuilder, invoices.NewServicioBasicoZFCabeceraBuilder, invoices.NewServicioBasicoZFDetalleBuilder),
		camposServicioBasico()...),

	sector(41, "Compra Venta Tasas", TipoDocumentoFacturaConCredito, FachadaCompraVenta,
		b(invoices.NewCompraVentaTasasBuilder, invoices.NewCompraVentaTasasCabeceraBuilder, invoices.NewCompraVentaTasasDetalleBuilder)),

	sector(42, "Alquiler Zona Franca", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewAlquilerZFBuilder, invoices.NewAlquilerZFCabeceraBuilder, invoices.NewAlquilerZFDetalleBuilder),
		campoE("periodo_facturado", "WithPeriodoFacturado", "string", true, "Período facturado", "2026-08")),

	sector(43, "Comercial de Exportación Hidrocarburos", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewComercialExportacionHidroBuilder, invoices.NewComercialExportacionHidroCabeceraBuilder, invoices.NewComercialExportacionHidroDetalleBuilder)),

	sector(44, "Importación y Comercialización de Lubricantes", TipoDocumentoFacturaConCredito, FachadaPorModalidad,
		b(invoices.NewImportacionComercializacionLubricantesBuilder, invoices.NewImportacionComercializacionLubricantesCabeceraBuilder, invoices.NewImportacionComercializacionLubricantesDetalleBuilder)),

	sector(45, "Comercial de Exportación Precio Venta", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewComercialExportacionPVentaBuilder, invoices.NewComercialExportacionPVentaCabeceraBuilder, invoices.NewComercialExportacionPVentaDetalleBuilder)),

	sectorEducativoSinCredito(46, "Sector Educativo Zona Franca",
		b(invoices.NewSectorEducativoZFBuilder, invoices.NewSectorEducativoZFCabeceraBuilder, invoices.NewSectorEducativoZFDetalleBuilder)),

	func() *SectorProfile {
		p := nota(47, "Nota Crédito Débito Descuentos",
			invoices.NewNotaCreditoDebitoDescuentoBuilder, invoices.NewNotaCreditoDebitoDescuentoCabeceraBuilder, invoices.NewNotaDetalleCreditoDebitoDescuentoBuilder,
			notasCampos()...)
		p.DetallePar = true
		return p
	}(),

	func() *SectorProfile {
		p := nota(48, "Nota Crédito Débito ICE",
			invoices.NewNotaCreditoDebitoIceBuilder, invoices.NewNotaCreditoDebitoIceCabeceraBuilder, invoices.NewNotaDetalleCreditoDebitoIceBuilder,
			notasCampos()...)
		p.DetallePar = true
		return p
	}(),

	sector(49, "Telecomunicaciones Zona Franca", TipoDocumentoFacturaSinCredito, FachadaTelecomunicaciones,
		b(invoices.NewTelecomunicacionesZFBuilder, invoices.NewTelecomunicacionesZFCabeceraBuilder, invoices.NewTelecomunicacionesZFDetalleBuilder),
		campo("nit_conjunto", "WithNitConjunto", "int", false)),

	sector(50, "Hospitales / Clínicas Zona Franca", TipoDocumentoFacturaSinCredito, FachadaPorModalidad,
		b(invoices.NewHospitalClinicaZFBuilder, invoices.NewHospitalClinicaZFCabeceraBuilder, invoices.NewHospitalClinicaZonaFrancaDetalleBuilder),
		campo("modalidad_servicio", "WithModalidadServicio", "string", false)),

	experimental(51, "Engarrafadoras", TipoDocumentoFacturaConCredito,
		b(invoices.NewEngarrafadorasBuilder, invoices.NewEngarrafadorasCabeceraBuilder, invoices.NewEngarrafadorasDetalleBuilder)),

	experimental(52, "Venta de Minerales al Banco Central (solo electrónica)", TipoDocumentoFacturaSinCredito,
		b(invoices.NewVentaMineralBCBBuilder, invoices.NewVentaMineralBCBCabeceraBuilder, invoices.NewVentaMineralBCBDetalleBuilder)),

	experimental(53, "Importación y Comercialización de Lubricantes IEHD", TipoDocumentoFacturaConCredito,
		b(invoices.NewLubricantesIehdBuilder, invoices.NewLubricantesIehdCabeceraBuilder, invoices.NewLubricantesIehdDetalleBuilder)),

	experimental(54, "Compra-Venta de Insumos para Biodiésel", TipoDocumentoFacturaSinCredito,
		b(invoices.NewBiodieselBuilder, invoices.NewBiodieselCabeceraBuilder, invoices.NewBiodieselDetalleBuilder)),

	experimental(55, "Comercialización de Combustible", TipoDocumentoFacturaConCredito,
		b(invoices.NewVentaCombustibleSinSubvencionBuilder, invoices.NewVentaCombustibleSinSubvencionCabeceraBuilder, invoices.NewVentaCombustibleSinSubvencionDetalleBuilder)),
}

// sector crea un perfil estándar de factura (recepcionFactura) con los metadatos
// normativos del documento-sector.
func sector(codigo int, nombre string, tipoDoc int, fachada FachadaSDK, bs buildersSector, campos ...CampoSector) *SectorProfile {
	return &SectorProfile{
		Codigo:               codigo,
		Nombre:               nombre,
		TipoFacturaDocumento: tipoDoc,
		Operacion:            OperacionRecepcionFactura,
		Fachada:              fachada,
		// El boleto aéreo (30) es el único sector sin líneas de detalle: su XSD
		// solo define cabecera y no tiene builder de detalle en el SDK.
		ConDetalle: bs.detalle != nil,
		Campos:     campos,
		builders:   bs,
	}
}

// sectorAjuste crea el perfil de un documento de ajuste (notas): se envía por el
// servicio DocumentoAjuste del SIAT en lugar de recepcionFactura.
func sectorAjuste(codigo int, nombre string, facturaCtor, cabeceraCtor, detalleCtor any, campos ...CampoSector) *SectorProfile {
	p := sector(codigo, nombre, TipoDocumentoNotaCreditoDebito, FachadaDocumentoAjuste, b(facturaCtor, cabeceraCtor, detalleCtor), campos...)
	p.Operacion = OperacionDocumentoAjuste
	return p
}

// nota crea un documento de ajuste con los campos estándar de las notas de
// crédito/débito (24, 47, 48).
func nota(codigo int, nombre string, facturaCtor, cabeceraCtor, detalleCtor any, campos ...CampoSector) *SectorProfile {
	p := sectorAjuste(codigo, nombre, facturaCtor, cabeceraCtor, detalleCtor, campos...)
	if codigo == SectorNotaCreditoDebito {
		p.Layout = string(LayoutNotaCreditoDebito)
	}
	return p
}

func notaLayout(codigo int, nombre, layout string, facturaCtor, cabeceraCtor, detalleCtor any, campos ...CampoSector) *SectorProfile {
	p := nota(codigo, nombre, facturaCtor, cabeceraCtor, detalleCtor, campos...)
	p.Layout = layout
	return p
}

// experimental conserva el nombre histórico del grupo de sectores recientes.
// Su cobertura se completa igual que los demás mediante completarEsquemaSDK.
func experimental(codigo int, nombre string, tipoDoc int, bs buildersSector) *SectorProfile {
	p := sector(codigo, nombre, tipoDoc, FachadaPorModalidad, bs)
	if codigo == 52 {
		p.Modalidades = []int{ModalidadElectronica}
	}
	return p
}

func sectorEducativo(codigo int, nombre string, bs buildersSector) *SectorProfile {
	return sector(codigo, nombre, TipoDocumentoFacturaConCredito, FachadaPorModalidad, bs,
		campoE("nombre_estudiante", "WithNombreEstudiante", "string", true, "Nombre del estudiante", "Juan Pérez"),
		campoE("periodo_facturado", "WithPeriodoFacturado", "string", true, "Período facturado", "2026-08"))
}

// sectorEducativoSinCredito variante sin derecho a crédito fiscal: el sector
// educativo dentro de zona franca (46).
func sectorEducativoSinCredito(codigo int, nombre string, bs buildersSector) *SectorProfile {
	return sector(codigo, nombre, TipoDocumentoFacturaSinCredito, FachadaPorModalidad, bs,
		campoE("nombre_estudiante", "WithNombreEstudiante", "string", true, "Nombre del estudiante", "Juan Pérez"),
		campoE("periodo_facturado", "WithPeriodoFacturado", "string", true, "Período facturado", "2026-08"))
}

func b(facturaCtor, cabeceraCtor, detalleCtor any) buildersSector {
	bs := buildersSector{
		factura: func(modalidad int) any {
			root := invocarCtor(facturaCtor)
			llamarMetodo(root, "WithModalidad", false, modalidad)
			return root
		},
		cabecera: func() any { return invocarCtor(cabeceraCtor) },
		detalle:  nil,
	}
	if detalleCtor != nil {
		bs.detalle = func() any { return invocarCtor(detalleCtor) }
	}
	return bs
}

func invocarCtor(ctor any) any {
	out := reflect.ValueOf(ctor).Call(nil)
	if len(out) == 0 {
		panic("el constructor no devolvió valor")
	}
	return out[0].Interface()
}

func campo(json, metodo, tipo string, requerido bool) CampoSector {
	return CampoSector{JSON: json, Metodo: metodo, Tipo: tipo, Requerido: requerido}
}

// campoE es campo con etiqueta y ejemplo para formularios dinámicos.
func campoE(json, metodo, tipo string, requerido bool, etiqueta, ejemplo string) CampoSector {
	c := campo(json, metodo, tipo, requerido)
	c.Etiqueta = etiqueta
	c.Ejemplo = ejemplo
	return c
}

// notasCampos declara los campos específicos del XSD de notas de crédito/débito
// (sectores 24, 47 y 48).
func notasCampos() []CampoSector {
	return []CampoSector{
		campoE("numero_autorizacion_cuf", "WithNumeroAutorizacionCuf", "string", true, "CUF de la factura original", "7894561237894561237894561237894561237894561237894561237894561237894561AA"),
		campoE("fecha_emision_factura", "WithFechaEmisionFactura", "fecha", true, "Fecha de emisión de la factura original", "2026-08-24T00:00:00.000"),
		campoE("monto_total_original", "WithMontoTotalOriginal", "float", true, "Monto total de la factura original", "100.00"),
		campoE("monto_total_devuelto", "WithMontoTotalDevuelto", "float", true, "Monto devuelto por la nota", "25.00"),
		campoE("monto_efectivo_credito_debito", "WithMontoEfectivoCreditoDebito", "float", true, "Monto efectivo de la nota", "75.00"),
		campoE("monto_descuento_credito_debito", "WithMontoDescuentoCreditoDebito", "float", false, "Descuento de la nota", "0.00"),
		campoE("numero_nota_credito_debito", "WithNumeroNotaCreditoDebito", "int", false, "Número de nota de crédito/débito", "1"),
	}
}

func camposServicioBasico() []CampoSector {
	return []CampoSector{
		campo("numero_medidor", "WithNumeroMedidor", "string", false),
		campo("consumo_periodo", "WithConsumoPeriodo", "float", false),
		campo("ciudad", "WithCiudad", "string", false),
		campo("zona", "WithZona", "string", false),
		campo("gestion", "WithGestion", "int", false),
		campo("mes", "WithMes", "string", false),
	}
}

func camposHospedaje() []CampoSector {
	return []CampoSector{
		campo("fecha_ingreso_hospedaje", "WithFechaIngresoHospedaje", "fecha", false),
		campo("cantidad_habitaciones", "WithCantidadHabitaciones", "int", false),
		campo("cantidad_huespedes", "WithCantidadHuespedes", "int", false),
		campo("cantidad_mayores", "WithCantidadMayores", "int", false),
		campo("cantidad_menores", "WithCantidadMenores", "int", false),
	}
}
