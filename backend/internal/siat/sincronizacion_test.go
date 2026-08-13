package siat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceSincronizarTipoPuntoVenta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		if !strings.Contains(payload, "sincronizarParametricaTipoPuntoVenta") {
			t.Fatalf("expected sincronizarParametricaTipoPuntoVenta in payload, got: %s", payload)
		}
		if !strings.Contains(payload, ">CUIS-TEST-001<") {
			t.Fatalf("expected CUIS in payload, got: %s", payload)
		}

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <sincronizarParametricaTipoPuntoVentaResponse>
      <RespuestaListaParametricas>
        <transaccion>true</transaccion>
        <listaCodigos>
          <codigoClasificador>1</codigoClasificador>
          <descripcion>VENTA POR MENOR</descripcion>
        </listaCodigos>
        <listaCodigos>
          <codigoClasificador>2</codigoClasificador>
          <descripcion>VENTA POR MAYOR</descripcion>
        </listaCodigos>
      </RespuestaListaParametricas>
    </sincronizarParametricaTipoPuntoVentaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	resp, err := svc.Sincronizar(context.Background(), SolicitudSincronizacion{
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoPuntoVenta: 3,
		Cuis:             "CUIS-TEST-001",
	}, OpTipoPuntoVenta)
	if err != nil {
		t.Fatalf("Sincronizar tipoPuntoVenta: %v", err)
	}

	if !resp.Transaccion {
		t.Fatalf("expected transaccion=true")
	}
	if len(resp.Codigos) != 2 {
		t.Fatalf("expected 2 codigos, got %d", len(resp.Codigos))
	}
	if resp.Codigos[0].CodigoClasificador != 1 || resp.Codigos[0].Descripcion != "VENTA POR MENOR" {
		t.Fatalf("unexpected first codigo: %+v", resp.Codigos[0])
	}
	if resp.Codigos[1].CodigoClasificador != 2 || resp.Codigos[1].Descripcion != "VENTA POR MAYOR" {
		t.Fatalf("unexpected second codigo: %+v", resp.Codigos[1])
	}
}

func TestServiceSincronizarActividades(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		if !strings.Contains(payload, "sincronizarActividades") {
			t.Fatalf("expected sincronizarActividades in payload, got: %s", payload)
		}

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <sincronizarActividadesResponse>
      <RespuestaListaActividades>
        <transaccion>true</transaccion>
        <listaActividades>
          <codigoCaeb>12345</codigoCaeb>
          <descripcion>RESTAURANTES</descripcion>
          <tipoActividad>ECONOMICA</tipoActividad>
        </listaActividades>
        <listaActividades>
          <codigoCaeb>67890</codigoCaeb>
          <descripcion>HOTELES</descripcion>
          <tipoActividad>ECONOMICA</tipoActividad>
        </listaActividades>
      </RespuestaListaActividades>
    </sincronizarActividadesResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	resp, err := svc.Sincronizar(context.Background(), SolicitudSincronizacion{
		CodigoAmbiente: AmbientePruebas,
		CodigoSistema:  "SYS-123",
		Nit:            "1020304050",
		Cuis:           "CUIS-TEST-001",
	}, OpActividades)
	if err != nil {
		t.Fatalf("Sincronizar actividades: %v", err)
	}

	if len(resp.Codigos) != 2 {
		t.Fatalf("expected 2 actividades, got %d", len(resp.Codigos))
	}
	if resp.Codigos[0].CodigoClasificador != 12345 || resp.Codigos[0].Descripcion != "RESTAURANTES" {
		t.Fatalf("unexpected first actividad: %+v", resp.Codigos[0])
	}
}

func TestServiceSincronizarFechaHora(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <sincronizarFechaHoraResponse>
      <RespuestaFechaHora>
        <transaccion>true</transaccion>
        <fechaHora>2026-08-13T15:04:05.000</fechaHora>
      </RespuestaFechaHora>
    </sincronizarFechaHoraResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	resp, err := svc.Sincronizar(context.Background(), SolicitudSincronizacion{
		CodigoAmbiente: AmbientePruebas,
		CodigoSistema:  "SYS-123",
		Nit:            "1020304050",
		Cuis:           "CUIS-TEST-001",
	}, OpFechaHora)
	if err != nil {
		t.Fatalf("Sincronizar fechaHora: %v", err)
	}

	if !resp.Transaccion {
		t.Fatalf("expected transaccion=true")
	}
	if resp.FechaHora.IsZero() {
		t.Fatalf("expected fechaHora to be parsed")
	}
	if resp.FechaHora.Hour() != 15 {
		t.Fatalf("unexpected hora: %v", resp.FechaHora)
	}
}

func TestServiceSincronizarRechazado(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <sincronizarParametricaTipoPuntoVentaResponse>
      <RespuestaListaParametricas>
        <transaccion>false</transaccion>
      </RespuestaListaParametricas>
    </sincronizarParametricaTipoPuntoVentaResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	_, err := svc.Sincronizar(context.Background(), SolicitudSincronizacion{
		CodigoAmbiente: AmbientePruebas,
		CodigoSistema:  "SYS-123",
		Nit:            "1020304050",
		Cuis:           "CUIS-TEST-001",
	}, OpTipoPuntoVenta)
	if err == nil {
		t.Fatalf("expected rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "rechazada") {
		t.Fatalf("expected rejection error message, got: %v", err)
	}
}

func TestServiceSincronizarValidation(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")

	req := SolicitudSincronizacion{
		CodigoAmbiente: AmbientePruebas,
		CodigoSistema:  "SYS-123",
		Nit:            "1020304050",
		Cuis:           "",
	}
	if _, err := svc.Sincronizar(context.Background(), req, OpTipoMoneda); err == nil {
		t.Fatalf("expected validation error for empty cuis")
	}

	req.Cuis = "CUIS-TEST-001"
	if _, err := svc.Sincronizar(context.Background(), req, SincronizacionOp("operacion-inexistente")); err == nil {
		t.Fatalf("expected error for unknown operation")
	}
}
