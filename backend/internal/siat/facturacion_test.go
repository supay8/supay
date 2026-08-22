package siat

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
)

func TestEmpaquetaArchivo(t *testing.T) {
	data := []byte(`<factura><numeroFactura>100</numeroFactura></factura>`)

	archivo, hash, err := empaquetaArchivo(data)
	if err != nil {
		t.Fatalf("empaquetaArchivo: %v", err)
	}

	compressed, err := base64.StdEncoding.DecodeString(archivo)
	if err != nil {
		t.Fatalf("archivo no es base64 válido: %v", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("archivo no es gzip válido: %v", err)
	}
	uncompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("no se pudo descomprimir: %v", err)
	}
	_ = reader.Close()

	if !bytes.Equal(uncompressed, data) {
		t.Fatalf("contenido descomprimido no coincide: %q", uncompressed)
	}

	sum := sha256.Sum256(compressed)
	if want := hex.EncodeToString(sum[:]); hash != want {
		t.Fatalf("hash SHA256 no coincide: got %q want %q", hash, want)
	}
}

func TestEmitirFacturaCompraVentaPayload(t *testing.T) {
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
    <recepcionFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>RCV-ETAPA-IV</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	req := SolicitudFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadComputarizada,
		NumeroFactura:         100,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      0,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-01",
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

	result, err := svc.EmitirFactura(t.Context(), req)
	if err != nil {
		t.Fatalf("EmitirFactura: %v", err)
	}

	if gotAPIKey != "TokenApi test-token" {
		t.Fatalf("unexpected apiKey header: %q", gotAPIKey)
	}
	if result.Cuf == "" {
		t.Fatal("se esperaba CUF generado")
	}
	if result.CodigoRecepcion != "RCV-ETAPA-IV" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}
	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}
	if result.XmlHash == "" || result.Archivo == "" {
		t.Fatal("se esperaba XmlHash y Archivo (base64 gzip)")
	}

	for _, want := range []string{
		"recepcionFactura",
		"SolicitudServicioRecepcionFactura",
		"<codigoEmision>1</codigoEmision>",
		"<tipoFacturaDocumento>1</tipoFacturaDocumento>",
		"<codigoDocumentoSector>1</codigoDocumentoSector>",
		"<codigoModalidad>2</codigoModalidad>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
		"<nit>1020304050</nit>",
		"<hashArchivo>" + result.XmlHash + "</hashArchivo>",
		"<archivo>" + result.Archivo + "</archivo>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q", want)
		}
	}

	assertFechaEnvioEnLaPaz(t, gotBody)

	assertFacturaXMLSinXsiNil(t, result.Archivo)
}

func TestEmitirFacturaSectorEducativoPayload(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <recepcionFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>RCV-FSEDU</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	req := SolicitudFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadComputarizada,
		NumeroFactura:         200,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      0,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-02",
		FechaEmision:          time.Now(),
		Usuario:               "SUPAY",
		Leyenda:               "Ley N° 453",
		RazonSocialEmisor:     "UNIVERSIDAD TEST SRL",
		Municipio:             "LA PAZ",
		Direccion:             "AV. MOCK 456",
		CodigoMetodoPago:      1,
		CodigoMoneda:          1,
		TipoCambio:            1,
		MontoTotal:            150,
		CodigoDocumentoSector: SectorEducativo,
		CodigoTipoFactura:     1,
		NombreEstudiante:      "MARIA TEST",
		PeriodoFacturado:      "2026-1",
		Cliente: ClienteFactura{
			NombreRazonSocial:            "MARIA TEST",
			CodigoTipoDocumentoIdentidad: 1,
			NumeroDocumento:              "7654321",
			CodigoCliente:                "C-002",
		},
		Items: []ItemFactura{
			{
				ActividadEconomica: "8549100",
				CodigoProductoSin:  67890,
				CodigoProducto:     "P-002",
				Descripcion:        "Matrícula",
				Cantidad:           1,
				UnidadMedida:       1,
				PrecioUnitario:     150,
				SubTotal:           150,
			},
		},
	}

	result, err := svc.EmitirFactura(t.Context(), req)
	if err != nil {
		t.Fatalf("EmitirFactura (sector educativo): %v", err)
	}

	if !strings.Contains(gotBody, "<codigoDocumentoSector>11</codigoDocumentoSector>") {
		t.Error("el payload SOAP no declara codigoDocumentoSector=11")
	}
	if result.CodigoRecepcion != "RCV-FSEDU" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}

	assertFacturaXMLSinXsiNil(t, result.Archivo)
}

