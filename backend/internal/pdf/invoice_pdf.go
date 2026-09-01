// Package pdf genera el PDF formal de una factura electrónica SIAT.
// Diseño sobrio, corporativo y legible: tipografía DejaVu, paleta navy/slate,
// tarjetas con borde sutil, tabla con cabecera oscura y QR nativo gpdf.
package pdf

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/brandsrx/supay/internal/models"
	gpdf "github.com/gpdf-dev/gpdf"
	"github.com/gpdf-dev/gpdf/document"
	"github.com/gpdf-dev/gpdf/pdf"
	"github.com/gpdf-dev/gpdf/template"
)

// InvoicePDFData agrupa la información necesaria para renderizar el PDF.
type InvoicePDFData struct {
	Invoice *models.Invoice
	QRImage []byte // compat: si CUF está pendiente se usa como fallback Image
}

// Generate renderiza el PDF formal de la factura.
func Generate(data InvoicePDFData) ([]byte, error) {
	if data.Invoice == nil {
		return nil, fmt.Errorf("pdf: invoice es obligatoria")
	}
	inv := data.Invoice
	if inv.Company.ID == "" || inv.Customer.Name == "" {
		return nil, fmt.Errorf("pdf: factura sin datos de empresa o cliente precargados")
	}

	title := fmt.Sprintf("Factura %d - %s", inv.InvoiceNumber, inv.Company.BusinessName)
	doc := gpdf.NewDocument(
		gpdf.WithPageSize(document.A4),
		gpdf.WithMargins(document.Edges{
			Top:    document.Mm(10),
			Right:  document.Mm(14),
			Bottom: document.Mm(12),
			Left:   document.Mm(14),
		}),
		gpdf.WithMetadata(document.DocumentMetadata{
			Title:   title,
			Author:  inv.Company.BusinessName,
			Subject: "Factura electrónica SIAT",
			Creator: "Supay",
		}),
		template.WithFont("DejaVu", dejaVuRegular),
		template.WithFont("DejaVu-Bold", dejaVuBold),
		template.WithDefaultFont("DejaVu", 8),
	)

	page := doc.AddPage()

	drawTopRule(page)
	drawHeaderFormal(page, inv)
	drawCUFFormal(page, inv)
	drawFechasCard(page, inv)
	drawClienteFormal(page, inv)
	drawDetalleFormal(page, inv)
	drawTotalesFormal(page, inv)
	drawLeyendaFormal(page, inv)
	drawQRFormal(page, inv, data.QRImage)
	drawFooter(page, inv)

	out, err := doc.Generate()
	if err != nil {
		return nil, fmt.Errorf("pdf: generar: %w", err)
	}
	return out, nil
}

// ── Paleta formal ──────────────────────────────────────────────────────────
var (
	navy       = pdf.RGBHex(0x0F2A44) // cabeceras, líneas principales
	navyLight  = pdf.RGBHex(0x1E3A5F)
	slate900   = pdf.RGBHex(0x111827)
	slate700   = pdf.RGBHex(0x374151)
	slate500   = pdf.RGBHex(0x6B7280)
	slate400   = pdf.RGBHex(0x9CA3AF)
	lineSoft   = pdf.RGBHex(0xE5E7EB)
	lineMid    = pdf.RGBHex(0xD1D5DB)
	bgCanvas   = pdf.RGBHex(0xF8FAFC)
	bgCuf      = pdf.RGBHex(0xF1F5F9)
	bgStripe   = pdf.RGBHex(0xF9FAFB)
	bgHeader   = pdf.RGBHex(0x0F2A44)
	goldSubtle = pdf.RGBHex(0xF59E0B)
)

// ── Helpers de estilo ──────────────────────────────────────────────────────

func drawTopRule(page *template.PageBuilder) {
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Line(template.LineColor(navy), template.LineThickness(document.Pt(1.4)))
			c.Spacer(document.Mm(3))
		})
	})
}

