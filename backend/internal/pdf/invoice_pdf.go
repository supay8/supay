// Package pdf genera el PDF de una factura electrónica conforme al layout
// habitual de las facturas del SIAT (cabecera del emisor, CUF, cliente,
// detalle, totales, leyenda y código QR).
package pdf

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/brandsrx/supay/internal/models"
	"github.com/jung-kurt/gofpdf"
)

const (
	pageW = 210.0 // A4 width (mm)
	pageH = 297.0 // A4 height (mm)
	ml    = 15.0  // left margin
	mr    = 15.0  // right margin
	mt    = 12.0  // top margin
	mb    = 12.0  // bottom margin
	bodyW = pageW - ml - mr
)

// InvoicePDFData agrupa la información necesaria para renderizar el PDF.
// Se usa models.Invoice (con Company, Customer, PointOfSale, CufdRecord e
// Items precargados) y el código QR del CUFD vigente.
type InvoicePDFData struct {
	Invoice *models.Invoice
	QRImage []byte // PNG del código QR del CUFD (si está disponible)
}

// Generate renderiza el PDF de la factura y lo devuelve como []byte.
func Generate(data InvoicePDFData) ([]byte, error) {
	if data.Invoice == nil {
		return nil, fmt.Errorf("pdf: invoice es obligatoria")
	}
	inv := data.Invoice
	if inv.Company.ID == "" || inv.Customer.Name == "" {
		return nil, fmt.Errorf("pdf: factura sin datos de empresa o cliente precargados")
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DejaVu", "", dejaVuRegular)
	pdf.AddUTF8FontFromBytes("DejaVu", "B", dejaVuBold)
	pdf.SetMargins(ml, mt, mr)
	pdf.SetAutoPageBreak(true, mb)
	pdf.AddPage()

	drawHeader(pdf, inv)
	drawCUF(pdf, inv)
	drawFechasYNumeros(pdf, inv)
	drawCliente(pdf, inv)
	drawDetalle(pdf, inv)
	drawTotales(pdf, inv)
	drawLeyenda(pdf, inv)
	drawQR(pdf, data.QRImage, inv)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf: generar: %w", err)
	}
	return buf.Bytes(), nil
}

// drawHeader imprime los datos del emisor (NIT, razón social, dirección,
// teléfono, municipio y la leyenda "FACTURA ELECTRÓNICA").
func drawHeader(pdf *gofpdf.Fpdf, inv *models.Invoice) {
	company := inv.Company
	sucursal := "Casa Matriz"
	if inv.PointOfSale.CodigoSucursal > 0 {
		sucursal = fmt.Sprintf("Sucursal %d", inv.PointOfSale.CodigoSucursal)
	}

	pdf.SetFont("DejaVu", "B", 10)
	pdf.SetTextColor(40, 40, 40)
	pdf.CellFormat(bodyW, 6, strings.ToUpper(company.BusinessName), "", 1, "C", false, 0, "")
	pdf.SetFont("DejaVu", "", 8)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(bodyW, 4.5, fmt.Sprintf("NIT: %s", company.Nit), "", 1, "C", false, 0, "")
	if strings.TrimSpace(company.Direccion) != "" {
		pdf.CellFormat(bodyW, 4.5, company.Direccion, "", 1, "C", false, 0, "")
	}
	if strings.TrimSpace(company.Telefono) != "" {
		pdf.CellFormat(bodyW, 4.5, fmt.Sprintf("Tel.: %s", company.Telefono), "", 1, "C", false, 0, "")
	}
	if strings.TrimSpace(company.Municipio) != "" {
		pdf.CellFormat(bodyW, 4.5, company.Municipio, "", 1, "C", false, 0, "")
	}

	pdf.Ln(1.5)
	pdf.SetFillColor(18, 52, 86)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("DejaVu", "B", 11)
	pdf.CellFormat(bodyW, 8, "FACTURA ELECTRONICA", "1", 1, "C", true, 0, "")
	pdf.SetTextColor(40, 40, 40)

	pdf.Ln(0.5)
	pdf.SetFont("DejaVu", "", 7.5)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(bodyW, 4, fmt.Sprintf("%s - Punto de Venta: %d (%s)", sucursal, codigoPuntoVenta(inv), inv.PointOfSale.Description), "", 1, "C", false, 0, "")
	pdf.SetTextColor(40, 40, 40)
}

