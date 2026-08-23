package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
)

func main() {
	fmt.Println("=== Extracción Masiva de Catálogos SIAT a JSON ===")

	nit := int64(9971522011)
	token := "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9.eyJzdWIiOiJyYW1pdHByNTNAZ21haWwuY29tIiwiY29kaWdvU2lzdGVtYSI6IjIyODQ1MkMzOEVEODczOTQwOEFCNiIsIm5pdCI6Ikg0c0lBQUFBQUFBQUFMTzBORGMwTlRJeU1EUUVBRVhzZmhJS0FBQUEiLCJpZCI6NjE1NjMyNSwiZXhwIjoxNzg4MTkyNDEwLCJpYXQiOjE3ODY2NTE1ODAsIm5pdERlbGVnYWRvIjo5OTcxNTIyMDExLCJzdWJzaXN0ZW1hIjoiU0ZFIn0.PuB5t3vfE3iBf_mT5MvBt2tDyTALnVRBtCX1Cf0KYAgaWNQeT7L3hrrlYY2Ggdsn8cVPnQJxemJfAvRs86Svvw"
	codigoSistema := "228452C38ED8739408AB6"
	baseURL := "https://pilotosiatservicios.impuestos.gob.bo/v2"
	cuis := "F7404ABC"

	cfg := siat.Config{
		Nit:            nit,
		Token:          token,
		CodigoSistema:  codigoSistema,
		CodigoAmbiente: siat.AmbientePruebas, // 2 = Piloto
		BaseURL:        baseURL,
	}

	s, err := siat.New(cfg)
	if err != nil {
		log.Fatalf("Error inicializando cliente: %v", err)
	}

	ctx := context.Background()

	// Diccionario maestro para guardar todo el JSON
	catalogos := make(map[string]interface{})

	// ---------------------------------------------------------
	// 1. ACTIVIDADES Y PRODUCTOS
	// ---------------------------------------------------------
	fmt.Println("⏳ Descargando Actividades...")
	reqAct := models.NewSincronizarActividadesBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respAct, _ := s.Sincronizacion().SincronizarActividades(ctx, reqAct)
	catalogos["actividades"] = respAct.Body.Content.RespuestaListaActividades.ListaActividades

	fmt.Println("⏳ Descargando Actividades - Documento Sector...")
	reqDocSec := models.NewSincronizarListaActividadesDocumentoSectorBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respDocSec, _ := s.Sincronizacion().SincronizarListaActividadesDocumentoSector(ctx, reqDocSec)
	catalogos["actividades_documento_sector"] = respDocSec.Body.Content.RespuestaListaActividadesDocumentoSector.ListaActividadesDocumentoSector

	fmt.Println("⏳ Descargando Productos y Servicios SIN...")
	reqProd := models.NewSincronizarListaProductosServiciosBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respProd, _ := s.Sincronizacion().SincronizarListaProductosServicios(ctx, reqProd)
	catalogos["productos_sin"] = respProd.Body.Content.RespuestaListaProductos.ListaCodigos

	// ---------------------------------------------------------
	// 2. PARAMÉTRICAS GENERALES
	// ---------------------------------------------------------
	fmt.Println("⏳ Descargando Paramétrica: Tipo Documento Identidad...")
	reqId := models.NewSincronizarParametricaTipoDocumentoIdentidadBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respId, _ := s.Sincronizacion().SincronizarParametricaTipoDocumentoIdentidad(ctx, reqId)
	catalogos["param_tipo_documento_identidad"] = respId.Body.Content.RespuestaListaParametricas.ListaCodigos

	fmt.Println("⏳ Descargando Paramétrica: Tipo Método Pago...")
	reqPago := models.NewSincronizarParametricaTipoMetodoPagoBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respPago, _ := s.Sincronizacion().SincronizarParametricaTipoMetodoPago(ctx, reqPago)
	catalogos["param_tipo_metodo_pago"] = respPago.Body.Content.RespuestaListaParametricas.ListaCodigos

	fmt.Println("⏳ Descargando Paramétrica: Unidad de Medida...")
	reqUm := models.NewSincronizarParametricaUnidadMedidaBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respUm, _ := s.Sincronizacion().SincronizarParametricaUnidadMedida(ctx, reqUm)
	catalogos["param_unidad_medida"] = respUm.Body.Content.RespuestaListaParametricas.ListaCodigos

	fmt.Println("⏳ Descargando Paramétrica: Tipo Moneda...")
	reqMon := models.NewSincronizarParametricaTipoMonedaBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respMon, _ := s.Sincronizacion().SincronizarParametricaTipoMoneda(ctx, reqMon)
	catalogos["param_tipo_moneda"] = respMon.Body.Content.RespuestaListaParametricas.ListaCodigos

	fmt.Println("⏳ Descargando Paramétrica: Tipo Documento Sector...")
	reqTds := models.NewSincronizarParametricaTipoDocumentoSectorBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respTds, _ := s.Sincronizacion().SincronizarParametricaTipoDocumentoSector(ctx, reqTds)
	catalogos["param_tipo_documento_sector"] = respTds.Body.Content.RespuestaListaParametricas.ListaCodigos

	fmt.Println("⏳ Descargando Paramétrica: Motivo Anulación...")
	reqAnul := models.NewSincronizarParametricaMotivoAnulacionBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respAnul, _ := s.Sincronizacion().SincronizarParametricaMotivoAnulacion(ctx, reqAnul)
	catalogos["param_motivo_anulacion"] = respAnul.Body.Content.RespuestaListaParametricas.ListaCodigos

	// ---------------------------------------------------------
	// 3. TEXTOS Y LEYENDAS
	// ---------------------------------------------------------
	fmt.Println("⏳ Descargando Leyendas de Factura...")
	reqLeyendas := models.NewSincronizarListaLeyendasFacturaBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respLeyendas, _ := s.Sincronizacion().SincronizarListaLeyendasFactura(ctx, reqLeyendas)
	catalogos["leyendas_factura"] = respLeyendas.Body.Content.RespuestaListaParametricasLeyendas.ListaLeyendas

	fmt.Println("⏳ Descargando Mensajes de Servicios...")
	reqMsj := models.NewSincronizarListaMensajesServiciosBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respMsj, _ := s.Sincronizacion().SincronizarListaMensajesServicios(ctx, reqMsj)
	catalogos["mensajes_servicios"] = respMsj.Body.Content.RespuestaListaParametricas.ListaCodigos

	// ---------------------------------------------------------
	// 4. EVENTOS SIGNIFICATIVOS
	// ---------------------------------------------------------
	fmt.Println("⏳ Eventos Significativos...")
	reqEventos := models.NewSincronizarParametricaEventosSignificativosBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respEventos, _ := s.Sincronizacion().SincronizarParametricaEventosSignificativos(ctx, reqEventos)
	catalogos["eventos_significativos"] = respEventos.Body.Content.RespuestaListaParametricas.ListaCodigos

	/// Tipo de emision
	fmt.Println("⏳ Tipo de Emisión...")
	reqTipoEmision := models.NewSincronizarParametricaTipoEmisionBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respTipoEmision, _ := s.Sincronizacion().SincronizarParametricaTipoEmision(ctx, reqTipoEmision)
	catalogos["tipo_emision"] = respTipoEmision.Body.Content.RespuestaListaParametricas.ListaCodigos

	/// Tipo de factura
	fmt.Println("⏳ Tipo de Factura...")
	reqTipoFactura := models.NewSincronizarParametricaTiposFacturaBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respTipoFactura, _ := s.Sincronizacion().SincronizarParametricaTiposFactura(ctx, reqTipoFactura)
	catalogos["tipo_factura"] = respTipoFactura.Body.Content.RespuestaListaParametricas.ListaCodigos

	// Tipo de Punto de venta
	fmt.Println("⏳ Tipo de Punto de Venta...")
	reqTipoPdv := models.NewSincronizarParametricaTipoPuntoVentaBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respTipoPdv, _ := s.Sincronizacion().SincronizarParametricaTipoPuntoVenta(ctx, reqTipoPdv)
	catalogos["tipo_punto_venta"] = respTipoPdv.Body.Content.RespuestaListaParametricas.ListaCodigos

	// fecha y HOra del siat
	fmt.Println("⏳ Fecha y Hora del SIAT...")
	reqFechaHora := models.NewSincronizarFechaHoraBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(0).WithCuis(cuis).Build()
	respFechaHora, _ := s.Sincronizacion().SincronizarFechaHora(ctx, reqFechaHora)
	catalogos["fecha_hora_siat"] = respFechaHora.Body.Content.RespuestaFechaHora.FechaHora

	// ---------------------------------------------------------
	// GUARDAR TODO EN UN ARCHIVO JSON
	// ---------------------------------------------------------
	fmt.Println("💾 Procesando y guardando datos...")

	// MarshalIndent lo convierte a formato JSON bonito (pretty-print)
	jsonData, err := json.MarshalIndent(catalogos, "", "  ")
	if err != nil {
		log.Fatalf("Error al formatear JSON: %v", err)
	}

	err = os.WriteFile("catalogos_siat.json", jsonData, 0644)
	if err != nil {
		log.Fatalf("Error al guardar archivo: %v", err)
	}

	fmt.Println("✅ ¡Éxito! Abre el archivo 'catalogos_siat.json' en la raíz de tu proyecto para ver todas las estructuras.")
}