// drawHeaderFormal: izquierda datos del emisor, derecha tarjeta FACTURA ELECTRONICA.
func drawHeaderFormal(page *template.PageBuilder, inv *models.Invoice) {
	company := inv.Company
	sucursal := "Casa Matriz"
	if inv.PointOfSale.CodigoSucursal > 0 {
		sucursal = fmt.Sprintf("Sucursal %d", inv.PointOfSale.CodigoSucursal)
	}
	// fila principal 8 / 4
	page.AutoRow(func(r *template.RowBuilder) {
		// ── Col izquierda: emisor
		r.Col(8, func(c *template.ColBuilder) {
			c.Text(strings.ToUpper(company.BusinessName),
				template.FontSize(12), template.Bold(), template.FontFamily("DejaVu-Bold"),
				template.TextColor(navy), template.AlignLeft())
			c.Spacer(document.Mm(1.2))
			c.Line(template.LineColor(goldSubtle), template.LineThickness(document.Pt(0.7)))
			c.Spacer(document.Mm(1.8))

			c.Text(fmt.Sprintf("NIT  %s", company.Nit),
				template.FontSize(8), template.Bold(), template.FontFamily("DejaVu-Bold"),
				template.TextColor(slate900), template.AlignLeft())
			if strings.TrimSpace(company.Direccion) != "" {
				c.Text(company.Direccion, template.FontSize(7.5), template.TextColor(slate700), template.AlignLeft())
			}
			// Tel / Municipio en una línea
			var contact string
			if strings.TrimSpace(company.Telefono) != "" {
				contact = fmt.Sprintf("Tel. %s", company.Telefono)
			}
			if strings.TrimSpace(company.Municipio) != "" {
				if contact != "" {
					contact += "  ·  "
				}
				contact += company.Municipio
			}
			if contact != "" {
				c.Text(contact, template.FontSize(7.2), template.TextColor(slate500), template.AlignLeft())
			}
			if company.CodigoActividad != nil && strings.TrimSpace(*company.CodigoActividad) != "" {
				c.Spacer(document.Mm(1))
				c.Text(fmt.Sprintf("Actividad Económica: %s", strings.TrimSpace(*company.CodigoActividad)),
					template.FontSize(6.8), template.TextColor(slate500), template.AlignLeft())
			}
			c.Spacer(document.Mm(1))
			c.Text(fmt.Sprintf("%s  ·  Punto de Venta %d", sucursal, codigoPuntoVenta(inv)),
				template.FontSize(6.8), template.TextColor(slate400), template.AlignLeft())
			if strings.TrimSpace(inv.PointOfSale.Description) != "" {
				c.Text(inv.PointOfSale.Description, template.FontSize(6.8), template.TextColor(slate400), template.AlignLeft())
			}
		})
		// ── Col derecha: tarjeta FACTURA
		r.Col(4, func(c *template.ColBuilder) {
			c.Box(func(c *template.ColBuilder) {
				c.Text("FACTURA", template.FontSize(13), template.Bold(), template.FontFamily("DejaVu-Bold"), template.TextColor(navy), template.AlignCenter())
				c.Text("ELECTRÓNICA", template.FontSize(7.5), template.TextColor(slate500), template.AlignCenter(), template.LetterSpacing(1.2))
				c.Spacer(document.Mm(2.2))
				c.Line(template.LineColor(lineSoft), template.LineThickness(document.Pt(0.6)))
				c.Spacer(document.Mm(2.2))

				c.Text(fmt.Sprintf("Nº  %d", inv.InvoiceNumber),
					template.FontSize(10), template.Bold(), template.FontFamily("DejaVu-Bold"),
					template.TextColor(slate900), template.AlignCenter())
				c.Spacer(document.Mm(0.8))
				c.Text(fmt.Sprintf("Fecha  %s", inv.IssueDate.UTC().Format("02/01/2006")),
					template.FontSize(7.2), template.TextColor(slate700), template.AlignCenter())
				c.Text(inv.IssueDate.UTC().Format("15:04:05  UTC"),
					template.FontSize(6.7), template.TextColor(slate500), template.AlignCenter())

				// Modalidad / Moneda sutil
				c.Spacer(document.Mm(1.5))
				c.Text(fmt.Sprintf("%s  ·  %s", emissionLabel(inv.EmissionType), currencyText(inv.CodigoMoneda)),
					template.FontSize(6.5), template.TextColor(slate400), template.AlignCenter())
			},
				template.WithBoxBackground(pdf.White),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.7)), template.BorderColor(navy))),
				template.WithBoxPadding(document.Edges{Top: document.Mm(4), Right: document.Mm(4), Bottom: document.Mm(4), Left: document.Mm(4)}),
			)
		})
	})
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) { c.Spacer(document.Mm(3)) })
	})
}