func TestEmitirFacturaTasaCeroPayload(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <recepcionFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>RCV-TASA0</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	req := SolicitudFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadComputarizada,
		NumeroFactura:         300,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      0,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-03",
		FechaEmision:          time.Now(),
		Usuario:               "SUPAY",
		Leyenda:               "Ley N° 453",
		RazonSocialEmisor:     "EMPRESA TEST SRL",
		Municipio:             "LA PAZ",
		Direccion:             "AV. MOCK 123",
		CodigoMetodoPago:      1,
		CodigoMoneda:          1,
		TipoCambio:            1,
		MontoTotal:            200,
		CodigoDocumentoSector: SectorTasaCero,
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
				Descripcion:        "Producto tasa cero",
				Cantidad:           2,
				UnidadMedida:       1,
				PrecioUnitario:     100,
				SubTotal:           200,
			},
		},
	}

	result, err := svc.EmitirFactura(t.Context(), req)
	if err != nil {
		t.Fatalf("EmitirFactura (tasa cero): %v", err)
	}

	if !strings.Contains(gotBody, "<codigoDocumentoSector>8</codigoDocumentoSector>") {
		t.Error("el payload SOAP no declara codigoDocumentoSector=8")
	}
	if result.CodigoRecepcion != "RCV-TASA0" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}

	raw, err := base64.StdEncoding.DecodeString(result.Archivo)
	if err != nil {
		t.Fatalf("decodificando base64: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("abriendo gzip: %v", err)
	}
	defer zr.Close()
	xmlData, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("descomprimiendo: %v", err)
	}
	xmlStr := string(xmlData)

	if !strings.Contains(xmlStr, "facturaComputarizadaTasaCero") {
		t.Errorf("el XML debe usar la raíz facturaComputarizadaTasaCero, got:\n%s", xmlStr[:min(len(xmlStr), 200)])
	}
	if !strings.Contains(xmlStr, "<montoTotalSujetoIva>0</montoTotalSujetoIva>") {
		t.Error("montoTotalSujetoIva debe ser 0 para Tasa Cero")
	}
	if !strings.Contains(xmlStr, "<codigoDocumentoSector>8</codigoDocumentoSector>") {
		t.Error("el XML debe contener codigoDocumentoSector=8")
	}
}

// assertFacturaXMLSinXsiNil descomprime el archivo gzip+Base64 del SIAT y
// verifica que ningún campo opcional de la factura viaje con xsi:nil="true":
// el SIAT rechaza ese atributo (Undeclared namespace prefix 'xsi'). Solo se
// tolera en numeroSerie/numeroImei del detalle de compraventa, que no tienen
// builder público en el SDK.
func assertFacturaXMLSinXsiNil(t *testing.T, archivo string) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(archivo)
	if err != nil {
		t.Fatalf("decodificando base64 del archivo: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("abriendo gzip del archivo: %v", err)
	}
	defer zr.Close()
	xmlData, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("descomprimiendo el archivo: %v", err)
	}
	xmlStr := string(xmlData)

	for _, campo := range []string{
		"telefono", "complemento", "numeroTarjeta", "montoGiftCard",
		"descuentoAdicional", "codigoExcepcion", "cafc", "montoDescuento",
	} {
		if strings.Contains(xmlStr, "<"+campo+" xsi:nil=\"true\">") {
			t.Errorf("el campo %q NO debe viajar con xsi:nil en el XML:\n%s", campo, xmlStr)
		}
	}

	sinExcepciones := strings.ReplaceAll(xmlStr, "<numeroSerie xsi:nil=\"true\">", "")
	sinExcepciones = strings.ReplaceAll(sinExcepciones, "<numeroImei xsi:nil=\"true\">", "")
	if strings.Contains(sinExcepciones, "xsi:nil=\"true\"") {
		t.Errorf("el XML contiene xsi:nil fuera de numeroSerie/numeroImei:\n%s", xmlStr)
	}
}

