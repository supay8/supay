package siat

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
)

// TestCatalogoCubreLosSectoresDelSDK verifica invariantes que deben cumplirse
// para cualquier entrada del registro. La paridad de códigos y raíces XML se
// deriva directamente del catálogo fuente del SDK en sector_sdk_parity_test.go.
func TestCatalogoCubreLosSectoresDelSDK(t *testing.T) {
	for _, p := range PerfilesSector() {
		if p.Codigo == 33 {
			if p.HasBuilder() || p.Operacion != OperacionRecepcionFactura {
				t.Errorf("sector 33 debe ser recepción sin builder: %#v", p)
			}
			continue
		}
		if p.TipoFacturaDocumento < TipoDocumentoFacturaConCredito || p.TipoFacturaDocumento > TipoDocumentoNotaCreditoDebito {
			t.Errorf("sector %d tiene tipoFacturaDocumento inválido: %d", p.Codigo, p.TipoFacturaDocumento)
		}
		if !p.HasBuilder() {
			t.Errorf("sector %d no tiene builders", p.Codigo)
		}
		if p.Operacion == OperacionDocumentoAjuste && p.Facade.Fixed() != FachadaDocumentoAjuste {
			t.Errorf("sector %d debe usar fachada DocumentoAjuste", p.Codigo)
		}
	}
}

// datosDeEjemplo genera un objeto datos_sector válido para el perfil: cada
// campo requerido recibe un valor plausible según su tipo declarado.
func datosDeEjemplo(p *SectorProfile) string {
	pares := make([]string, 0, len(p.Campos))
	for _, c := range p.Campos {
		if !c.Requerido {
			continue
		}
		switch c.Tipo {
		case "int":
			pares = append(pares, fmt.Sprintf("%q:1", c.JSON))
		case "float":
			pares = append(pares, fmt.Sprintf("%q:1.0", c.JSON))
		case "fecha":
			pares = append(pares, fmt.Sprintf("%q:%q", c.JSON, "2025-08-15"))
		default:
			pares = append(pares, fmt.Sprintf("%q:%q", c.JSON, "PRUEBA"))
		}
	}
	return "{" + strings.Join(pares, ",") + "}"
}

// TestBuildFacturaTodosLosSectores ejercita buildFacturaSDK contra TODOS los
// perfiles registrados con datos_sector mínimos válidos: detecta cualquier
// desajuste entre los campos declarados y los métodos reales de los builders
// del SDK (firma, tipos o existencia).
func TestBuildFacturaTodosLosSectores(t *testing.T) {
	fecha := time.Date(2025, 8, 15, 10, 30, 0, 0, LaPaz)

	for _, p := range PerfilesSector() {
		p := p
		t.Run(fmt.Sprintf("sector_%02d", p.Codigo), func(t *testing.T) {
			if !p.HasBuilder() {
				return
			}
			datos := jsonRaw(datosDeEjemplo(p))
			req := SolicitudFactura{
				CodigoAmbiente:        AmbientePruebas,
				CodigoSistema:         "SYS-TEST",
				Nit:                   "1020304050",
				Modalidad:             goSiat.ModalidadElectronica,
				NumeroFactura:         1,
				Cuis:                  "CUIS-TEST",
				Cufd:                  "CUFD-TEST",
				CodigoControl:         "CONTROL-CODE-29-CHARACTERS-01",
				FechaEmision:          fecha,
				Usuario:               "SUPAY",
				Leyenda:               "Ley N° 453",
				RazonSocialEmisor:     "EMPRESA TEST SRL",
				Municipio:             "LA PAZ",
				Direccion:             "AV. TEST 123",
				CodigoMetodoPago:      1,
				CodigoMoneda:          1,
				TipoCambio:            1,
				MontoTotal:            100,
				CodigoDocumentoSector: p.Codigo,
				Layout:                p.Layout,
				DatosSector:           datos,
				Cliente: ClienteFactura{
					NombreRazonSocial:            "CLIENTE TEST",
					CodigoTipoDocumentoIdentidad: 1,
					NumeroDocumento:              "1234567",
					CodigoCliente:                "C-001",
				},
				Items: []ItemFactura{{
					ActividadEconomica: "473000",
					CodigoProductoSin:  12345,
					CodigoProducto:     "P-001",
					Descripcion:        "Ítem de prueba",
					Cantidad:           1,
					UnidadMedida:       1,
					PrecioUnitario:     100,
					SubTotal:           100,
				}},
			}

			factura, cuf, tipoDoc, err := buildFacturaSDK(req, goSiat.EmisionOnline)
			if err != nil {
				t.Fatalf("buildFacturaSDK sector %d: %v", p.Codigo, err)
			}
			if cuf == "" {
				t.Fatalf("sector %d: CUF vacío", p.Codigo)
			}
			if tipoDoc != p.TipoDocumentoResuelto(0) {
				t.Fatalf("sector %d: tipoDoc=%d, se esperaba %d", p.Codigo, tipoDoc, p.TipoDocumentoResuelto(0))
			}
			xmlData, err := xml.Marshal(factura)
			if err != nil {
				t.Fatalf("sector %d: xml.Marshal: %v", p.Codigo, err)
			}
			xmlStr := string(xmlData)
			if !strings.Contains(xmlStr, fmt.Sprintf("<codigoDocumentoSector>%d</codigoDocumentoSector>", p.Codigo)) &&
				!strings.Contains(xmlStr, fmt.Sprintf("<codigoDocumentoSector>%d", p.Codigo)) {
				t.Errorf("sector %d: el XML no declara codigoDocumentoSector:\n%.400s", p.Codigo, xmlStr)
			}
		})
	}
}

func jsonRaw(s string) []byte { return []byte(s) }