// drawCUFFormal: tarjeta clara con label y valor monoespaciado.
func drawCUFFormal(page *template.PageBuilder, inv *models.Invoice) {
	cuf := cufText(inv)
	pending := cuf == "PENDIENTE DE EMISION"
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Box(func(c *template.ColBuilder) {
				c.Text("CÓDIGO ÚNICO DE FACTURA  ·  CUF",
					template.FontSize(6.6), template.Bold(), template.FontFamily("DejaVu-Bold"),
					template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.8))
				c.Spacer(document.Mm(1.4))
				if pending {
					c.Text(cuf, template.FontSize(8), template.TextColor(slate400), template.AlignCenter())
				} else {
					// CUF largo: tamaño menor, tracking sutil para legibilidad
					c.Text(cuf, template.FontSize(7.4), template.TextColor(slate900), template.AlignCenter(), template.LetterSpacing(0.2))
				}
			},
				template.WithBoxBackground(bgCuf),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.4)), template.BorderColor(lineMid))),
				template.WithBoxPadding(document.UniformEdges(document.Mm(2.8))),
			)
		})
	})
}

// drawFechasCard: fecha, control y estado en grilla 3 cols con etiquetas.
func drawFechasCard(page *template.PageBuilder, inv *models.Invoice) {
	leftLabel := "FECHA DE EMISIÓN"
	leftValue := inv.IssueDate.UTC().Format("02/01/2006 15:04:05")
	centerLabel := "CÓDIGO DE CONTROL"
	centerValue := inv.CufdRecord.CodigoControl
	if strings.TrimSpace(centerValue) == "" {
		centerValue = "—"
	}
	rightLabel := "Nº FACTURA"
	rightValue := fmt.Sprintf("%d", inv.InvoiceNumber)

	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) { c.Spacer(document.Mm(2.5)) })
	})
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(4, func(c *template.ColBuilder) {
			c.Text(leftLabel, template.FontSize(6.5), template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.6))
			c.Text(leftValue, template.FontSize(7.6), template.TextColor(slate900), template.AlignLeft())
		})
		r.Col(4, func(c *template.ColBuilder) {
			c.Text(centerLabel, template.FontSize(6.5), template.TextColor(slate500), template.AlignCenter(), template.LetterSpacing(0.6))
			c.Text(centerValue, template.FontSize(7.6), template.TextColor(slate900), template.AlignCenter())
		})
		r.Col(4, func(c *template.ColBuilder) {
			c.Text(rightLabel, template.FontSize(6.5), template.TextColor(slate500), template.AlignRight(), template.LetterSpacing(0.6))
			c.Text(rightValue, template.FontSize(7.6), template.Bold(), template.FontFamily("DejaVu-Bold"), template.TextColor(navy), template.AlignRight())
		})
	})
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Spacer(document.Mm(1.8))
			c.Line(template.LineColor(lineSoft), template.LineThickness(document.Pt(0.4)))
		})
	})
}

