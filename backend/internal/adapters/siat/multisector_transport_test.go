package siat

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Real SDK transport and signing, with a local SOAP receiver. This verifies
// integration, not tax acceptance or homologation against the pilot service.
func TestAllSDKProfilesSendThroughTheirSOAPService(t *testing.T) {
	type captured struct{ path, body string }
	requests := make(chan captured, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- captured{r.URL.Path, string(body)}
		method := "recepcionFactura"
		if strings.Contains(string(body), "recepcionDocumentoAjuste") {
			method = "recepcionDocumentoAjuste"
		}
		if strings.Contains(string(body), "recepcionMasivaFactura") {
			method = "recepcionMasivaFactura"
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<Envelope><Body><%sResponse><RespuestaServicioFacturacion><transaccion>true</transaccion><codigoEstado>908</codigoEstado><codigoRecepcion>TEST-OK</codigoRecepcion></RespuestaServicioFacturacion></%sResponse></Body></Envelope>`, method, method)
	}))
	defer server.Close()
	service := newSignedTestService(t, server.URL)
	for _, profile := range PerfilesSector() {
		if !profile.HasBuilder() {
			continue
		}
		for _, mode := range []int{ModalidadElectronica, ModalidadComputarizada} {
			if profile.ValidarModalidad(mode) != nil {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s/%d", profile.Codigo, profile.Layout, mode), func(t *testing.T) {
				req := baseItemConstruyeFactura(t)
				req.CodigoSistema = "SYS-123"
				req.CodigoDocumentoSector, req.Layout, req.Modalidad = profile.Codigo, profile.Layout, mode
				req.DatosSector = jsonRaw(datosDeEjemplo(profile))
				req.Items[0].DatosSector = datosDetalleDeEjemplo(profile)
				if profile.Codigo == 30 {
					req.Items = nil
					_, err := service.EnviarMasivaFacturas(t.Context(), SolicitudMasivaFactura{
						Modalidad: mode, Cuis: req.Cuis, Cufd: req.Cufd, CodigoControl: req.CodigoControl,
						CodigoDocumentoSector: 30, Facturas: []SolicitudFactura{req},
					})
					if err != nil {
						t.Fatal(err)
					}
				} else {
					result, err := service.EmitirFactura(t.Context(), req)
					if err != nil {
						t.Fatal(err)
					}
					if !result.Transaccion || result.CodigoRecepcion != "TEST-OK" || result.XmlHash == "" {
						t.Fatalf("bad SOAP result: %+v", result)
					}
					if mode == ModalidadElectronica && !strings.Contains(result.Xml, "Signature") {
						t.Fatal("electronic XML is unsigned")
					}
					if mode == ModalidadComputarizada && strings.Contains(result.Xml, "Signature") {
						t.Fatal("unexpected computerized signature")
					}
				}
				var got captured
				select {
				case got = <-requests:
				default:
					t.Fatal("no SOAP request")
				}
				if got.path != "/"+expectedSOAPService(profile.Codigo, mode) {
					t.Fatalf("wrong endpoint: %s", got.path)
				}
				for _, expected := range []string{
					fmt.Sprintf("<codigoDocumentoSector>%d</codigoDocumentoSector>", profile.Codigo),
					fmt.Sprintf("<tipoFacturaDocumento>%d</tipoFacturaDocumento>", profile.TipoFacturaDocumento),
					fmt.Sprintf("<codigoModalidad>%d</codigoModalidad>", mode),
				} {
					if !strings.Contains(got.body, expected) {
						t.Errorf("SOAP missing %s", expected)
					}
				}
			})
		}
	}
}

func expectedSOAPService(sector, mode int) string {
	suffix := "Electronica"
	if mode == ModalidadComputarizada {
		suffix = "Computarizada"
	}
	switch sector {
	case 1, 35, 41:
		suffix = "CompraVenta"
	case 13, 40:
		suffix = "ServicioBasico"
	case 22, 49:
		suffix = "Telecomunicaciones"
	case 15:
		suffix = "EntidadFinanciera"
	case 24, 29, 47, 48:
		suffix = "DocumentoAjuste"
	case 30:
		suffix = "BoletoAereo"
	}
	return "ServicioFacturacion" + suffix
}

func TestPartialReturnSerializesOriginalAndReturnedAmounts(t *testing.T) {
	for _, profile := range PerfilesSector() {
		if profile.Codigo != 24 && profile.Codigo != 47 && profile.Codigo != 48 {
			continue
		}
		t.Run(fmt.Sprintf("%d/%s", profile.Codigo, profile.Layout), func(t *testing.T) {
			req := baseItemConstruyeFactura(t)
			req.CodigoDocumentoSector, req.Layout = profile.Codigo, profile.Layout
			req.DatosSector = jsonRaw(datosDeEjemplo(profile))
			req.Items[0].DatosSector = datosDetalleDeEjemplo(profile)
			req.Items[0].PrecioUnitario, req.Items[0].SubTotal = 1250, 1250
			original := req.Items[0]
			original.Cantidad, original.SubTotal = 2, 2500
			req.OriginalItems = []ItemFactura{original}
			built, _, _, err := buildFacturaSDK(req, 1)
			if err != nil {
				t.Fatal(err)
			}
			data, err := xml.Marshal(built)
			if err != nil {
				t.Fatal(err)
			}
			var document struct {
				Details []struct {
					Code   int     `xml:"codigoDetalleTransaccion"`
					Amount float64 `xml:"subTotal"`
				} `xml:"detalle"`
			}
			if err := xml.Unmarshal(data, &document); err != nil {
				t.Fatal(err)
			}
			if len(document.Details) != 2 || document.Details[0].Code != 1 || document.Details[0].Amount != 2500 || document.Details[1].Code != 2 || document.Details[1].Amount != 1250 {
				t.Fatalf("wrong original/return details: %+v", document)
			}
		})
	}
}
