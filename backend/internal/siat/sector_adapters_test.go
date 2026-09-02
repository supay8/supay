package siat

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestCompraVentaAdapterNormalizaPayloadTipado(t *testing.T) {
	p, err := PerfilSector(SectorCompraVenta)
	if err != nil {
		t.Fatal(err)
	}
	descuento := 1.239
	req := SolicitudFactura{
		Modalidad:             ModalidadComputarizada,
		CodigoDocumentoSector: SectorCompraVenta,
		MontoTotal:            10.129,
		TipoCambio:            6.967,
		DatosSector:           nil,
		Items: []ItemFactura{{
			Cantidad:       2.345,
			PrecioUnitario: 4.567,
			SubTotal:       10.129,
			MontoDescuento: &descuento,
		}},
	}

	doc, err := p.adapter.Prepare(p, req)
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := doc.Payload.(CompraVentaPayload)
	if !ok {
		t.Fatalf("payload = %T, se esperaba CompraVentaPayload", doc.Payload)
	}
	// Tras Fase A, MontoTotal y SubTotal se recalculan dinámicamente (quantity*price - discount)
	// 2.35*4.57 -1.24 = 9.5, por lo que MontoTotal debe ser 9.5, no 10.13 (corrige valor quemado 1013/1018)
	if payload.MontoTotal != 9.5 || payload.TipoCambio != 6.97 {
		t.Fatalf("montos no normalizados: total=%v cambio=%v (esperado 9.5/6.97)", payload.MontoTotal, payload.TipoCambio)
	}
	item := payload.Items[0]
	if item.Cantidad != 2.35 || item.PrecioUnitario != 4.57 || item.SubTotal != 9.5 {
		t.Fatalf("detalle no normalizado: %+v (esperado subtotal 9.5)", item)
	}
	if item.MontoDescuento == nil || *item.MontoDescuento != 1.24 {
		t.Fatalf("descuento no normalizado: %v", item.MontoDescuento)
	}
	if req.MontoTotal != 10.129 || req.Items[0].Cantidad != 2.345 {
		t.Fatal("el adaptador mutó el request original")
	}
}

func TestSectorDocumentDistingueAusenteDeNulo(t *testing.T) {
	present := presentSectorFields([]byte(`{"ausente_no":null,"vacio":""}`))
	null := nullSectorFields([]byte(`{"ausente_no":null,"vacio":""}`))
	if !present["ausente_no"] || !present["vacio"] {
		t.Fatalf("presencia perdida: %#v", present)
	}
	if !null["ausente_no"] || null["vacio"] {
		t.Fatalf("nulabilidad incorrecta: %#v", null)
	}
	if present["realmente_ausente"] || null["realmente_ausente"] {
		t.Fatal("un campo ausente no debe marcarse presente ni nulo")
	}
}

func TestCompraVentaBuilderSeleccionaModalidad(t *testing.T) {
	base := SolicitudFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		NumeroFactura:         11,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      0,
		Cuis:                  "CUIS",
		Cufd:                  "CUFD",
		FechaEmision:          time.Date(2026, 1, 2, 10, 30, 0, 0, LaPaz),
		MontoTotal:            100,
		CodigoDocumentoSector: SectorCompraVenta,
		Cliente: ClienteFactura{
			NombreRazonSocial:            "CLIENTE",
			CodigoTipoDocumentoIdentidad: 1,
			NumeroDocumento:              "123",
			CodigoCliente:                ptrStr("C-1"),
		},
		Items: []ItemFactura{{
			ActividadEconomica: "473000",
			CodigoProductoSin:  123,
			CodigoProducto:     "P-1",
			Descripcion:        "Producto",
			Cantidad:           1,
			UnidadMedida:       1,
			PrecioUnitario:     100,
			SubTotal:           100,
		}},
	}
	for _, tc := range []struct {
		modalidad int
		root      string
	}{
		{ModalidadComputarizada, "facturaComputarizadaCompraVenta"},
		{ModalidadElectronica, "facturaElectronicaCompraVenta"},
	} {
		req := base
		req.Modalidad = tc.modalidad
		factura, _, _, err := buildFacturaSDK(req, 1)
		if err != nil {
			t.Fatalf("modalidad %d: %v", tc.modalidad, err)
		}
		data, err := xml.Marshal(factura)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), tc.root) {
			t.Fatalf("modalidad %d: XML no contiene %q", tc.modalidad, tc.root)
		}
	}
}

func TestBuildFacturaSDKRechazaModalidadNoHabilitada(t *testing.T) {
	_, _, _, err := buildFacturaSDK(SolicitudFactura{
		CodigoDocumentoSector: SectorCompraVenta,
		Modalidad:             99,
	}, 1)
	if err == nil || !strings.Contains(err.Error(), "modalidad") {
		t.Fatalf("se esperaba error de modalidad, got %v", err)
	}
}

func TestRoundMoneyUsesTwoDecimals(t *testing.T) {
	if got := roundMoney(1.234); got != 1.23 {
		t.Fatalf("roundMoney(1.234) = %v, want 1.23", got)
	}
}
