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

func facturaDePrueba(numero int64) SolicitudFactura {
	return SolicitudFactura{
		NumeroFactura:         numero,
		FechaEmision:          time.Now(),
		Usuario:               "SUPAY",
		Leyenda:               "Ley N° 453",
		RazonSocialEmisor:     "EMPRESA TEST SRL",
		Municipio:             "LA PAZ",
		Direccion:             "AV. MOCK 123",
		CodigoMetodoPago:      1,
		CodigoMoneda:          1,
		TipoCambio:            1,
		MontoTotal:            100,
		CodigoDocumentoSector: SectorCompraVenta,
		CodigoTipoFactura:     1,
		Cliente: ClienteFactura{
			NombreRazonSocial:            "CLIENTE TEST",
			CodigoTipoDocumentoIdentidad: 1,
			NumeroDocumento:              "1234567",
			CodigoCliente:                "C-001",
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
	}
}

func paqueteDePrueba() SolicitudPaqueteFactura {
	return SolicitudPaqueteFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadComputarizada,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      0,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-01",
		CodigoDocumentoSector: SectorCompraVenta,
		CodigoTipoFactura:     1,
		CodigoEmision:         EmisionPaqueteOffline,
		CodigoEvento:          12345,
		Descripcion:           "CORTE DEL SERVICIO DE INTERNET",
		Facturas: []SolicitudFactura{
			facturaDePrueba(100),
			facturaDePrueba(101),
		},
	}
}

func TestEnviarPaqueteFacturaPayload(t *testing.T) {
	var gotAPIKey string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("apiKey")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <recepcionPaqueteFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>RCV-PAQ-ETAPA-IV</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionPaqueteFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	result, err := svc.EnviarPaqueteFactura(t.Context(), paqueteDePrueba())
	if err != nil {
		t.Fatalf("EnviarPaqueteFactura: %v", err)
	}

	if gotAPIKey != "TokenApi test-token" {
		t.Fatalf("unexpected apiKey header: %q", gotAPIKey)
	}
	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}
	if result.CodigoRecepcion != "RCV-PAQ-ETAPA-IV" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}
	if result.CantidadFacturas != 2 {
		t.Fatalf("unexpected cantidadFacturas: %d", result.CantidadFacturas)
	}
	if len(result.Cufs) != 2 {
		t.Fatalf("se esperaban 2 CUF, se obtuvieron %d", len(result.Cufs))
	}
	for i, cuf := range result.Cufs {
		if cuf == "" {
			t.Fatalf("el CUF %d está vacío", i)
		}
	}
	if result.Archivo == "" || result.HashArchivo == "" {
		t.Fatal("se esperaba archivo (base64 tar.gz) y hashArchivo en el resultado")
	}

	// El archivo debe ser un TAR.GZ con un XML por factura (factura_N.xml).
	compressed, err := base64.StdEncoding.DecodeString(result.Archivo)
	if err != nil {
		t.Fatalf("archivo no es base64 válido: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("archivo no es gzip válido: %v", err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error leyendo tar: %v", err)
		}
		names = append(names, hdr.Name)
		if !strings.HasPrefix(hdr.Name, "factura_") || !strings.HasSuffix(hdr.Name, ".xml") {
			t.Fatalf("entrada tar inesperada: %q", hdr.Name)
		}
	}
	if len(names) != 2 {
		t.Fatalf("se esperaban 2 XMLs en el tar, se obtuvieron %d: %v", len(names), names)
	}
	_ = gz.Close()

	for _, want := range []string{
		"recepcionPaqueteFactura",
		"SolicitudServicioRecepcionPaquete",
		"<codigoEmision>2</codigoEmision>",
		"<tipoFacturaDocumento>1</tipoFacturaDocumento>",
		"<codigoDocumentoSector>1</codigoDocumentoSector>",
		"<codigoModalidad>2</codigoModalidad>",
		"<cantidadFacturas>2</cantidadFacturas>",
		"<codigoEvento>12345</codigoEvento>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
		"<nit>1020304050</nit>",
		"<hashArchivo>" + result.HashArchivo + "</hashArchivo>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q", want)
		}
	}
	if !strings.Contains(gotBody, "<archivo>"+result.Archivo+"</archivo>") {
		t.Error("el payload SOAP no contiene el archivo base64 enviado")
	}
}