// drawClienteFormal: tarjeta con header navy y campos en grilla.
func drawClienteFormal(page *template.PageBuilder, inv *models.Invoice) {
	customer := inv.Customer
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) { c.Spacer(document.Mm(2.2)) })
	})
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			// header
			c.Box(func(c *template.ColBuilder) {
				c.Text("DATOS DEL CLIENTE  ·  RECEPTOR",
					template.FontSize(7), template.Bold(), template.FontFamily("DejaVu-Bold"),
					template.TextColor(pdf.White), template.AlignLeft(), template.LetterSpacing(0.7))
			}, template.WithBoxBackground(bgHeader), template.WithBoxPadding(document.Edges{Top: document.Mm(2), Right: document.Mm(3), Bottom: document.Mm(2), Left: document.Mm(3)}))
			// body
			c.Box(func(c *template.ColBuilder) {
				// Nombre / Razón social - ocupa todo
				c.Text("NOMBRE / RAZÓN SOCIAL", template.FontSize(6.3), template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.5))
				c.Text(customer.Name, template.FontSize(8.4), template.Bold(), template.FontFamily("DejaVu-Bold"), template.TextColor(slate900), template.AlignLeft())
				c.Spacer(document.Mm(2))
				// segunda fila: documento + complemento + código cliente
				c.Text(fmt.Sprintf("DOCUMENTO  (%s)", customer.DocumentType),
					template.FontSize(6.3), template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.5))
				docVal := customer.DocumentNumber
				if customer.Complement != nil && strings.TrimSpace(*customer.Complement) != "" {
					docVal += "  " + strings.TrimSpace(*customer.Complement)
				}
				c.Text(docVal, template.FontSize(7.8), template.TextColor(slate900), template.AlignLeft())
			},
				template.WithBoxBackground(pdf.White),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.4)), template.BorderColor(lineSoft))),
				template.WithBoxPadding(document.UniformEdges(document.Mm(3.2))),
			)
		})
	})
	// documento / email / código en grilla secundaria si existen
	hasExtra := false
	var extraLabel, extraValue string
	if customer.Email != nil && strings.TrimSpace(*customer.Email) != "" {
		hasExtra = true
		extraLabel = "CORREO ELECTRÓNICO"
		extraValue = strings.TrimSpace(*customer.Email)
	} else if strings.TrimSpace(customer.CodigoCliente) != "" {
		hasExtra = true
		extraLabel = "CÓDIGO CLIENTE"
		extraValue = strings.TrimSpace(customer.CodigoCliente)
	}
	if hasExtra {
		page.AutoRow(func(r *template.RowBuilder) {
			r.Col(6, func(c *template.ColBuilder) {
				c.Box(func(c *template.ColBuilder) {
					c.Text(extraLabel, template.FontSize(6.3), template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.5))
					c.Text(extraValue, template.FontSize(7.5), template.TextColor(slate700), template.AlignLeft())
				}, template.WithBoxBackground(bgCanvas), template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.35)), template.BorderColor(lineSoft))), template.WithBoxPadding(document.UniformEdges(document.Mm(2.5))))
			})
			r.Col(6, func(c *template.ColBuilder) {
				// columna vacía para balance, o mostrar CodigoCliente si no se mostró antes
				if extraLabel == "CORREO ELECTRÓNICO" && strings.TrimSpace(customer.CodigoCliente) != "" {
					c.Box(func(c *template.ColBuilder) {
						c.Text("CÓDIGO CLIENTE", template.FontSize(6.3), template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.5))
						c.Text(strings.TrimSpace(customer.CodigoCliente), template.FontSize(7.5), template.TextColor(slate700), template.AlignLeft())
					}, template.WithBoxBackground(bgCanvas), template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.35)), template.BorderColor(lineSoft))), template.WithBoxPadding(document.UniformEdges(document.Mm(2.5))))
				}
			})
		})
	}
}

