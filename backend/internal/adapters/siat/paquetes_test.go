package siat

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
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

	// Regresión: cafc es Nilable en el SDK y, si queda en nil, emite
	// <cafc xsi:nil="true"/> con el prefijo xsi sin declarar en el envelope del
	// request, y el SIAT rechaza el paquete con "Undeclared namespace prefix".
	if strings.Contains(gotBody, "xsi:nil") {
		t.Error("el payload SOAP no debe contener atributos xsi:nil (prefijo sin declarar en el envelope)")
	}
	if !strings.Contains(gotBody, "<cafc></cafc>") {
		t.Error("el payload SOAP debe enviar <cafc></cafc> (vacío) en lugar de xsi:nil")
	}

	assertFechaEnvioEnLaPaz(t, gotBody)
}

// TestEnviarPaqueteFacturaFirmadoPreservaXsi envía un paquete en modalidad
// electrónica (la que firma cada factura con etree/goxmldsig) y verifica que la
// re-serialización de la firma no pierda la declaración xmlns:xsi de la raíz.
// Si se perdiera, el SIAT rechazaría el XML de la factura con "Undeclared
// namespace prefix" pese a que el request fuera válido.
func TestEnviarPaqueteFacturaFirmadoPreservaXsi(t *testing.T) {
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
        <codigoRecepcion>RCV-PAQ-FIRMADO</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionPaqueteFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newSignedTestService(t, server.URL)

	paquete := paqueteDePrueba()
	paquete.Modalidad = ModalidadElectronica

	result, err := svc.EnviarPaqueteFactura(t.Context(), paquete)
	if err != nil {
		t.Fatalf("EnviarPaqueteFactura (electrónica): %v", err)
	}
	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}

	// El request tampoco puede llevar xsi:nil, independiente de la modalidad.
	if strings.Contains(gotBody, "xsi:nil") {
		t.Error("el payload SOAP firmado no debe contener xsi:nil (prefijo sin declarar en el envelope)")
	}

	facturaXML := facturaXMLDelArchivo(t, result.Archivo, "factura_1.xml")

	if !strings.Contains(facturaXML, "<ds:Signature") {
		t.Fatalf("la factura no fue firmada (sin <ds:Signature>): %s", facturaXML)
	}
	if !strings.Contains(facturaXML, `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`) {
		t.Fatalf("el XML firmado perdió la declaración xmlns:xsi:\n%s", facturaXML)
	}

	// Recorrido completo de tokens: fuerza la validación de namespaces de Go. Un
	// prefijo xsi usado sin declaración produce un error aquí.
	dec := xml.NewDecoder(strings.NewReader(facturaXML))
	for {
		if _, err := dec.Token(); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("el XML firmado no es parseable (namespace inválido): %v", err)
		}
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

func paqueteConXMLPersistido(t *testing.T) SolicitudPaqueteFactura {
	t.Helper()
	req := paqueteDePrueba()
	req.Modalidad = ModalidadElectronica
	req.Facturas = req.Facturas[:1]
	req.Facturas[0].FechaEmision = time.Date(2026, 9, 8, 11, 20, 30, 123000000, LaPaz)
	req = req.normalized()
	signer := newSignedTestService(t, "http://localhost:9999")
	emitted, err := signer.PrepararFacturaOffline(t.Context(), req.Facturas[0])
	if err != nil {
		t.Fatalf("preparar factura offline: %v", err)
	}
	req.Facturas[0].XML = emitted.Xml
	req.Facturas[0].Cuf = emitted.Cuf
	// El repositorio puede devolver UTC; debe representar el mismo instante.
	req.Facturas[0].FechaEmision = req.Facturas[0].FechaEmision.UTC()
	// El CUFD de envío puede ser distinto al CUFD histórico de la factura.
	req.Cufd = "CUFD-ACTUAL-ENVIO"
	req.CodigoControl = ""
	return req
}

