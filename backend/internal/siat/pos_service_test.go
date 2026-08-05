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

func mockPuntoVentaServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		switch {
		case strings.Contains(body, "consultaPuntoVenta"):
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
				  <soapenv:Body>
				    <consultaPuntoVentaResponse>
				      <RespuestaConsultaPuntoVenta>
				        <transaccion>true</transaccion>
				        <listaPuntosVenta>
				          <codigoTipoPuntoVenta>1</codigoTipoPuntoVenta>
				          <nombrePuntoVenta>POS FISICO</nombrePuntoVenta>
				          <descripcion>Punto de venta fisico</descripcion>
				          <estado>ABIERTO</estado>
				          <fechaRegistro>2026-01-01T00:00:00</fechaRegistro>
				        </listaPuntosVenta>
				      </RespuestaConsultaPuntoVenta>
				    </consultaPuntoVentaResponse>
				  </soapenv:Body>
				</soapenv:Envelope>`))
		case strings.Contains(body, "cierrePuntoVenta"):
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
				  <soapenv:Body>
				    <cierrePuntoVentaResponse>
				      <RespuestaCierrePuntoVenta>
				        <codigoPuntoVenta>123</codigoPuntoVenta>
				        <transaccion>true</transaccion>
				      </RespuestaCierrePuntoVenta>
				    </cierrePuntoVentaResponse>
				  </soapenv:Body>
				</soapenv:Envelope>`))
		case strings.Contains(body, "registroPuntoVenta"):
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
				  <soapenv:Body>
				    <registroPuntoVentaResponse>
				      <RespuestaRegistroPuntoVenta>
				        <mensajesList>
				          <codigo>947</codigo>
				          <descripcion>EL PARAMETRO TIPO DE PUNTO DE VENTA ES INVALIDO</descripcion>
				        </mensajesList>
				        <transaccion>false</transaccion>
				      </RespuestaRegistroPuntoVenta>
				    </registroPuntoVentaResponse>
				  </soapenv:Body>
				</soapenv:Envelope>`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("unknown request"))
		}
	}))
}

func newPuntoVentaTestService(t *testing.T, srvURL string) *PuntoVentaService {
	t.Helper()
	cfg := Config{EndpointURL: srvURL, WSDLURL: srvURL + "?wsdl", Timeout: 5 * time.Second, Headers: map[string]string{"apikey": "TokenApi FAKE"}}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return NewPuntoVentaService(client)
}

func TestPuntoVentaService_Consultar(t *testing.T) {
	srv := mockPuntoVentaServer(t)
	defer srv.Close()

	svc := newPuntoVentaTestService(t, srv.URL)
	req := ConsultaPuntoVentaRequest{
		CodigoAmbiente: 2,
		CodigoSistema:  "CODE-TEST",
		CodigoSucursal: 1,
		Cuis:           "CUIS-X",
		Nit:            "9971522011",
	}
	ctx := context.Background()
	resp, err := svc.Consultar(ctx, req)
	if err != nil {
		t.Fatalf("consultar: %v", err)
	}
	if !resp.Transaccion {
		t.Fatal("expected transaccion=true")
	}
	if len(resp.ListaPuntosVenta) != 1 {
		t.Fatalf("expected 1 punto de venta, got %d", len(resp.ListaPuntosVenta))
	}
	if resp.ListaPuntosVenta[0].NombrePuntoVenta != "POS FISICO" {
		t.Fatalf("unexpected nombre: %s", resp.ListaPuntosVenta[0].NombrePuntoVenta)
	}
	if !strings.Contains(resp.RawRequest, "consultaPuntoVenta") {
		t.Fatal("expected raw request captured")
	}
}

func TestPuntoVentaService_Registrar(t *testing.T) {
	srv := mockPuntoVentaServer(t)
	defer srv.Close()

	svc := newPuntoVentaTestService(t, srv.URL)
	req := RegistroPuntoVentaRequest{
		CodigoAmbiente:       2,
		CodigoModalidad:      1,
		CodigoSistema:        "CODE-TEST",
		CodigoSucursal:       0,
		CodigoTipoPuntoVenta: 1,
		Cuis:                 "CUIS-X",
		Nit:                  "9971522011",
		NombrePuntoVenta:     "Caja Principal",
	}
	ctx := context.Background()
	resp, err := svc.Registrar(ctx, req)
	if err != nil {
		t.Fatalf("registrar: %v", err)
	}
	if resp.Transaccion {
		t.Fatal("expected transaccion=false for rejection response")
	}
	if len(resp.Mensajes) != 1 {
		t.Fatalf("expected 1 mensaje parsed, got %d", len(resp.Mensajes))
	}
	if resp.Mensajes[0].Codigo != "947" {
		t.Fatalf("expected mensaje codigo 947, got %q", resp.Mensajes[0].Codigo)
	}
	if resp.Mensajes[0].Descripcion != "EL PARAMETRO TIPO DE PUNTO DE VENTA ES INVALIDO" {
		t.Fatalf("unexpected descripcion: %q", resp.Mensajes[0].Descripcion)
	}
}

func TestPuntoVentaService_Cerrar(t *testing.T) {
	srv := mockPuntoVentaServer(t)
	defer srv.Close()

	svc := newPuntoVentaTestService(t, srv.URL)
	req := CierrePuntoVentaRequest{
		CodigoAmbiente:   2,
		CodigoSistema:    "CODE-TEST",
		CodigoSucursal:   1,
		Cuis:             "CUIS-X",
		Nit:              "9971522011",
		CodigoPuntoVenta: 123,
	}
	ctx := context.Background()
	resp, err := svc.Cerrar(ctx, req)
	if err != nil {
		t.Fatalf("cerrar: %v", err)
	}
	if !resp.Transaccion {
		t.Fatal("expected transaccion=true")
	}
	if resp.CodigoPuntoVenta != 123 {
		t.Fatalf("expected code 123, got %d", resp.CodigoPuntoVenta)
	}
}