// drawDetalleFormal: tabla elegante con cabecera navy y filas con borde sutil.
func drawDetalleFormal(page *template.PageBuilder, inv *models.Invoice) {
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) { c.Spacer(document.Mm(2.8)) })
	})
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Text("DETALLE DE LA TRANSACCIÓN",
				template.FontSize(7), template.Bold(), template.FontFamily("DejaVu-Bold"),
				template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.7))
			c.Spacer(document.Mm(1))
			c.Line(template.LineColor(navy), template.LineThickness(document.Pt(0.6)))
			c.Spacer(document.Mm(2))
		})
	})

	header := []string{"CÓD.", "DESCRIPCIÓN", "CANT.", "P. UNIT.", "SUBTOTAL"}
	rows := make([][]string, 0, len(inv.Items))
	for _, it := range inv.Items {
		rows = append(rows, []string{
			it.Code,
			it.Description,
			fmt.Sprintf("%.2f", it.Quantity),
			fmt.Sprintf("%.2f", it.UnitPrice),
			fmt.Sprintf("%.2f", it.Subtotal),
		})
	}
	if len(rows) == 0 {
		rows = [][]string{{"—", "Sin ítems", "—", "—", "—"}}
	}

	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Table(header, rows,
				template.ColumnWidths(11, 52, 11, 13, 13),
				template.ColumnAlign(document.AlignCenter, document.AlignLeft, document.AlignCenter, document.AlignRight, document.AlignRight),
				template.TableHeaderStyle(
					template.FontSize(7), template.Bold(), template.FontFamily("DejaVu-Bold"),
					template.TextColor(pdf.White), template.BgColor(navy),
					template.AlignCenter(), template.LetterSpacing(0.4),
				),
				template.TableStripe(bgStripe),
				template.WithTableBorder(template.Border(template.BorderWidth(document.Pt(0.5)), template.BorderColor(navy))),
				template.WithTableCellBorder(template.Border(template.BorderWidth(document.Pt(0.3)), template.BorderColor(lineSoft))),
				template.TableCellVAlign(document.VAlignMiddle),
			)
		})
	})
}

// drawTotalesFormal: tarjeta alineada a la derecha (offset 5 + 7) con total destacado.
func drawTotalesFormal(page *template.PageBuilder, inv *models.Invoice) {
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) { c.Spacer(document.Mm(3)) })
	})

	// Etiquetas formales
	type tot struct {
		label string
		value float64
		kind  string // "sub" | "desc" | "total"
	}
	tots := []tot{
		{"Subtotal", inv.Subtotal, "sub"},
		{"Descuento", inv.Discount, "desc"},
		{"TOTAL", inv.Total, "total"},
	}

	// Contenedor alineado a la derecha: dejamos 5 cols vacías
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(5, func(c *template.ColBuilder) {})
		r.Col(7, func(c *template.ColBuilder) {
			c.Box(func(c *template.ColBuilder) {
				for i, t := range tots {
					isTotal := t.kind == "total"
					c.Spacer(document.Mm(0.6))
					// fila interna 6 / 6 dentro de la caja
					// usamos RichText simulado con dos Text en columnas internas via Box anidado es complejo,
					// optamos por una sola línea con tabulación y alineación: label izquierda, valor derecha
					// Para lograrlo sin nesting, usamos una tabla minimal de 1 fila dentro del Box:
					// pero lo más simple y compatible es usar dos Text con alineaciones opuestas en la misma línea
					// usando un truco: un AutoRow no disponible dentro de Col, entonces renderizamos con Table de 1 fila sin header
					// Alternativa pragmática: Text combinado con espaciado
					if isTotal {
						c.Box(func(c *template.ColBuilder) {
							c.Text(t.label, template.FontSize(8.5), template.Bold(), template.FontFamily("DejaVu-Bold"), template.TextColor(pdf.White), template.AlignLeft())
							c.Text(fmt.Sprintf("Bs  %s", formatMoney(t.value)), template.FontSize(10), template.Bold(), template.FontFamily("DejaVu-Bold"), template.TextColor(pdf.White), template.AlignRight())
							c.Text(fmt.Sprintf("Moneda: %s", currencyText(inv.CodigoMoneda)), template.FontSize(6.5), template.TextColor(pdf.RGBHex(0xCBD5E1)), template.AlignRight())
						}, template.WithBoxBackground(navy), template.WithBoxPadding(document.Edges{Top: document.Mm(2.5), Right: document.Mm(3), Bottom: document.Mm(2.5), Left: document.Mm(3)}))
					} else {
						// fila clara
						c.Box(func(c *template.ColBuilder) {
							// label
							c.Text(t.label, template.FontSize(7.4), template.TextColor(slate700), template.AlignLeft())
							// value alineado a la derecha en siguiente línea para simplicidad formal
							c.Text(fmt.Sprintf("Bs  %s", formatMoney(t.value)), template.FontSize(7.6), template.TextColor(slate900), template.AlignRight())
						}, template.WithBoxBackground(pdf.White), template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.35)), template.BorderColor(lineSoft))), template.WithBoxPadding(document.Edges{Top: document.Mm(1.8), Right: document.Mm(3), Bottom: document.Mm(1.8), Left: document.Mm(3)}))
						if i < len(tots)-1 {
							c.Spacer(document.Mm(1))
						}
					}
				}
			},
				template.WithBoxBackground(bgCanvas),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.4)), template.BorderColor(lineSoft))),
				template.WithBoxPadding(document.UniformEdges(document.Mm(2.2))),
			)
		})
	})
}

