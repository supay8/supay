package siat

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

// Exercise optional setters too: merely registering a builder or building its
// root with empty data misses incompatible methods and lost item fields.
func TestEverySDKProfileBuildsWithAllExposedFields(t *testing.T) {
	for _, profile := range PerfilesSector() {
		if !profile.HasBuilder() {
			continue
		}
		t.Run(fmt.Sprintf("%d/%s", profile.Codigo, profile.Layout), func(t *testing.T) {
			req := baseItemConstruyeFactura(t)
			req.CodigoDocumentoSector = profile.Codigo
			req.Layout = profile.Layout
			req.Modalidad = ModalidadElectronica
			req.DatosSector = allSDKFields(t, profile.Campos)
			req.Items[0].DatosSector = allSDKFields(t, profile.CamposDetalle)
			built, _, _, err := buildFacturaSDK(req, 1)
			if err != nil {
				t.Fatal(err)
			}
			data, err := xml.Marshal(built)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range append(append([]CampoSector(nil), profile.Campos...), profile.CamposDetalle...) {
				if field.Metodo == "" {
					continue
				}
				name := strings.TrimPrefix(field.Metodo, "With")
				name = strings.ToLower(name[:1]) + name[1:]
				if !strings.Contains(string(data), "<"+name+">") {
					t.Errorf("field %s not serialized as %s", field.JSON, name)
				}
			}
		})
	}
}

func allSDKFields(t *testing.T, fields []CampoSector) json.RawMessage {
	t.Helper()
	values := make(map[string]any)
	for _, field := range fields {
		switch field.Tipo {
		case "int":
			values[field.JSON] = 1
		case "float":
			values[field.JSON] = 1.25
		case "fecha":
			values[field.JSON] = "2026-08-24"
		case "bool":
			values[field.JSON] = true
		case "json":
			values[field.JSON] = map[string]any{"descripcion": "PRUEBA", "valor": 12}
		default:
			values[field.JSON] = "PRUEBA"
		}
	}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSectorDataRejectsFractionalIntegerAndWrongJSONObject(t *testing.T) {
	p, _ := PerfilSector(3)
	data := allSDKFields(t, p.Campos)
	var values map[string]any
	_ = json.Unmarshal(data, &values)
	values["costos_gastos_nacionales"] = "not an object"
	data, _ = json.Marshal(values)
	if _, err := p.ValidarDatosSector(data); err == nil {
		t.Fatal("accepted invalid SDK map")
	}
	if _, err := normalizarValorCampo(1, campo("code", "", "int", true), 1.5, true); err == nil {
		t.Fatal("fractional code truncated")
	}
}
