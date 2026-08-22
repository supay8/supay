package siat

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
)

// TestCatalogoCubreLosSectoresDelSDK verifica que el registro contenga todos
// los documentos-sector con builder público en go-siat y que la clasificación
// normativa (tipoFacturaDocumento) sea la correcta por grupo.
func TestCatalogoCubreLosSectoresDelSDK(t *testing.T) {
	conCredito := map[int]bool{1: true, 2: true, 11: true, 12: true, 13: true, 14: true, 15: true,
		16: true, 17: true, 18: true, 19: true, 21: true, 22: true, 23: true, 30: true, 31: true,
		34: true, 35: true, 37: true, 38: true, 39: true, 41: true, 44: true, 51: true, 53: true, 55: true}
	sinCredito := map[int]bool{3: true, 4: true, 5: true, 6: true, 7: true, 8: true, 9: true,
		10: true, 20: true, 28: true, 36: true, 40: true, 42: true, 43: true, 45: true, 46: true,
		49: true, 50: true, 52: true, 54: true}
	ajuste := map[int]bool{24: true, 29: true, 47: true, 48: true}

	for _, p := range PerfilesSector() {
		switch {
		case conCredito[p.Codigo]:
			if got := p.TipoDocumentoResuelto(0); got != TipoDocumentoFacturaConCredito {
				t.Errorf("sector %d (%s): tipoDocumento=%d, se esperaba %d", p.Codigo, p.Nombre, got, TipoDocumentoFacturaConCredito)
			}
		case sinCredito[p.Codigo]:
			if got := p.TipoDocumentoResuelto(0); got != TipoDocumentoFacturaSinCredito {
				t.Errorf("sector %d (%s): tipoDocumento=%d, se esperaba %d", p.Codigo, p.Nombre, got, TipoDocumentoFacturaSinCredito)
			}
		case ajuste[p.Codigo]:
			if got := p.TipoDocumentoResuelto(0); got != TipoDocumentoNotaCreditoDebito {
				t.Errorf("sector %d (%s): tipoDocumento=%d, se esperaba %d", p.Codigo, p.Nombre, got, TipoDocumentoNotaCreditoDebito)
			}
			if !p.EsAjuste() {
				t.Errorf("sector %d debería ser documento de ajuste", p.Codigo)
			}
		default:
			t.Errorf("sector %d (%s) inesperado en el catálogo; clasifíquelo en este test", p.Codigo, p.Nombre)
		}
	}

	total := len(conCredito) + len(sinCredito) + len(ajuste)
	if got := len(PerfilesSector()); got != total {
		t.Errorf("el catálogo tiene %d sectores, se esperaban %d", got, total)
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