// drawLeyendaFormal: bloque con borde izquierdo azul y tipografía pequeña.
func drawLeyendaFormal(page *template.PageBuilder, inv *models.Invoice) {
	if strings.TrimSpace(inv.Company.PiePagina) == "" {
		return
	}
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) { c.Spacer(document.Mm(3)) })
	})
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Box(func(c *template.ColBuilder) {
				c.Text("LEYENDA  ·  INFORMACIÓN LEGAL",
					template.FontSize(6.4), template.Bold(), template.FontFamily("DejaVu-Bold"),
					template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.6))
				c.Spacer(document.Mm(1.2))
				c.Text(strings.TrimSpace(inv.Company.PiePagina),
					template.FontSize(6.9), template.TextColor(slate700), template.AlignLeft())
			},
				template.WithBoxBackground(pdf.White),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.4)), template.BorderColor(lineSoft))),
				template.WithBoxPadding(document.UniformEdges(document.Mm(3))),
			)
		})
	})
}

// drawQRFormal: tarjeta inferior con QR enmarcado + nota de verificación.
func drawQRFormal(page *template.PageBuilder, inv *models.Invoice, qrFallback []byte) {
	cuf := cufText(inv)
	hasQRString := cuf != "" && cuf != "PENDIENTE DE EMISION"
	hasFallback := len(qrFallback) > 0
	if !hasQRString && !hasFallback {
		return
	}

	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) { c.Spacer(document.Mm(3.5)) })
	})
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Box(func(c *template.ColBuilder) {
				// fila interna: QR a la izquierda, texto a la derecha via columnas anidadas simuladas con Box
				// gpdf no permite Row dentro de Col, por lo que usamos dos AutoRow secuenciales dentro del Box no es posible.
				// Solución: renderizamos QR y texto en dos columnas de la fila exterior pero visualmente dentro de la tarjeta
				// usando el truco de dibujar la tarjeta como contenedor exterior y luego una fila exterior con QR+texto
				// Aquí dentro de la Box solo dejamos el texto; el QR se dibuja fuera. Para mantener封装, hacemos layout externo:
				c.Text("VERIFICACIÓN SIAT",
					template.FontSize(6.5), template.Bold(), template.FontFamily("DejaVu-Bold"),
					template.TextColor(navy), template.AlignLeft(), template.LetterSpacing(0.6))
				c.Spacer(document.Mm(1))
				c.Text("Escaneá este código QR para verificar la autenticidad de la factura en el portal oficial del Servicio de Impuestos Nacionales. Este documento es la representación gráfica de un Documento Fiscal Electrónico.",
					template.FontSize(6.7), template.TextColor(slate700), template.AlignLeft())
				c.Spacer(document.Mm(1))
				c.Text("Conservar este documento como respaldo. La validez legal está sujeta a la verificación del CUF y Código de Control ante el SIAT.",
					template.FontSize(6.3), template.TextColor(slate500), template.AlignLeft())
			},
				template.WithBoxBackground(bgCanvas),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.4)), template.BorderColor(lineSoft))),
				template.WithBoxPadding(document.UniformEdges(document.Mm(3))),
			)
		})
	})
	// Fila QR + tarjeta: como gpdf no permite Row dentro de Box, lo hacemos como fila siguiente con QR en col 3 y texto ya arriba.
	// Para un acabado formal con QR enmarcado, lo dibujamos en una fila dedicada debajo con marco.
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(3, func(c *template.ColBuilder) {
			c.Box(func(c *template.ColBuilder) {
				if hasQRString {
					c.QRCode(cuf, template.QRSize(document.Mm(26)))
				} else if hasFallback {
					c.Image(qrFallback, template.FitWidth(document.Mm(26)))
				}
			},
				template.WithBoxBackground(pdf.White),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.5)), template.BorderColor(navyLight))),
				template.WithBoxPadding(document.UniformEdges(document.Mm(2))),
			)
			c.Spacer(document.Mm(1))
			c.Text("CUF · QR", template.FontSize(6), template.TextColor(slate400), template.AlignCenter(), template.LetterSpacing(0.5))
		})
		r.Col(9, func(c *template.ColBuilder) {
			// Columna de cortesía: datos de emisión resumidos para archivo
			c.Box(func(c *template.ColBuilder) {
				c.Text("RESUMEN DE EMISIÓN",
					template.FontSize(6.4), template.Bold(), template.FontFamily("DejaVu-Bold"),
					template.TextColor(slate500), template.AlignLeft(), template.LetterSpacing(0.5))
				c.Spacer(document.Mm(1))
				c.Text(fmt.Sprintf("CUF: %s", truncate(cuf, 42)),
					template.FontSize(6.5), template.TextColor(slate700), template.AlignLeft())
				if inv.CufdRecord.CodigoControl != "" {
					c.Text(fmt.Sprintf("Control: %s  ·  CUFD: %s", inv.CufdRecord.CodigoControl, truncate(inv.CufdRecord.Cufd, 18)),
						template.FontSize(6.5), template.TextColor(slate700), template.AlignLeft())
				}
				c.Spacer(document.Mm(1))
				c.Text(fmt.Sprintf("Documento Sector %d  ·  Método Pago %d  ·  %s",
					inv.CodigoDocumentoSector, inv.CodigoMetodoPago, emissionLabel(inv.EmissionType)),
					template.FontSize(6.3), template.TextColor(slate500), template.AlignLeft())
			},
				template.WithBoxBackground(pdf.White),
				template.WithBoxBorder(template.Border(template.BorderWidth(document.Pt(0.35)), template.BorderColor(lineSoft))),
				template.WithBoxPadding(document.UniformEdges(document.Mm(3))),
			)
		})
	})
}

