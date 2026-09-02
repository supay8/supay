package siat

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEnviarMasivaFacturasPayload(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <recepcionMasivaFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>RCV-ETAPA-IX</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionMasivaFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newSignedTestService(t, server.URL)

	req := SolicitudMasivaFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadElectronica,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      1,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-01",
		CodigoDocumentoSector: 1,
		CodigoTipoFactura:     1,
		Facturas: []SolicitudFactura{
			{
				NumeroFactura:     1,
				FechaEmision:      time.Now(),
				Usuario:           "SUPAY",
				Leyenda:           "Ley N° 453",
				RazonSocialEmisor: "EMPRESA TEST SRL",
				Municipio:         "LA PAZ",
				Direccion:         "AV. MOCK 123",
				CodigoMetodoPago:  1,
				CodigoMoneda:      1,
				TipoCambio:        1,
				MontoTotal:        100,
				Cliente: ClienteFactura{
					NombreRazonSocial:            "CLIENTE TEST",
					CodigoTipoDocumentoIdentidad: 1,
					NumeroDocumento:              "1234567",
					CodigoCliente:                ptrStr("C-001"),
				},
				Items: []ItemFactura{
					{
						ActividadEconomica: "473000",
						CodigoProductoSin:  12345,
						CodigoProducto:     "P-001",
						Descripcion:        "Producto de prueba",
						Cantidad:           1,
						UnidadMedida:       1,
						PrecioUnitario:     100,
						SubTotal:           100,
					},
				},
			},
			{
				NumeroFactura:     2,
				FechaEmision:      time.Now(),
				Usuario:           "SUPAY",
				Leyenda:           "Ley N° 453",
				RazonSocialEmisor: "EMPRESA TEST SRL",
				Municipio:         "LA PAZ",
				Direccion:         "AV. MOCK 123",
				CodigoMetodoPago:  1,
				CodigoMoneda:      1,
				TipoCambio:        1,
				MontoTotal:        200,
				Cliente: ClienteFactura{
					NombreRazonSocial:            "CLIENTE TEST 2",
					CodigoTipoDocumentoIdentidad: 1,
					NumeroDocumento:              "7654321",
					CodigoCliente:                ptrStr("C-002"),
				},
				Items: []ItemFactura{
					{
						ActividadEconomica: "473000",
						CodigoProductoSin:  12345,
						CodigoProducto:     "P-002",
						Descripcion:        "Producto de prueba 2",
						Cantidad:           1,
						UnidadMedida:       1,
						PrecioUnitario:     200,
						SubTotal:           200,
					},
				},
			},
		},
	}

	result, err := svc.EnviarMasivaFacturas(t.Context(), req)
	if err != nil {
		t.Fatalf("EnviarMasivaFacturas: %v", err)
	}

	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}
	if result.CodigoRecepcion != "RCV-ETAPA-IX" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}
	if result.CantidadFacturas != 2 {
		t.Fatalf("se esperaba cantidadFacturas 2, got %d", result.CantidadFacturas)
	}
	if len(result.Cufs) != 2 {
		t.Fatalf("se esperaban 2 CUF, got %d", len(result.Cufs))
	}

	for _, want := range []string{
		"recepcionMasivaFactura",
		"SolicitudServicioRecepcionMasiva",
		"<codigoEmision>3</codigoEmision>",
		"<cantidadFacturas>2</cantidadFacturas>",
		"<tipoFacturaDocumento>1</tipoFacturaDocumento>",
		"<codigoDocumentoSector>1</codigoDocumentoSector>",
		"<codigoModalidad>1</codigoModalidad>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
		"<nit>1020304050</nit>",
		"<hashArchivo>" + result.HashArchivo + "</hashArchivo>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q", want)
		}
	}

	assertFechaEnvioEnLaPaz(t, gotBody)

	// El archivo enviado debe ser un TAR.GZ válido con una entrada por factura,
	// y el XML de cada factura debe estar firmado (modalidad electrónica).
	compressed, err := base64.StdEncoding.DecodeString(result.Archivo)
	if err != nil {
		t.Fatalf("archivo no es base64 válido: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("archivo no es gzip válido: %v", err)
	}
	tarReader := tar.NewReader(gz)
	firmadas := 0
	entries := 0
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("archivo no es tar válido: %v", err)
		}
		entries++
		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("no se pudo leer entrada %s: %v", hdr.Name, err)
		}
		if !strings.Contains(hdr.Name, "factura_") || !strings.HasSuffix(hdr.Name, ".xml") {
			t.Fatalf("nombre de entrada inesperado: %q", hdr.Name)
		}
		if strings.Contains(string(content), "<ds:Signature") {
			firmadas++
		}
	}
	if entries != 2 {
		t.Fatalf("se esperaban 2 entradas tar, got %d", entries)
	}
	if firmadas != 2 {
		t.Fatalf("se esperaban 2 facturas firmadas, got %d", firmadas)
	}
}
