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

func TestCuisAndCufdIntegrationWithSOAPMock(t *testing.T) {
	t.Helper()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if got := r.Header.Get("apikey"); got != "TokenApi test-token" {
			t.Fatalf("unexpected apikey header: %q", got)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/xml") {
			t.Fatalf("unexpected content type: %q", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		payload := string(body)
		w.Header().Set("Content-Type", soapContentType)

		switch {
		case strings.Contains(payload, "SolicitudCuis"):
			_, _ = w.Write([]byte(mockCuisSOAPResponse()))
		case strings.Contains(payload, "SolicitudCufd"):
			_, _ = w.Write([]byte(mockCufdSOAPResponse()))
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		WSDLURL:     server.URL + "?wsdl",
		EndpointURL: server.URL,
		Timeout:     5 * time.Second,
		Headers: map[string]string{
			"apikey": "TokenApi test-token",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	cuisService := NewCuisService(client)
	cufdService := NewCufdService(client)

	cuisResp, err := cuisService.SolicitarCUIS(context.Background(), SolicitudCuis{
		CodigoAmbiente:   2,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		CodigoModalidad:  1,
		CodigoPuntoVenta: 0,
	})
	if err != nil {
		t.Fatalf("SolicitarCUIS: %v", err)
	}
	if cuisResp.Codigo != "CUIS-TEST-001" {
		t.Fatalf("unexpected cuis code: %q", cuisResp.Codigo)
	}
	if !cuisResp.Transaccion {
		t.Fatalf("expected transaccion=true for cuis")
	}
	if cuisResp.FechaVigencia.Time.IsZero() {
		t.Fatalf("expected CUIS vigencia to be parsed")
	}

	cufdResp, err := cufdService.SolicitarCUFD(context.Background(), SolicitudCufd{
		CodigoAmbiente:   2,
		CodigoSistema:    "SYS-123",
		Nit:              "1020304050",
		CodigoSucursal:   0,
		Cuis:             "CUIS-TEST-001",
		CodigoModalidad:  1,
		CodigoPuntoVenta: 0,
	})
	if err != nil {
		t.Fatalf("SolicitarCUFD: %v", err)
	}
	if cufdResp.Codigo != "CUFD-TEST-001" {
		t.Fatalf("unexpected cufd code: %q", cufdResp.Codigo)
	}
	if cufdResp.CodigoControl != "CTRL-TEST-001" {
		t.Fatalf("unexpected codigo control: %q", cufdResp.CodigoControl)
	}
	if cufdResp.Direccion != "AV. MOCK 123" {
		t.Fatalf("unexpected direccion: %q", cufdResp.Direccion)
	}
	if !cufdResp.Transaccion {
		t.Fatalf("expected transaccion=true for cufd")
	}
	if cufdResp.FechaVigencia.Time.IsZero() {
		t.Fatalf("expected CUFD vigencia to be parsed")
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 SOAP requests, got %d", requestCount)
	}
}

func mockCuisSOAPResponse() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
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
</soapenv:Envelope>`
}

func mockCufdSOAPResponse() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
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
</soapenv:Envelope>`
}

func TestSincronizarParametricaTipoPuntoVenta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "SolicitudSincronizacion") {
			t.Fatalf("expected SolicitudSincronizacion in payload")
		}
		w.Header().Set("Content-Type", soapContentType)
		resp := `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <sincronizarParametricaTipoPuntoVentaResponse>
      <RespuestaListaParametricas>
        <transaccion>true</transaccion>
        <listaCodigos>
          <codigoClasificador>1</codigoClasificador>
          <descripcion>Punto de Venta Fisico</descripcion>
        </listaCodigos>
        <listaCodigos>
          <codigoClasificador>2</codigoClasificador>
          <descripcion>Punto de Venta Movil</descripcion>
        </listaCodigos>
      </RespuestaListaParametricas>
    </sincronizarParametricaTipoPuntoVentaResponse>
  </soapenv:Body>
</soapenv:Envelope>`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		WSDLURL:     server.URL + "?wsdl",
		EndpointURL: server.URL,
		Timeout:     5 * time.Second,
		Headers:     map[string]string{"apikey": "TokenApi test-token"},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	svc := NewSincronizacionService(client)
	resp, err := svc.SincronizarTipoPuntoVenta(context.Background(), SolicitudSincronizacion{
		CodigoAmbiente: 2,
		CodigoSistema:  "SYS-123",
		CodigoSucursal: 0,
		Cuis:           "CUIS-TEST-001",
		Nit:            "1020304050",
	})
	if err != nil {
		t.Fatalf("SincronizarTipoPuntoVenta: %v", err)
	}
	if !resp.Transaccion {
		t.Fatalf("expected transaccion=true")
	}
	if len(resp.ListaCodigos) != 2 {
		t.Fatalf("expected 2 catalog codes, got %d", len(resp.ListaCodigos))
	}
	if resp.ListaCodigos[0].CodigoClasificador != 1 {
		t.Fatalf("expected first clasificador 1, got %d", resp.ListaCodigos[0].CodigoClasificador)
	}
	if resp.ListaCodigos[1].Descripcion != "Punto de Venta Movil" {
		t.Fatalf("unexpected description: %q", resp.ListaCodigos[1].Descripcion)
	}
}

func TestRegistroPuntoVentaParsesOfficialResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "SolicitudRegistroPuntoVenta") {
			t.Fatalf("expected SolicitudRegistroPuntoVenta in payload")
		}
		w.Header().Set("Content-Type", soapContentType)
		resp := `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <registroPuntoVentaResponse>
      <RespuestaRegistroPuntoVenta>
        <codigoPuntoVenta>42</codigoPuntoVenta>
        <transaccion>true</transaccion>
      </RespuestaRegistroPuntoVenta>
    </registroPuntoVentaResponse>
  </soapenv:Body>
</soapenv:Envelope>`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		WSDLURL:     server.URL + "?wsdl",
		EndpointURL: server.URL,
		Timeout:     5 * time.Second,
		Headers:     map[string]string{"apikey": "TokenApi test-token"},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	svc := NewPuntoVentaService(client)
	resp, err := svc.Registrar(context.Background(), RegistroPuntoVentaRequest{
		CodigoAmbiente:       2,
		CodigoModalidad:      1,
		Nit:                  "1020304050",
		CodigoSistema:        "SYS-123",
		CodigoSucursal:       0,
		NombrePuntoVenta:     "POS-TEST",
		CodigoTipoPuntoVenta: 1,
		Cuis:                 "CUIS-TEST-001",
		Descripcion:          "desc",
	})
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}
	if resp.CodigoPuntoVenta != 42 {
		t.Fatalf("expected official code 42, got %d", resp.CodigoPuntoVenta)
	}
	if !resp.Transaccion {
		t.Fatalf("expected transaccion=true")
	}
}
