package siat

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestEnviarComprasPayload(t *testing.T) {
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
    <recepcionPaqueteComprasResponse>
      <RespuestaServicioFacturacion>
        <transaccion>true</transaccion>
        <codigoEstado>904</codigoEstado>
        <codigoRecepcion>RCV-ETAPA-XI</codigoRecepcion>
      </RespuestaServicioFacturacion>
    </recepcionPaqueteComprasResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	req := SolicitudCompras{
		Descripcion:      "RECEPCIÓN - INTERNO/ACTIVIDADES GRAVADAS",
		TipoCompra:       1,
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoPuntoVenta: 0,
		Cuis:             "CUIS-TEST-001",
		Cufd:             "CUFD-TEST-001",
		Archivo:          "H4sIAAAAAAACA...",
		HashArchivo:      "8f4b2b3a9c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f",
		CantidadFacturas: 2,
		Gestion:          2026,
		Periodo:          8,
		FechaEnvio:       time.Now(),
	}

	result, err := svc.EnviarCompras(t.Context(), req)
	if err != nil {
		t.Fatalf("EnviarCompras: %v", err)
	}

	if gotAPIKey != "TokenApi test-token" {
		t.Fatalf("unexpected apiKey header: %q", gotAPIKey)
	}
	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}
	if result.CodigoEstado != 904 {
		t.Fatalf("se esperaba codigoEstado 904, got %d", result.CodigoEstado)
	}
	if result.CodigoRecepcion != "RCV-ETAPA-XI" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}

	for _, want := range []string{
		"recepcionPaqueteCompras",
		"SolicitudRecepcionCompras",
		"<codigoAmbiente>2</codigoAmbiente>",
		"<codigoSistema>SYS-123</codigoSistema>",
		"<codigoSucursal>0</codigoSucursal>",
		"<codigoPuntoVenta>0</codigoPuntoVenta>",
		"<cuis>CUIS-TEST-001</cuis>",
		"<cufd>CUFD-TEST-001</cufd>",
		"<nit>1020304050</nit>",
		"<archivo>H4sIAAAAAAACA...</archivo>",
		"<hashArchivo>" + req.HashArchivo + "</hashArchivo>",
		"<cantidadFacturas>2</cantidadFacturas>",
		"<gestion>2026</gestion>",
		"<periodo>8</periodo>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q", want)
		}
	}

	fechaEnvioRe := regexp.MustCompile(`<fechaEnvio>(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3})</fechaEnvio>`)
	if m := fechaEnvioRe.FindStringSubmatch(gotBody); m == nil {
		t.Fatal("fechaEnvio no encontrado o con formato incorrecto en el payload")
	}

	// descripcion y tipoCompra son informativos del lote: la solicitud SOAP no
	// los transporta (viven dentro de los registros del archivo).
	for _, notWant := range []string{"descripcion", "tipoCompra"} {
		if strings.Contains(gotBody, notWant) {
			t.Errorf("el payload SOAP no debe contener %q", notWant)
		}
	}
}

func TestEnviarComprasValidaLimite(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")

	req := validSolicitudCompras()
	req.CantidadFacturas = 10
	if _, err := svc.EnviarCompras(t.Context(), req); err == nil {
		t.Fatal("se esperaba error: cantidadFacturas debe ser menor a 10")
	}
}

func TestEnviarComprasValidaPeriodo(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")

	req := validSolicitudCompras()
	req.Periodo = 13
	if _, err := svc.EnviarCompras(t.Context(), req); err == nil {
		t.Fatal("se esperaba error: periodo inválido")
	}

	req.Periodo = 0
	if _, err := svc.EnviarCompras(t.Context(), req); err == nil {
		t.Fatal("se esperaba error: periodo inválido")
	}
}

func TestEnviarComprasValidaPuntoVenta(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")

	req := validSolicitudCompras()
	req.CodigoPuntoVenta = 1
	if _, err := svc.EnviarCompras(t.Context(), req); err == nil {
		t.Fatal("se esperaba error: codigoPuntoVenta no aplica (debe ser 0)")
	}
}

func validSolicitudCompras() SolicitudCompras {
	return SolicitudCompras{
		Descripcion:      "RECEPCIÓN - INTERNO/ACTIVIDADES GRAVADAS",
		TipoCompra:       1,
		CodigoAmbiente:   AmbientePruebas,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoPuntoVenta: 0,
		Cuis:             "CUIS-TEST-001",
		Cufd:             "CUFD-TEST-001",
		Archivo:          "H4sIAAAAAAACA...",
		HashArchivo:      "8f4b2b3a9c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f",
		CantidadFacturas: 2,
		Gestion:          2026,
		Periodo:          8,
		FechaEnvio:       time.Now(),
	}
}
