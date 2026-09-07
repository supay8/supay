package siat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServiceSolicitarCUIS(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("apiKey")

		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		if !strings.Contains(payload, "SolicitudCuis") {
			t.Fatalf("expected SolicitudCuis in payload, got: %s", payload)
		}

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <cuisResponse>
      <RespuestaCuis>
        <codigo>CUIS-TEST-001</codigo>
        <fechaVigencia>2026-08-05T10:30:00-04:00</fechaVigencia>
        <transaccion>true</transaccion>
      </RespuestaCuis>
    </cuisResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	resp, err := svc.SolicitarCUIS(context.Background(), SolicitudCuis{
		CodigoAmbiente:   AmbientePruebas,
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoModalidad:  ModalidadElectronica,
		CodigoPuntoVenta: 0,
	})
	if err != nil {
		t.Fatalf("SolicitarCUIS: %v", err)
	}

	if gotAPIKey != "TokenApi test-token" {
		t.Fatalf("unexpected apiKey header: %q", gotAPIKey)
	}
	if resp.Codigo != "CUIS-TEST-001" {
		t.Fatalf("unexpected cuis code: %q", resp.Codigo)
	}
	if !resp.Transaccion {
		t.Fatalf("expected transaccion=true")
	}
	if resp.FechaVigencia.Time.IsZero() {
		t.Fatalf("expected CUIS vigencia to be parsed")
	}
}

func TestServiceSolicitarCUFD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		if !strings.Contains(payload, "SolicitudCufd") {
			t.Fatalf("expected SolicitudCufd in payload, got: %s", payload)
		}

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <cufdResponse>
      <RespuestaCufd>
        <codigo>CUFD-TEST-001</codigo>
        <codigoControl>CTRL-TEST-001</codigoControl>
        <direccion>AV. MOCK 123</direccion>
        <fechaVigencia>2026-08-05T10:30:00-04:00</fechaVigencia>
        <transaccion>true</transaccion>
      </RespuestaCufd>
    </cufdResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	resp, err := svc.SolicitarCUFD(context.Background(), SolicitudCufd{
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoModalidad:  ModalidadElectronica,
		CodigoPuntoVenta: 0,
		Cuis:             "CUIS-TEST-001",
	})
	if err != nil {
		t.Fatalf("SolicitarCUFD: %v", err)
	}

	if resp.Codigo != "CUFD-TEST-001" {
		t.Fatalf("unexpected cufd code: %q", resp.Codigo)
	}
	if resp.CodigoControl != "CTRL-TEST-001" {
		t.Fatalf("unexpected codigo control: %q", resp.CodigoControl)
	}
	if resp.Direccion != "AV. MOCK 123" {
		t.Fatalf("unexpected direccion: %q", resp.Direccion)
	}
	if !resp.Transaccion {
		t.Fatalf("expected transaccion=true")
	}
	if resp.FechaVigencia.Time.IsZero() {
		t.Fatalf("expected CUFD vigencia to be parsed")
	}
}

func TestServiceSolicitarCUISBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <cuisResponse>
      <RespuestaCuis>
        <transaccion>false</transaccion>
        <mensajesList>
          <codigo>913</codigo>
          <descripcion>Cuis invalido</descripcion>
        </mensajesList>
      </RespuestaCuis>
    </cuisResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	_, err := svc.SolicitarCUIS(context.Background(), SolicitudCuis{
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoModalidad:  ModalidadElectronica,
		CodigoPuntoVenta: 0,
	})
	if err == nil {
		t.Fatalf("expected business error, got nil")
	}
	if !strings.Contains(err.Error(), "913") {
		t.Fatalf("expected error to include SIAT code 913, got: %v", err)
	}
}