// drawCUF imprime el Código Único de Factura dentro de un recuadro, tal como
// lo muestran las facturas electrónicas del SIAT.
func drawCUF(pdf *gofpdf.Fpdf, inv *models.Invoice) {
	pdf.Ln(2)
	pdf.SetFont("DejaVu", "B", 7)
	pdf.SetTextColor(18, 52, 86)
	pdf.CellFormat(bodyW, 4, "CUF (Codigo Unico de Factura)", "", 1, "L", false, 0, "")

	cuf := cufText(inv)
	pdf.SetDrawColor(18, 52, 86)
	pdf.SetFillColor(244, 247, 251)
	pdf.SetFont("DejaVu", "", 7.5)
	pdf.SetTextColor(30, 30, 30)
	pdf.MultiCell(bodyW, 4.2, cuf, "1", "C", true)
	pdf.SetTextColor(40, 40, 40)
}

// drawFechasYNumeros imprime la fecha de emisión, número de factura y el
// código de control del CUFD en una línea.
func drawFechasYNumeros(pdf *gofpdf.Fpdf, inv *models.Invoice) {
	pdf.Ln(1.5)
	pdf.SetFont("DejaVu", "", 8)
	left := fmt.Sprintf("Fecha: %s", inv.IssueDate.UTC().Format("02/01/2006 15:04:05"))
	right := fmt.Sprintf("Nº Factura: %d", inv.InvoiceNumber)
	codigoControl := inv.CufdRecord.CodigoControl
	center := "Cod. Control: " + codigoControl
	if codigoControl == "" {
		center = ""
	}

	pdf.SetTextColor(40, 40, 40)
	pdf.CellFormat(bodyW/3, 5, left, "", 0, "L", false, 0, "")
	if center != "" {
		pdf.CellFormat(bodyW/3, 5, center, "", 0, "C", false, 0, "")
	} else {
		pdf.CellFormat(bodyW/3, 5, "", "", 0, "C", false, 0, "")
	}
	pdf.CellFormat(bodyW/3, 5, right, "", 1, "R", false, 0, "")
}

// drawCliente imprime los datos del cliente.
func drawCliente(pdf *gofpdf.Fpdf, inv *models.Invoice) {
	pdf.Ln(2)
	pdf.SetFillColor(18, 52, 86)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("DejaVu", "B", 8.5)
	pdf.CellFormat(bodyW, 6, "DATOS DEL CLIENTE", "1", 1, "L", true, 0, "")
	pdf.SetTextColor(40, 40, 40)

	customer := inv.Customer
	pdf.SetFont("DejaVu", "", 8)
	pdf.CellFormat(bodyW/2, 5, fmt.Sprintf("Nombre/Razón Social: %s", customer.Name), "", 1, "L", false, 0, "")
	pdf.CellFormat(bodyW/2, 5, fmt.Sprintf("Documento (%s): %s", customer.DocumentType, customer.DocumentNumber), "", 1, "L", false, 0, "")
}