func TestAnularFacturaPayload(t *testing.T) {
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
    <anulacionFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>905</codigoEstado>
        <codigoRecepcion>RCV-ETAPA-VII</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </anulacionFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	req := SolicitudDocumento{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadElectronica,
		Cuf:                   "B01D-ETAPA-VII-CUF-DE-LA-FACTURA",
		CodigoSucursal:        0,
		CodigoPuntoVenta:      1,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoDocumentoSector: 1,
		CodigoTipoFactura:     1,
	}

	result, err := svc.AnularFactura(t.Context(), req, 1)
	if err != nil {
		t.Fatalf("AnularFactura: %v", err)
	}

	if gotAPIKey != "TokenApi test-token" {
		t.Fatalf("unexpected apiKey header: %q", gotAPIKey)
	}
	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}
	if result.CodigoEstado != 905 {
		t.Fatalf("se esperaba codigoEstado 905 (ANULACION CONFIRMADA), got %d", result.CodigoEstado)
	}
	if result.CodigoRecepcion != "RCV-ETAPA-VII" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}

	for _, want := range []string{
		"anulacionFactura",
		"SolicitudServicioAnulacionFactura",
		"<codigoEmision>1</codigoEmision>",
		"<codigoMotivo>1</codigoMotivo>",
		"<cuf>B01D-ETAPA-VII-CUF-DE-LA-FACTURA</cuf>",
		"<tipoFacturaDocumento>1</tipoFacturaDocumento>",
		"<codigoDocumentoSector>1</codigoDocumentoSector>",
		"<codigoModalidad>1</codigoModalidad>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
		"<nit>1020304050</nit>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q", want)
		}
	}
}

func TestRevertirAnulacionPayload(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <reversionAnulacionFacturaResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>907</codigoEstado>
        <codigoRecepcion>RCV-ETAPA-X</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </reversionAnulacionFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	req := SolicitudDocumento{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadElectronica,
		Cuf:                   "B01D-ETAPA-X-CUF-DE-LA-FACTURA",
		CodigoSucursal:        0,
		CodigoPuntoVenta:      1,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoDocumentoSector: 1,
		CodigoTipoFactura:     1,
	}

	result, err := svc.RevertirAnulacion(t.Context(), req)
	if err != nil {
		t.Fatalf("RevertirAnulacion: %v", err)
	}

	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}
	if result.CodigoEstado != 907 {
		t.Fatalf("se esperaba codigoEstado 907 (REVERSION DE ANULACION CONFIRMADA), got %d", result.CodigoEstado)
	}
	if result.CodigoRecepcion != "RCV-ETAPA-X" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}

	for _, want := range []string{
		"reversionAnulacionFactura",
		"SolicitudServicioReversionAnulacionFactura",
		"<codigoEmision>1</codigoEmision>",
		"<cuf>B01D-ETAPA-X-CUF-DE-LA-FACTURA</cuf>",
		"<tipoFacturaDocumento>1</tipoFacturaDocumento>",
		"<codigoDocumentoSector>1</codigoDocumentoSector>",
		"<codigoModalidad>1</codigoModalidad>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
		"<nit>1020304050</nit>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q", want)
		}
	}

	// El servicio de reversión del SIAT no acepta codigoMotivo: la solicitud
	// (SolicitudServicioReversionAnulacionFactura) solo lleva la identidad y el
	// CUF de la factura a revertir.
	if strings.Contains(gotBody, "codigoMotivo") {
		t.Error("la reversionAnulacionFactura no debe enviar codigoMotivo")
	}
}