func TestEnviarPaquetePreservaXMLFirmadoHistorico(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><recepcionPaqueteFacturaResponse><RespuestaServicioFacturacion><transaccion>true</transaccion><codigoEstado>908</codigoEstado><codigoRecepcion>PKG-PERSISTED</codigoRecepcion></RespuestaServicioFacturacion></recepcionPaqueteFacturaResponse></soapenv:Body></soapenv:Envelope>`)
	}))
	defer server.Close()
	req := paqueteConXMLPersistido(t)
	// Los datos de negocio ya no reconstruyen ni modifican la factura firmada.
	req.Facturas[0].Items = nil
	req.Facturas[0].MontoTotal = 999
	// No hay credencial de firma: enviar un XML firmado no debe volver a firmarlo.
	svc := newTestService(t, server.URL)
	result, err := svc.EnviarPaqueteFactura(t.Context(), req)
	if err != nil {
		t.Fatalf("enviar paquete persistido: %v", err)
	}
	if got := facturaXMLDelArchivo(t, result.Archivo, "factura_1.xml"); got != req.Facturas[0].XML {
		t.Fatal("se modificaron los bytes del XML firmado persistido")
	}
	if len(result.Cufs) != 1 || result.Cufs[0] != req.Facturas[0].Cuf {
		t.Fatalf("CUF persistido modificado: %v", result.Cufs)
	}
	if !strings.Contains(body, "<cufd>CUFD-ACTUAL-ENVIO</cufd>") {
		t.Fatal("el sobre no usa el CUFD de envío")
	}
}

func TestEnviarPaqueteRechazaIdentidadXMLInconsistente(t *testing.T) {
	base := paqueteConXMLPersistido(t)
	for name, mutate := range map[string]func(*SolicitudPaqueteFactura){
		"cuf":         func(p *SolicitudPaqueteFactura) { p.Facturas[0].Cuf = "OTRO-CUF" },
		"cufd":        func(p *SolicitudPaqueteFactura) { p.Facturas[0].Cufd = p.Cufd },
		"fecha":       func(p *SolicitudPaqueteFactura) { p.Facturas[0].FechaEmision = time.Now() },
		"numero":      func(p *SolicitudPaqueteFactura) { p.Facturas[0].NumeroFactura++ },
		"nit":         func(p *SolicitudPaqueteFactura) { p.Facturas[0].Nit = "9876543210" },
		"sucursal":    func(p *SolicitudPaqueteFactura) { p.Facturas[0].CodigoSucursal++ },
		"punto_venta": func(p *SolicitudPaqueteFactura) { p.Facturas[0].CodigoPuntoVenta++ },
		"modalidad": func(p *SolicitudPaqueteFactura) {
			p.Modalidad = ModalidadComputarizada
			p.Facturas[0].Modalidad = ModalidadComputarizada
		},
		"tipo":          func(p *SolicitudPaqueteFactura) { p.CodigoTipoFactura = 2; p.Facturas[0].CodigoTipoFactura = 2 },
		"control":       func(p *SolicitudPaqueteFactura) { p.Facturas[0].CodigoControl = "OTRO-CONTROL" },
		"xml_invalido":  func(p *SolicitudPaqueteFactura) { p.Facturas[0].XML = "<factura>" },
		"xml_ausente":   func(p *SolicitudPaqueteFactura) { p.Facturas[0].XML = "" },
		"xml_mezclados": func(p *SolicitudPaqueteFactura) { p.Facturas = append(p.Facturas, facturaDePrueba(2)) },
		"emision":       func(p *SolicitudPaqueteFactura) { p.CodigoEmision = EmisionMasiva },
	} {
		t.Run(name, func(t *testing.T) {
			req := base
			req.Facturas = append([]SolicitudFactura(nil), base.Facturas...)
			mutate(&req)
			// Validar localmente prueba que la solicitud se rechaza antes de HTTP.
			if err := req.normalized().validate(); err == nil {
				t.Fatal("se aceptó un documento fiscal inconsistente")
			}
		})
	}
}

func TestValidacionRecepcionNoRequiereCodigoControl(t *testing.T) {
	pkg := paqueteDePrueba()
	pkg.CodigoControl = ""
	if err := pkg.validateBase(); err != nil {
		t.Fatalf("validación paquete: %v", err)
	}
	bulk := SolicitudMasivaFactura{Modalidad: pkg.Modalidad, Cuis: pkg.Cuis, Cufd: pkg.Cufd}
	if err := bulk.validateBase(); err != nil {
		t.Fatalf("validación masiva: %v", err)
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

// assertFechaEnvioEnLaPaz verifica que el payload SOAP transporte fechaEnvio con
// la hora de pared de Bolivia (UTC-4). El SIAT interpreta esa cadena sin zona
// como hora local; si se enviara hora UTC la diferencia sería de ~4 horas y el
// SIAT rechazaría con "EL PARAMETRO FECHA DE ENVIO ES INVALIDO".
func assertFechaEnvioEnLaPaz(t *testing.T, body string) {
	t.Helper()
	re := regexp.MustCompile(`<fechaEnvio>(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3})</fechaEnvio>`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("fechaEnvio no encontrado o con formato incorrecto en el payload")
	}
	sent, err := time.ParseInLocation("2006-01-02T15:04:05.000", m[1], LaPaz)
	if err != nil {
		t.Fatalf("fechaEnvio no parseable: %v", err)
	}
	if d := time.Since(sent); d < -5*time.Minute || d > 5*time.Minute {
		t.Fatalf("fechaEnvio no corresponde a la hora local de La Paz (UTC-4): %q (diferencia %s)", m[1], d.Round(time.Second))
	}
}

// facturaXMLDelArchivo extrae una entrada del TAR.GZ (base64) que contiene el
// paquete de facturas enviado al SIAT y devuelve su contenido como texto.
func facturaXMLDelArchivo(t *testing.T, archivo, name string) string {
	t.Helper()

	compressed, err := base64.StdEncoding.DecodeString(archivo)
	if err != nil {
		t.Fatalf("archivo no es base64 válido: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("archivo no es gzip válido: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("no se encontró %q en el tar", name)
		}
		if err != nil {
			t.Fatalf("error leyendo tar: %v", err)
		}
		if hdr.Name == name {
			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, tr); err != nil {
				t.Fatalf("error leyendo %q: %v", name, err)
			}
			return buf.String()
		}
	}
}