func drawFooter(page *template.PageBuilder, inv *models.Invoice) {
	page.AutoRow(func(r *template.RowBuilder) {
		r.Col(12, func(c *template.ColBuilder) {
			c.Spacer(document.Mm(4))
			c.Line(template.LineColor(lineSoft), template.LineThickness(document.Pt(0.4)))
			c.Spacer(document.Mm(1.5))
			c.Text(fmt.Sprintf("Documento generado por %s  ·  NIT %s  ·  %s  ·  Gracias por su preferencia",
				inv.Company.BusinessName, inv.Company.Nit, inv.Company.Municipio),
				template.FontSize(6.2), template.TextColor(slate400), template.AlignCenter())
			c.Text("ORIGINAL  ·  CLIENTE",
				template.FontSize(6), template.TextColor(slate400), template.AlignCenter(), template.LetterSpacing(0.8))
		})
	})
}

// ── helpers dominio ────────────────────────────────────────────────────────

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

func emissionLabel(e models.EmissionType) string {
	switch e {
	case models.EmissionEnLinea:
		return "En Línea"
	case models.EmissionOffline:
		return "Fuera de Línea"
	case models.EmissionContingencia:
		return "Contingencia"
	default:
		return string(e)
	}
}

func formatMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// DecodeQR extrae la imagen PNG (base64) del código QR del CUFD.
func DecodeQR(codigoQR *string) []byte {
	if codigoQR == nil || strings.TrimSpace(*codigoQR) == "" {
		return nil
	}
	data := strings.TrimSpace(*codigoQR)
	data = strings.TrimPrefix(data, "data:image/png;base64,")
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil
	}
	return decoded
}