func TestFirmarFacturaXMLFirmaDigital(t *testing.T) {
	svc := newSignedTestService(t, "http://localhost:9999")

	xml := `<factura xmlns="https://siat.impuestos.gob.bo/"><nitEmisor>1020304050</nitEmisor><cufd>CUFD-TEST-001</cufd></factura>`
	result, err := svc.FirmarFacturaXML(t.Context(), xml)
	if err != nil {
		t.Fatalf("FirmarFacturaXML: %v", err)
	}

	if !strings.Contains(result.XmlFirmado, "<ds:Signature") {
		t.Fatalf("el XML firmado no contiene <ds:Signature>: %s", result.XmlFirmado)
	}
	if strings.Contains(result.XmlFirmado, xml) == false && !strings.Contains(result.XmlFirmado, "nitEmisor") {
		t.Fatalf("el XML firmado no preserva el documento original: %s", result.XmlFirmado)
	}

	compressed, err := base64.StdEncoding.DecodeString(result.Archivo)
	if err != nil {
		t.Fatalf("archivo no es base64 válido: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("archivo no es gzip válido: %v", err)
	}
	uncompressed, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatalf("no se pudo descomprimir: %v", err)
	}
	if string(uncompressed) != result.XmlFirmado {
		t.Fatalf("el archivo descomprimido no coincide con el XML firmado")
	}

	sum := sha256.Sum256(compressed)
	if want := hex.EncodeToString(sum[:]); result.HashArchivo != want {
		t.Fatalf("hashArchivo no coincide: got %q want %q", result.HashArchivo, want)
	}

	if result.Firma != "XAdES-BES" {
		t.Fatalf("unexpected firma: %q", result.Firma)
	}
}

func TestFirmarFacturaXMLSinCredencial(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")
	if _, err := svc.FirmarFacturaXML(t.Context(), "<factura/>"); err == nil {
		t.Fatal("se esperaba error al firmar sin credenciales configuradas")
	}
}

func TestBuildNotaCreditoDebitoPayload(t *testing.T) {
	fecha := time.Date(2025, 8, 15, 10, 30, 0, 0, LaPaz)
	fechaFacturaOriginal := fecha.Add(-24 * time.Hour)

	req := SolicitudNotaCreditoDebito{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadComputarizada,
		NumeroFactura:         501,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      0,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-03",
		FechaEmision:          fecha,
		Usuario:               "SUPAY",
		RazonSocialEmisor:     "EMPRESA TEST SRL",
		Municipio:             "LA PAZ",
		Direccion:             "AV. MOCK 123",
		Telefono:              ptrStr("2123456"),
		CodigoMetodoPago:      1,
		CodigoMoneda:          1,
		TipoCambio:            1,
		Leyenda:               "Ley N° 453",
		CufFacturaOriginal:    "CUF-FACTURA-ORIGINAL-ABC123",
		FechaEmisionFactura:   fechaFacturaOriginal,
		MontoTotalOriginal:    1000,
		MontoTotalDevuelto:    500,
		MontoEfectivoNota:     500,
		TipoNota:              TipoNotaCredito,
		CodigoDocumentoSector: SectorNotaCreditoDebito,
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
				Descripcion:        "Producto devuelto",
				Cantidad:           1,
				UnidadMedida:       1,
				PrecioUnitario:     500,
				SubTotal:           500,
			},
		},
	}

	factura, cuf, err := buildNotaCreditoDebito(req, goSiat.EmisionOnline)
	if err != nil {
		t.Fatalf("buildNotaCreditoDebito: %v", err)
	}
	if cuf == "" {
		t.Fatal("CUF no debe ser vacío")
	}

	xmlData, err := xml.Marshal(factura)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	xmlStr := string(xmlData)

	if !strings.Contains(xmlStr, "notaFiscalComputarizadaCreditoDebito") {
		t.Errorf("el XML debe usar la raíz notaFiscalComputarizadaCreditoDebito, got:\n%s", xmlStr[:min(len(xmlStr), 300)])
	}
	if !strings.Contains(xmlStr, "<codigoDocumentoSector>24</codigoDocumentoSector>") {
		t.Error("codigoDocumentoSector debe ser 24")
	}
	if !strings.Contains(xmlStr, "<numeroNotaCreditoDebito>501</numeroNotaCreditoDebito>") {
		t.Error("numeroNotaCreditoDebito debe ser 501")
	}
	if !strings.Contains(xmlStr, "<numeroAutorizacionCuf>CUF-FACTURA-ORIGINAL-ABC123</numeroAutorizacionCuf>") {
		t.Error("numeroAutorizacionCuf debe contener el CUF de la factura original")
	}
	if !strings.Contains(xmlStr, "<montoTotalOriginal>") {
		t.Error("montoTotalOriginal debe estar presente")
	}
	if !strings.Contains(xmlStr, "<montoTotalDevuelto>") {
		t.Error("montoTotalDevuelto debe estar presente")
	}
	if !strings.Contains(xmlStr, "<montoEfectivoCreditoDebito>") {
		t.Error("montoEfectivoCreditoDebito debe estar presente")
	}
	if !strings.Contains(xmlStr, "<codigoDetalleTransaccion>1</codigoDetalleTransaccion>") {
		t.Error("codigoDetalleTransaccion del primer ítem debe ser 1")
	}
	t.Logf("NotaCreditoDebito XML:\n%s", xmlStr)
}

