package siat

import (
	"encoding/json"
	"errors"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestTypedSerializationRejectsMalformedPayloadBeforeSOAP(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.Error(w, "unexpected SOAP", 500) }))
	defer server.Close()
	svc, err := NewService(Config{Token: "test", Nit: 1020304050, CodigoSistema: "SYS-TEST", CodigoAmbiente: AmbientePruebas, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewFiscalAdapter(svc)
	for _, data := range []string{`{"unknown_field":1}`, `{"numero_tarjeta":1.5}`, `{"monto_gift_card":{}}`} {
		t.Run(data, func(t *testing.T) {
			req := baseItemConstruyeFactura(t)
			req.DatosSector = []byte(data)
			encoded, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			var doc ports.FiscalDocument
			if err := json.Unmarshal(encoded, &doc); err != nil {
				t.Fatal(err)
			}
			_, err = adapter.PrepareEmission(t.Context(), doc)
			var validation *domain.BadRequestError
			if !errors.As(err, &validation) {
				t.Fatalf("expected HTTP400 validation error, got %T %v", err, err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid payload triggered %d SOAP requests", calls.Load())
	}
}