func TestValidarPaqueteFactura(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <validacionRecepcionPaqueteFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>RCV-PAQ-ETAPA-IV</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </validacionRecepcionPaqueteFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	result, err := svc.ValidarPaqueteFactura(t.Context(), paqueteDePrueba(), "RCV-PAQ-ETAPA-IV")
	if err != nil {
		t.Fatalf("ValidarPaqueteFactura: %v", err)
	}

	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}
	if result.CodigoEstado != 908 {
		t.Fatalf("unexpected codigoEstado: %d", result.CodigoEstado)
	}
	if result.CodigoRecepcion != "RCV-PAQ-ETAPA-IV" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}

	for _, want := range []string{
		"validacionRecepcionPaqueteFactura",
		"SolicitudServicioValidacionRecepcionPaquete",
		"<codigoRecepcion>RCV-PAQ-ETAPA-IV</codigoRecepcion>",
		"<codigoEmision>2</codigoEmision>",
		"<codigoModalidad>2</codigoModalidad>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP de validación no contiene %q", want)
		}
	}
}

func TestEnviarPaqueteFacturaValidation(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")

	if _, err := svc.EnviarPaqueteFactura(t.Context(), SolicitudPaqueteFactura{
		CodigoAmbiente: 99,
	}); err == nil {
		t.Fatal("se esperaba error de validación por codigoAmbiente inválido")
	}

	req := paqueteDePrueba()
	req.CodigoEvento = 0
	if _, err := svc.EnviarPaqueteFactura(t.Context(), req); err == nil {
		t.Fatal("se esperaba error de validación por falta de codigoEvento")
	}

	req = paqueteDePrueba()
	req.Facturas = nil
	if _, err := svc.EnviarPaqueteFactura(t.Context(), req); err == nil {
		t.Fatal("se esperaba error de validación por paquete vacío")
	}

	req = paqueteDePrueba()
	for i := 0; i < MaxFacturasPorPaquete+1; i++ {
		f := facturaDePrueba(int64(100 + i))
		req.Facturas = append(req.Facturas, f)
	}
	if _, err := svc.EnviarPaqueteFactura(t.Context(), req); err == nil {
		t.Fatalf("se esperaba error de validación por superar el límite de %d facturas", MaxFacturasPorPaquete)
	}

	if _, err := svc.ValidarPaqueteFactura(t.Context(), paqueteDePrueba(), "  "); err == nil {
		t.Fatal("se esperaba error por codigoRecepcion vacío")
	}
}

func TestEnviarPaqueteFacturaInheritsIdentity(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <recepcionPaqueteFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>RCV-PAQ-02</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionPaqueteFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	// La factura no trae identidad propia: debe heredarla del paquete.
	factura := facturaDePrueba(200)
	factura.CodigoAmbiente = 0
	factura.CodigoSistema = ""
	factura.Nit = ""
	factura.Modalidad = 0
	factura.Cuis = ""
	factura.Cufd = ""
	factura.CodigoControl = ""
	factura.CodigoSucursal = 0
	factura.CodigoPuntoVenta = 0

	req := paqueteDePrueba()
	req.Facturas = []SolicitudFactura{factura}

	if _, err := svc.EnviarPaqueteFactura(t.Context(), req); err != nil {
		t.Fatalf("EnviarPaqueteFactura: %v", err)
	}

	for _, want := range []string{
		"<nit>1020304050</nit>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
		"<cantidadFacturas>1</cantidadFacturas>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q (identidad no heredada)", want)
		}
	}
}
