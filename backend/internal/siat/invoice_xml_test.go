package siat

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/models"
)

func TestDocumentTypeCode(t *testing.T) {
	tests := []struct {
		dt   models.DocumentType
		want int
	}{
		{models.DocCI, 1},
		{models.DocCEX, 2},
		{models.DocPAS, 3},
		{models.DocOD, 4},
		{models.DocNIT, 5},
		{"DESCONOCIDO", 1},
	}
	for _, tt := range tests {
		if got := DocumentTypeCode(tt.dt); got != tt.want {
			t.Errorf("DocumentTypeCode(%q) = %d; se esperaba %d", tt.dt, got, tt.want)
		}
	}
}

func TestBuildFacturaXML(t *testing.T) {
	inv := testInvoice()

	payload, err := BuildFacturaXML(InvoiceXMLParams{
		Invoice:             inv,
		Cuf:                 "CUF-123",
		Cufd:                "CUFD-456",
		CodigoControl:       "CTRL-789",
		DireccionSucursal:   "Av. Camacho 123",
		Municipio:           "La Paz",
		Telefono:            "2444444",
		PiePagina:           "Ley Nº 453",
		CodigoActividad:     "461011",
		Modalidad:           1,
		TipoEmision:         1,
		CodigoDocumentoSector: 1,
		CodigoPuntoVenta:    2,
	})
	if err != nil {
		t.Fatalf("BuildFacturaXML: %v", err)
	}

	doc := string(payload)
	// El XML debe estar bien formado: volver a parsearlo no debe fallar.
	var parsed facturaElectronicaCompraVenta
	if err := xml.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("el XML generado no es válido: %v", err)
	}

	checks := []string{
		"<facturaElectronicaCompraVenta",
		"<nitEmisor>102965402</nitEmisor>",
		"<razonSocialEmisor>Supay SRL</razonSocialEmisor>",
		"<cuf>CUF-123</cuf>",
		"<cufd>CUFD-456</cufd>",
		"<codigoPuntoVenta>2</codigoPuntoVenta>",
		"<codigoTipoDocumentoIdentidad>5</codigoTipoDocumentoIdentidad>",
		"<numeroDocumento>3456789012</numeroDocumento>",
		"<complemento>C001</complemento>",
		"<codigoMetodoPago>1</codigoMetodoPago>",
		"<montoTotal>150.5</montoTotal>",
		"<codigoMoneda>1</codigoMoneda>",
		"<tipoCambio>1</tipoCambio>",
		"<leyenda>Ley Nº 453</leyenda>",
		"<codigoDocumentoSector>1</codigoDocumentoSector>",
		"<actividadEconomica>461011</actividadEconomica>",
		"<descripcion>Servicio de software</descripcion>",
		"<subTotal>100.25</subTotal>",
	}
	for _, c := range checks {
		if !strings.Contains(doc, c) {
			t.Errorf("XML no contiene %q", c)
		}
	}

	if len(parsed.Detalle) != 2 {
		t.Fatalf("detalle len = %d; se esperaba 2", len(parsed.Detalle))
	}
	// Los items sin código de actividad deben heredar el del emisor.
	if parsed.Detalle[0].ActividadEconomica != "461011" {
		t.Errorf("actividad item[0] = %q; se esperaba 461011", parsed.Detalle[0].ActividadEconomica)
	}
	if parsed.Cabecera.FechaEmision == "" {
		t.Errorf("fechaEmision vacía")
	}
	if parsed.Cabecera.NumeroFactura != inv.InvoiceNumber {
		t.Errorf("numeroFactura = %d; se esperaba %d", parsed.Cabecera.NumeroFactura, inv.InvoiceNumber)
	}
}

func TestBuildFacturaXML_Validaciones(t *testing.T) {
	inv := testInvoice()

	if _, err := BuildFacturaXML(InvoiceXMLParams{Invoice: nil, Cuf: "x"}); err == nil {
		t.Fatal("se esperaba error con Invoice nil")
	}
	if _, err := BuildFacturaXML(InvoiceXMLParams{Invoice: inv, Cuf: ""}); err == nil {
		t.Fatal("se esperaba error con Cuf vacío")
	}
}

func testInvoice() *models.Invoice {
	comp := "C001"
	act := "461011"
	siatCode := 2
	unitCode := 1
	return &models.Invoice{
		ID:             "11111111-1111-1111-1111-111111111111",
		CompanyId:      "22222222-2222-2222-2222-222222222222",
		CustomerId:     "33333333-3333-3333-3333-333333333333",
		PointOfSaleId:  "44444444-4444-4444-4444-444444444444",
		InvoiceNumber:  1001,
		EmissionType:   models.EmissionEnLinea,
		CodigoMetodoPago: 1,
		CodigoMoneda:   1,
		TipoCambio:     1,
		IssueDate:      time.Date(2025, 5, 10, 14, 30, 0, 0, time.UTC),
		Subtotal:       150.50,
		Discount:       0,
		Total:          150.50,
		Company: models.Company{
			Nit:             "102965402",
			BusinessName:    "Supay SRL",
			CodigoSistema:   "SUPAY-1",
			Municipio:       "La Paz",
			Direccion:       "Av. Camacho 123",
			Telefono:        "2444444",
			CodigoActividad: &act,
			PiePagina:       "Ley Nº 453",
		},
		Customer: models.Customer{
			DocumentType:   models.DocNIT,
			DocumentNumber: "3456789012",
			Complement:     &comp,
			Name:           "Cliente Supay",
		},
		PointOfSale: models.PointOfSale{
			CodigoSucursal:   0,
			CodigoPuntoVenta: 1,
			SiatCode:         &siatCode,
		},
		Items: []models.InvoiceItem{
			{
				Code:            "SRV-001",
				Description:     "Servicio de software",
				CodigoActividad: nil,
				Quantity:        1,
				UnitPrice:       100.25,
				Subtotal:        100.25,
				UnitCode:        &unitCode,
			},
			{
				Code:            "SRV-002",
				Description:     "Mantenimiento",
				CodigoActividad: &act,
				Quantity:        2,
				UnitPrice:       25.125,
				Subtotal:        50.25,
			},
		},
	}
}
