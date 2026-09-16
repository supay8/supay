package pdf

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/models"
	"github.com/shopspring/decimal"
)

func TestGenerate_PDFValido(t *testing.T) {
	inv := sampleInvoice()
	data, err := Generate(InvoicePDFData{Invoice: inv})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("PDF demasiado pequeño: %d bytes", len(data))
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("el archivo no comienza con %%PDF: %q", data[:8])
	}
	// Debe contener la marca de fin de PDF.
	if !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatalf("el PDF no termina con %%EOF")
	}
}

func TestGenerate_ConQR(t *testing.T) {
	// Un PNG 4x4 válido generado con image/png.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}
	var qr bytes.Buffer
	if err := png.Encode(&qr, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}

	inv := sampleInvoice()
	data, err := Generate(InvoicePDFData{Invoice: inv, QRImage: qr.Bytes()})
	if err != nil {
		t.Fatalf("Generate con QR: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("el archivo no comienza con %%PDF")
	}
}

func TestGenerate_Validaciones(t *testing.T) {
	if _, err := Generate(InvoicePDFData{Invoice: nil}); err == nil {
		t.Fatal("se esperaba error con invoice nil")
	}
	if _, err := Generate(InvoicePDFData{Invoice: &models.Invoice{}}); err == nil {
		t.Fatal("se esperaba error con factura sin datos precargados")
	}
}

func TestDecodeQR(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	valid := base64.StdEncoding.EncodeToString(buf.Bytes())
	if got := DecodeQR(&valid); len(got) == 0 {
		t.Fatal("DecodeQR no decodificó el PNG base64")
	}
	dataURL := "data:image/png;base64," + valid
	if got := DecodeQR(&dataURL); len(got) == 0 {
		t.Fatal("DecodeQR no decodificó el data URL")
	}
	empty := ""
	if got := DecodeQR(&empty); got != nil {
		t.Fatal("DecodeQR debía devolver nil para cadena vacía")
	}
	if got := DecodeQR(nil); got != nil {
		t.Fatal("DecodeQR debía devolver nil para puntero nil")
	}
}

func sampleInvoice() *models.Invoice {
	act := "461011"
	comp := "C1"
	siatCode := 1
	cuf := "8727F63A15F8976591FDDE5B387C5D015A29E06A1A19E23EF34124CD"
	unitCode := 1
	return &models.Invoice{
		ID:                     "factura-1",
		InvoiceNumber:          1001,
		EmissionType:           models.EmissionEnLinea,
		CodigoMetodoPago:       1,
		CodigoMoneda:           1,
		TipoCambio:             decimal.NewFromInt(1),
		IssueDate:              time.Date(2026, 8, 5, 14, 30, 0, 0, time.UTC),
		Subtotal:               decimal.NewFromFloat(150.50),
		Discount:               decimal.Zero,
		Total:                  decimal.NewFromFloat(150.50),
		Cuf:                    &cuf,
		CustomerId:             "customer-1",
		CustomerDocumentType:   models.DocNIT,
		CustomerDocumentNumber: "3456789012",
		CustomerComplement:     &comp,
		CustomerName:           "Cliente Supay S.A.",
		CustomerCode:           "NIT3456789012",
		Company: models.Company{
			ID:              "company-1",
			Nit:             "102965402",
			BusinessName:    "Supay SRL",
			Municipio:       "La Paz",
			Direccion:       "Av. Camacho 123",
			Telefono:        "2444444",
			CodigoActividad: &act,
			PiePagina:       "Ley Nro 453: Toda persona, natural o juridica, tiene derecho a la informacion.",
		},
		PointOfSale: models.PointOfSale{
			CodigoSucursal:   0,
			CodigoPuntoVenta: 1,
			SiatCode:         &siatCode,
			Description:      "Punto de venta principal",
		},
		CufdRecord: models.Cufd{
			Cufd:          "CUFD-ABC123",
			CodigoControl: "A19E23EF34124CD",
		},
		Items: []models.InvoiceItem{
			{
				Code:            "SRV-001",
				Description:     "Servicio de desarrollo de software a medida para plataformas de facturacion",
				CodigoActividad: nil,
				Quantity:        decimal.NewFromInt(1),
				UnitPrice:       decimal.NewFromFloat(100.25),
				Subtotal:        decimal.NewFromFloat(100.25),
				UnitCode:        &unitCode,
			},
			{
				Code:            "SRV-002",
				Description:     "Mantenimiento",
				CodigoActividad: &act,
				Quantity:        decimal.NewFromInt(2),
				UnitPrice:       decimal.NewFromFloat(25.125),
				Subtotal:        decimal.NewFromFloat(50.25),
			},
		},
	}
}
