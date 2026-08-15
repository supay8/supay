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

func TestRegistrarEventoSignificativo(t *testing.T) {
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
    <registroEventoSignificativoResponse>
      <RespuestaListaEventos>
        <codigoRecepcionEventoSignificativo>998877</codigoRecepcionEventoSignificativo>
        <transaccion>true</transaccion>
      </RespuestaListaEventos>
    </registroEventoSignificativoResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	inicio := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	fin := time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC)

	result, err := svc.RegistrarEventoSignificativo(context.Background(), SolicitudEventoSignificativo{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		CodigoSucursal:        0,
		CodigoPuntoVenta:      1,
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CufdEvento:            "CUFD-EVENTO-001",
		CodigoMotivoEvento:    MotivoCorteInternet,
		Descripcion:           "Corte del servicio de internet",
		FechaHoraInicioEvento: inicio,
		FechaHoraFinEvento:    fin,
	})
	if err != nil {
		t.Fatalf("RegistrarEventoSignificativo: %v", err)
	}

	if gotAPIKey != "TokenApi test-token" {
		t.Fatalf("unexpected apiKey header: %q", gotAPIKey)
	}
	if result.CodigoRecepcion != "998877" {
		t.Fatalf("unexpected codigoRecepcion: %q", result.CodigoRecepcion)
	}
	if !result.Transaccion {
		t.Fatal("se esperaba transaccion=true")
	}

	for _, want := range []string{
		"registroEventoSignificativo",
		"SolicitudEventoSignificativo",
		"<codigoMotivoEvento>1</codigoMotivoEvento>",
		"<codigoPuntoVenta>1</codigoPuntoVenta>",
		"<codigoSucursal>0</codigoSucursal>",
		"<cuis>CUIS-TEST-001</cuis>",
		// Los CUFD se limpian de caracteres no alfanuméricos antes de enviarse.
		"<cufd>CUFDTEST001</cufd>",
		"<cufdEvento>CUFDEVENTO001</cufdEvento>",
		"<descripcion>Corte del servicio de internet</descripcion>",
		"<nit>1020304050</nit>",
		"<codigoSistema>SYS-123</codigoSistema>",
		"<codigoAmbiente>2</codigoAmbiente>",
		// Las fechas del evento deben viajar en hora local de Bolivia (UTC-4):
		// 09:00Z -> 05:00-04:00 y 10:30Z -> 06:30-04:00.
		"<fechaHoraInicioEvento>2026-08-14T05:00:00-04:00</fechaHoraInicioEvento>",
		"<fechaHoraFinEvento>2026-08-14T06:30:00-04:00</fechaHoraFinEvento>",
	} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("el payload SOAP no contiene %q", want)
		}
	}

	for _, forbidden := range []string{
		"<fechaHoraInicioEvento>2026-08-14T09:00:00Z</fechaHoraInicioEvento>",
		"<fechaHoraFinEvento>2026-08-14T10:30:00Z</fechaHoraFinEvento>",
	} {
		if strings.Contains(gotBody, forbidden) {
			t.Errorf("el payload SOAP no debe enviar la fecha en UTC: %q", forbidden)
		}
	}
}

func TestRegistrarEventoSignificativoRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <registroEventoSignificativoResponse>
      <RespuestaListaEventos>
        <transaccion>false</transaccion>
        <mensajesList>
          <codigo>981</codigo>
          <descripcion>Rango de fechas del evento significativo invalido</descripcion>
        </mensajesList>
      </RespuestaListaEventos>
    </registroEventoSignificativoResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)

	_, err := svc.RegistrarEventoSignificativo(context.Background(), SolicitudEventoSignificativo{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS-123",
		Nit:                   "1020304050",
		Cuis:                  "CUIS-TEST-001",
		Cufd:                  "CUFD-TEST-001",
		CufdEvento:            "CUFD-EVENTO-001",
		CodigoMotivoEvento:    MotivoCorteInternet,
		Descripcion:           "Corte del servicio de internet",
		FechaHoraInicioEvento: time.Now(),
		FechaHoraFinEvento:    time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("se esperaba error de rechazo del SIAT")
	}
	if !strings.Contains(err.Error(), "981") {
		t.Fatalf("el error debe incluir el código SIAT 981, got: %v", err)
	}
}

func TestRegistrarEventoSignificativoValidation(t *testing.T) {
	svc := newTestService(t, "http://localhost:9999")

	cases := []struct {
		name string
		req  SolicitudEventoSignificativo
	}{
		{
			name: "ambiente inválido",
			req:  SolicitudEventoSignificativo{CodigoAmbiente: 99},
		},
		{
			name: "sin cuis",
			req: SolicitudEventoSignificativo{
				CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS", Nit: "1",
				Cufd: "CUFD", CufdEvento: "CUFD-EVENTO", CodigoMotivoEvento: 1,
				Descripcion: "d", FechaHoraInicioEvento: time.Now(), FechaHoraFinEvento: time.Now().Add(time.Hour),
			},
		},
		{
			name: "sin cufdEvento",
			req: SolicitudEventoSignificativo{
				CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS", Nit: "1",
				Cuis: "CUIS", Cufd: "CUFD", CodigoMotivoEvento: 1,
				Descripcion: "d", FechaHoraInicioEvento: time.Now(), FechaHoraFinEvento: time.Now().Add(time.Hour),
			},
		},
		{
			name: "sin motivo",
			req: SolicitudEventoSignificativo{
				CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS", Nit: "1",
				Cuis: "CUIS", Cufd: "CUFD", CufdEvento: "CUFD-EVENTO",
				Descripcion: "d", FechaHoraInicioEvento: time.Now(), FechaHoraFinEvento: time.Now().Add(time.Hour),
			},
		},
		{
			name: "sin descripcion",
			req: SolicitudEventoSignificativo{
				CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS", Nit: "1",
				Cuis: "CUIS", Cufd: "CUFD", CufdEvento: "CUFD-EVENTO", CodigoMotivoEvento: 1,
				FechaHoraInicioEvento: time.Now(), FechaHoraFinEvento: time.Now().Add(time.Hour),
			},
		},
		{
			name: "fin antes de inicio",
			req: SolicitudEventoSignificativo{
				CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS", Nit: "1",
				Cuis: "CUIS", Cufd: "CUFD", CufdEvento: "CUFD-EVENTO", CodigoMotivoEvento: 1,
				Descripcion: "d", FechaHoraInicioEvento: time.Now(), FechaHoraFinEvento: time.Now().Add(-time.Hour),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.RegistrarEventoSignificativo(context.Background(), tc.req); err == nil {
				t.Fatalf("se esperaba error de validación para %s", tc.name)
			}
		})
	}
}
