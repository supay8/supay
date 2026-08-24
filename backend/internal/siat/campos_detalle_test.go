package siat

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

// baseItemConstruyeFactura arma una solicitud mínima de compraventa con un
// ítem sin DatosSector, lista para comparar XML antes/después de añadir campos
// de detalle.
func baseItemConstruyeFactura(t *testing.T) SolicitudFactura {
	t.Helper()
	return SolicitudFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-TEST",
		Nit:                   "1020304050",
		Modalidad:             ModalidadElectronica,
		NumeroFactura:         1,
		Cuis:                  "CUIS-TEST",
		Cufd:                  "CUFD-TEST",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-01",
		FechaEmision:          time.Date(2025, 8, 15, 10, 30, 0, 0, LaPaz),
		Usuario:               "SUPAY",
		Leyenda:               "Ley N° 453",
		RazonSocialEmisor:     "EMPRESA TEST SRL",
		Municipio:             "LA PAZ",
		Direccion:             "AV. TEST 123",
		CodigoMetodoPago:      1,
		CodigoMoneda:          1,
		TipoCambio:            1,
		MontoTotal:            100,
		CodigoDocumentoSector: SectorCompraVenta,
		Cliente: ClienteFactura{
			NombreRazonSocial:            "CLIENTE TEST",
			CodigoTipoDocumentoIdentidad: 1,
			NumeroDocumento:              "1234567",
			CodigoCliente:                "C-001",
		},
		Items: []ItemFactura{{
			ActividadEconomica: "473000",
			CodigoProductoSin:  12345,
			CodigoProducto:     "P-001",
			Descripcion:        "Ítem de prueba",
			Cantidad:           1,
			UnidadMedida:       1,
			PrecioUnitario:     100,
			SubTotal:           100,
		}},
	}
}

// TestItemSinDatosSectorNoCambiaXML verifica la aditividad del Paso 1: un ítem
// sin DatosSector debe producir exactamente el mismo XML que antes de la
// introducción de CamposDetalle.
func TestItemSinDatosSectorNoCambiaXML(t *testing.T) {
	req := baseItemConstruyeFactura(t)
	// DatosSector nil: el ítem no trae datos sectoriales de detalle.
	req.Items[0].DatosSector = nil

	factura, _, _, err := buildFacturaSDK(req, 1)
	if err != nil {
		t.Fatalf("buildFacturaSDK: %v", err)
	}
	xmlSinDatos, err := xml.Marshal(factura)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}

	// DatosSector como JSON nulo explícito: debe ser equivalente a nil.
	req2 := baseItemConstruyeFactura(t)
	req2.Items[0].DatosSector = json.RawMessage("null")

	factura2, _, _, err := buildFacturaSDK(req2, 1)
	if err != nil {
		t.Fatalf("buildFacturaSDK con null: %v", err)
	}
	xmlConNull, err := xml.Marshal(factura2)
	if err != nil {
		t.Fatalf("xml.Marshal con null: %v", err)
	}

	if string(xmlSinDatos) != string(xmlConNull) {
		t.Fatalf("XML con DatosSector nil != XML con DatosSector null:\n--- nil ---\n%s\n--- null ---\n%s", xmlSinDatos, xmlConNull)
	}

	// DatosSector como objeto vacío: debe ser equivalente a nil (no hay
	// CamposDetalle declarados para compraventa).
	req3 := baseItemConstruyeFactura(t)
	req3.Items[0].DatosSector = json.RawMessage(`{}`)

	factura3, _, _, err := buildFacturaSDK(req3, 1)
	if err != nil {
		t.Fatalf("buildFacturaSDK con objeto vacío: %v", err)
	}
	xmlConVacio, err := xml.Marshal(factura3)
	if err != nil {
		t.Fatalf("xml.Marshal con objeto vacío: %v", err)
	}

	if string(xmlSinDatos) != string(xmlConVacio) {
		t.Fatalf("XML con DatosSector nil != XML con DatosSector {}:\n--- nil ---\n%s\n--- vacío ---\n%s", xmlSinDatos, xmlConVacio)
	}
}

// TestValidarDatosDetalleSinCamposDeclados verifica que ValidarDatosDetalle
// acepta datos vacíos/nulos cuando el perfil no declara CamposDetalle, y
// rechaza claves desconocidas cuando sí las declara.
func TestValidarDatosDetalle(t *testing.T) {
	p, err := PerfilSector(SectorCompraVenta)
	if err != nil {
		t.Fatal(err)
	}

	// Compraventa no declara CamposDetalle: datos vacíos y nulos son válidos.
	if v, err := p.ValidarDatosDetalle(nil); err != nil || len(v) != 0 {
		t.Fatalf("nil debía dar mapa vacío sin error: v=%v err=%v", v, err)
	}
	if v, err := p.ValidarDatosDetalle(json.RawMessage("null")); err != nil || len(v) != 0 {
		t.Fatalf("null debía dar mapa vacío sin error: v=%v err=%v", v, err)
	}

	// Compraventa no declara CamposDetalle: un objeto con claves debe ser
	// rechazado como claves desconocidas.
	_, err = p.ValidarDatosDetalle(json.RawMessage(`{"campo_inventado":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "no reconocidas") {
		t.Fatalf("se esperaba error de claves no reconocidas, got %v", err)
	}
}

func TestBuildFacturaRechazaDetalleRequeridoAusente(t *testing.T) {
	p, err := PerfilSector(SectorCompraVenta)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]CampoSector(nil), p.CamposDetalle...)
	p.CamposDetalle = []CampoSector{
		campo("codigo_producto_sectorial", "WithCodigoProducto", "string", true),
	}
	t.Cleanup(func() { p.CamposDetalle = original })

	req := baseItemConstruyeFactura(t)
	req.Items[0].DatosSector = nil

	_, _, _, err = buildFacturaSDK(req, 1)
	if err == nil || !strings.Contains(err.Error(), "faltan campos obligatorios") {
		t.Fatalf("se esperaba error por datos_sector de detalle requerido ausente, got %v", err)
	}
}