func TestBuildNotaCreditoDebitoDebitoPayload(t *testing.T) {
	fecha := time.Date(2025, 8, 15, 10, 30, 0, 0, LaPaz)

	req := SolicitudNotaCreditoDebito{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Modalidad:             ModalidadComputarizada,
		NumeroFactura:         601,
		CodigoSucursal:        0,
		CodigoPuntoVenta:      0,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CodigoControl:         "CONTROL-CODE-29-CHARACTERS-03",
		FechaEmision:          fecha,
		Usuario:               "SUPAY",
		RazonSocialEmisor:     "EMPRESA TEST SRL",
		Municipio:             "LA PAZ",
		Direccion:             "AV. MOCK 123",
		CodigoMetodoPago:      1,
		CodigoMoneda:          1,
		TipoCambio:            1,
		Leyenda:               "Ley N° 453",
		CufFacturaOriginal:    "CUF-FACTURA-ORIGINAL-ND456",
		FechaEmisionFactura:   fecha.Add(-48 * time.Hour),
		MontoTotalOriginal:    800,
		MontoTotalDevuelto:    200,
		MontoEfectivoNota:     200,
		TipoNota:              TipoNotaDebito,
		CodigoDocumentoSector: SectorNotaCreditoDebito,
		CodigoTipoFactura:     1,
		Cliente: ClienteFactura{
			NombreRazonSocial:            "CLIENTE ND",
			CodigoTipoDocumentoIdentidad: 4,
			NumeroDocumento:              "9876543210",
			Complemento:                  ptrStr("LP"),
			CodigoCliente:                "C-002",
		},
		Items: []ItemFactura{
			{
				ActividadEconomica: "473000",
				CodigoProductoSin:  12345,
				CodigoProducto:     "P-002",
				Descripcion:        "Cargo adicional",
				Cantidad:           1,
				UnidadMedida:       1,
				PrecioUnitario:     200,
				SubTotal:           200,
			},
		},
	}

	_, cuf, err := buildNotaCreditoDebito(req, goSiat.EmisionOnline)
	if err != nil {
		t.Fatalf("buildNotaCreditoDebito (debito): %v", err)
	}
	if cuf == "" {
		t.Fatal("CUF no debe ser vacío para nota de débito")
	}
	t.Logf("ND CUF: %s", cuf)
}

func ptrStr(s string) *string { return &s }

// newSignedTestService construye un servicio con un certificado digital
// autofirmado (PEM) válido para poder ejercitar la firma XML (XAdES).
func newSignedTestService(t *testing.T, baseURL string) *Service {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "SUPAY TEST"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("escribir cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("escribir key: %v", err)
	}

	svc, err := NewService(Config{
		Token:          "test-token",
		Nit:            1020304050,
		CodigoSistema:  "SYS-123",
		CodigoAmbiente: AmbientePruebas,
		BaseURL:        baseURL,
		Timeout:        5 * time.Second,
		CertPemCert:    certPath,
		CertPemKey:     keyPath,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}