// drawDetalle imprime la tabla de ítems.
func drawDetalle(pdf *gofpdf.Fpdf, inv *models.Invoice) {
	pdf.Ln(2)

	const (
		colCod   = 16.0
		colCant  = 16.0
		colPrecio = 24.0
		colSub   = 24.0
		colDesc  = bodyW - colCod - colCant - colPrecio - colSub
		lineH    = 5.0
	)

	pdf.SetFillColor(18, 52, 86)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("DejaVu", "B", 8)
	headers := []struct {
		w     float64
		text  string
		align string
	}{
		{colCod, "Código", "C"},
		{colDesc, "Descripción", "L"},
		{colCant, "Cant.", "C"},
		{colPrecio, "Precio", "R"},
		{colSub, "SubTotal", "R"},
	}
	for _, h := range headers {
		pdf.CellFormat(h.w, 6, h.text, "1", 0, h.align, true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(40, 40, 40)
	pdf.SetFont("DejaVu", "", 8)

	for _, it := range inv.Items {
		// Estimamos el alto de la fila según las líneas que ocupe la
		// descripción dentro del ancho de la columna.
		lines := len(pdf.SplitLines([]byte(it.Description), colDesc))
		if lines < 1 {
			lines = 1
		}
		rowH := float64(lines) * lineH
		if rowH < lineH {
			rowH = lineH
		}

		x0 := pdf.GetX()
		y0 := pdf.GetY()
		pdf.SetFillColor(250, 250, 250)
		pdf.Rect(x0, y0, bodyW, rowH, "F")

		pdf.SetTextColor(40, 40, 40)
		pdf.SetXY(x0, y0)
		pdf.CellFormat(colCod, rowH, it.Code, "", 0, "C", false, 0, "")
		pdf.SetXY(x0+colCod, y0)
		pdf.MultiCell(colDesc, lineH, it.Description, "", "L", false)
		pdf.SetXY(x0+colCod+colDesc, y0)
		pdf.CellFormat(colCant, rowH, fmt.Sprintf("%.3f", it.Quantity), "", 0, "C", false, 0, "")
		pdf.SetXY(x0+colCod+colDesc+colCant, y0)
		pdf.CellFormat(colPrecio, rowH, fmt.Sprintf("%.2f", it.UnitPrice), "", 0, "R", false, 0, "")
		pdf.SetXY(x0+colCod+colDesc+colCant+colPrecio, y0)
		pdf.CellFormat(colSub, rowH, fmt.Sprintf("%.2f", it.Subtotal), "", 0, "R", false, 0, "")
		pdf.SetXY(ml, y0+rowH)
	}

	// Línea inferior de la tabla.
	pdf.Ln(1)
}

// drawTotales imprime subtotal, descuento y monto total.
func drawTotales(pdf *gofpdf.Fpdf, inv *models.Invoice) {
	pdf.Ln(1)
	colW := bodyW * 0.55
	valW := bodyW * 0.45

	pdf.SetFont("DejaVu", "", 8)
	rows := []struct {
		label string
		value float64
		bold  bool
	}{
		{"SubTotal", inv.Subtotal, false},
		{"Descuento", inv.Discount, false},
		{"TOTAL Bs", inv.Total, true},
	}
	for _, row := range rows {
		if row.bold {
			pdf.SetFont("DejaVu", "B", 9)
			pdf.SetFillColor(230, 238, 247)
		} else {
			pdf.SetFont("DejaVu", "", 8)
			pdf.SetFillColor(245, 245, 245)
		}
		pdf.CellFormat(colW, 6, row.label, "1", 0, "L", true, 0, "")
		pdf.CellFormat(valW, 6, fmt.Sprintf("%.2f", row.value), "1", 1, "R", true, 0, "")
	}
	pdf.SetFont("DejaVu", "", 8)
	pdf.CellFormat(bodyW, 5, fmt.Sprintf("Monto Total en Moneda (%s)", currencyText(inv.CodigoMoneda)), "", 1, "L", false, 0, "")
}

// drawLeyenda imprime la leyenda (pie de página) del emisor.
func drawLeyenda(pdf *gofpdf.Fpdf, inv *models.Invoice) {
	if strings.TrimSpace(inv.Company.PiePagina) == "" {
		return
	}
	pdf.Ln(2)
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetFont("DejaVu", "", 7)
	pdf.SetTextColor(80, 80, 80)
	pdf.MultiCell(bodyW, 3.5, inv.Company.PiePagina, "T", "L", false)
	pdf.SetTextColor(40, 40, 40)
}

// drawQR embebe el código QR del CUFD en la esquina inferior izquierda.
func drawQR(pdf *gofpdf.Fpdf, qr []byte, inv *models.Invoice) {
	if len(qr) == 0 {
		return
	}
	// Cuadro del código QR, 24x24 mm al pie de la página.
	imgOpt := gofpdf.ImageOptions{
		ImageType: "PNG",
		ReadDpi:   true,
	}
	info := pdf.RegisterImageOptionsReader("qr", imgOpt, bytes.NewReader(qr))
	if info == nil {
		// No rompe el PDF si el QR no es válido; se omite.
		return
	}
	x := ml
	y := pageH - mb - 26
	pdf.Image("qr", x, y, 24, 24, false, "PNG", 0, "")
	pdf.SetFont("DejaVu", "", 6.5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(x+26, y+6)
	pdf.MultiCell(bodyW-26, 3, "Escaneá este código QR para verificar la factura en el portal del SIAT.", "", "L", false)
	pdf.SetTextColor(40, 40, 40)
}

func codigoPuntoVenta(inv *models.Invoice) int {
	if inv.PointOfSale.SiatCode != nil {
		return *inv.PointOfSale.SiatCode
	}
	return inv.PointOfSale.CodigoPuntoVenta
}

func cufText(inv *models.Invoice) string {
	if inv.Cuf != nil && *inv.Cuf != "" {
		return *inv.Cuf
	}
	return "PENDIENTE DE EMISION"
}

func currencyText(code int) string {
	switch code {
	case 1:
		return "Bolivianos (Bs)"
	case 2:
		return "Dólares Americanos (USD)"
	case 3:
		return "Euros (EUR)"
	default:
		return "Bolivianos (Bs)"
	}
}

// DecodeQR extrae la imagen PNG (base64) del código QR del CUFD.
func DecodeQR(codigoQR *string) []byte {
	if codigoQR == nil || strings.TrimSpace(*codigoQR) == "" {
		return nil
	}
	data := strings.TrimSpace(*codigoQR)
	// Algunos valores vienen con prefijo data:image/png;base64,
	data = strings.TrimPrefix(data, "data:image/png;base64,")
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil
	}
	return decoded
}