func TestServiceSolicitarCUISYaVigente(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <ns2:cuisResponse xmlns:ns2="https://siat.impuestos.gob.bo/">
      <RespuestaCuis>
        <codigo>4FF1AED1</codigo>
        <fechaVigencia>2027-08-05T18:34:19.295-04:00</fechaVigencia>
        <mensajesList>
          <codigo>980</codigo>
          <descripcion>EXISTE UN CUIS VIGENTE PARA LA SUCURSAL O PUNTO DE VENTA</descripcion>
        </mensajesList>
        <transaccion>false</transaccion>
      </RespuestaCuis>
    </ns2:cuisResponse>
  </soap:Body>
</soap:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	resp, err := svc.SolicitarCUIS(context.Background(), SolicitudCuis{
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoModalidad:  ModalidadElectronica,
		CodigoPuntoVenta: 1,
	})
	if err != nil {
		t.Fatalf("SolicitarCUIS con CUIS vigente (980): %v", err)
	}
	if resp.Codigo != "4FF1AED1" {
		t.Fatalf("unexpected cuis code: %q", resp.Codigo)
	}
	if !resp.Transaccion {
		t.Fatalf("expected transaccion=true for 980 with codigo")
	}
	if len(resp.Mensajes) != 1 || resp.Mensajes[0].Codigo != 980 {
		t.Fatalf("expected 980 message preserved, got: %+v", resp.Mensajes)
	}
	if resp.FechaVigencia.Time.IsZero() {
		t.Fatalf("expected CUIS vigencia to be parsed")
	}
}

func TestServiceSolicitarCUISVigenteSinCodigo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <cuisResponse>
      <RespuestaCuis>
        <transaccion>false</transaccion>
        <mensajesList>
          <codigo>980</codigo>
          <descripcion>EXISTE UN CUIS VIGENTE PARA LA SUCURSAL O PUNTO DE VENTA</descripcion>
        </mensajesList>
      </RespuestaCuis>
    </cuisResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	_, err := svc.SolicitarCUIS(context.Background(), SolicitudCuis{
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoModalidad:  ModalidadElectronica,
		CodigoPuntoVenta: 1,
	})
	if err == nil {
		t.Fatalf("expected error for 980 without codigo in body, got nil")
	}
	if !strings.Contains(err.Error(), "980") {
		t.Fatalf("expected error to include SIAT code 980, got: %v", err)
	}
}

func TestServiceSolicitarCUISWithWarningMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <cuisResponse>
      <RespuestaCuis>
        <codigo>CUIS-TEST-002</codigo>
        <fechaVigencia>2026-08-05T10:30:00-04:00</fechaVigencia>
        <transaccion>true</transaccion>
        <mensajesList>
          <codigo>3008</codigo>
          <descripcion>El Cuis esta a punto de caducar</descripcion>
        </mensajesList>
      </RespuestaCuis>
    </cuisResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	resp, err := svc.SolicitarCUIS(context.Background(), SolicitudCuis{
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoModalidad:  ModalidadElectronica,
		CodigoPuntoVenta: 0,
	})
	if err != nil {
		t.Fatalf("SolicitarCUIS: %v", err)
	}
	if len(resp.Mensajes) != 1 {
		t.Fatalf("expected 1 warning message, got %d", len(resp.Mensajes))
	}
	if resp.Mensajes[0].Codigo != 3008 {
		t.Fatalf("unexpected message code: %d", resp.Mensajes[0].Codigo)
	}
	if resp.Mensajes[0].Descripcion != "El Cuis esta a punto de caducar" {
		t.Fatalf("unexpected message description: %q", resp.Mensajes[0].Descripcion)
	}
}

func TestServiceValidation(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")

	if _, err := svc.SolicitarCUIS(context.Background(), SolicitudCuis{
		CodigoAmbiente: 99,
	}); err == nil {
		t.Fatalf("expected validation error for invalid ambiente")
	}

	if _, err := svc.SolicitarCUFD(context.Background(), SolicitudCufd{
		CodigoAmbiente: AmbientePruebas,
	}); err == nil {
		t.Fatalf("expected validation error for missing cuis")
	}
}

func newTestService(t *testing.T, baseURL string) *Service {
	t.Helper()
	svc, err := NewService(Config{
		Token:          "test-token",
		Nit:            1020304050,
		CodigoSistema:  "SYS-123",
		CodigoAmbiente: AmbientePruebas,
		BaseURL:        baseURL,
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}
