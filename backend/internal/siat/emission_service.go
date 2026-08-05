package siat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

type EmissionService struct {
	db     *gorm.DB
	client *Client
	logger *slog.Logger
}

func NewEmissionService(db *gorm.DB, client *Client, logger *slog.Logger) *EmissionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &EmissionService{db: db, client: client, logger: logger}
}

// Emit builds a minimal XML for the invoice, calculates its SHA256 and persists it.
// This is a first-pass implementation: signing and real SOAP "recepcion" are TODOs.
func (s *EmissionService) Emit(ctx context.Context, invoiceID string) (*models.Invoice, error) {
	var inv models.Invoice
	if err := s.db.Preload("Items").Preload("PointOfSale").Preload("Company").Preload("Customer").Preload("CufdRecord").First(&inv, "id = ?", invoiceID).Error; err != nil {
		return nil, fmt.Errorf("emit: invoice not found: %w", err)
	}

	// Ensure we have a CUFD. If not, try to find active CUFD for point of sale
	now := time.Now().UTC()
	if inv.CufdId == "" {
		var cufd models.Cufd
		if err := s.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ?", inv.PointOfSaleId, now, now).Order("valid_from desc").First(&cufd).Error; err == nil {
			inv.CufdId = cufd.ID
			inv.Cuf = &cufd.Cufd
		}
	}

	// Build a very small XML representation (not full SIAT XSD)
	type xmlItem struct {
		Code        string  `xml:"code"`
		Description string  `xml:"description"`
		Quantity    float64 `xml:"quantity"`
		UnitPrice   float64 `xml:"unitPrice"`
		Subtotal    float64 `xml:"subtotal"`
	}
	type xmlInvoice struct {
		XMLName       xml.Name  `xml:"Invoice"`
		CompanyNit    string    `xml:"companyNit"`
		InvoiceNumber int       `xml:"invoiceNumber"`
		IssueDate     string    `xml:"issueDate"`
		Total         float64   `xml:"total"`
		Cuf           *string   `xml:"cuf,omitempty"`
		Items         []xmlItem `xml:"items>item"`
	}

	xmlInv := xmlInvoice{
		CompanyNit:    inv.Company.Nit,
		InvoiceNumber: inv.InvoiceNumber,
		IssueDate:     inv.IssueDate.UTC().Format(time.RFC3339),
		Total:         inv.Total,
		Cuf:           inv.Cuf,
	}
	for _, it := range inv.Items {
		xmlInv.Items = append(xmlInv.Items, xmlItem{
			Code:        it.Code,
			Description: it.Description,
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
			Subtotal:    it.Subtotal,
		})
	}

	payload, err := xml.MarshalIndent(xmlInv, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("emit: marshal xml: %w", err)
	}

	// Calculate SHA256
	hash := sha256.Sum256(payload)
	hashHex := hex.EncodeToString(hash[:])

	xmlStr := string(payload)
	inv.Xml = &xmlStr
	inv.XmlHash = &hashHex
	inv.Status = models.StatusSent

	if err := s.db.Save(&inv).Error; err != nil {
		return nil, fmt.Errorf("emit: save invoice: %w", err)
	}

	// Record event
	eventPayload, _ := mapToJSON(map[string]any{"xml_hash": hashHex, "length": len(payload)})
	s.db.Create(&models.InvoiceEvent{
		InvoiceId: inv.ID,
		Type:      "EMIT_GENERATE_XML",
		Message:   "XML generado y guardado",
		Payload:   eventPayload,
		CreatedAt: time.Now().UTC(),
	})

	if s.logger != nil {
		s.logger.Info("emit: invoice emitted (local)", "invoice_id", inv.ID, "xml_hash", hashHex)
	}

	// TODO: firmar XML (XMLDSig) y enviar a SIAT vía SOAP (operación recepcionFactura)

	// Enviar a SIAT (sin firma si no hay certificado configurado)
	resp, err := s.sendToSiat(ctx, inv)
	if err != nil {
		// registrar evento de fallo de envío pero no fallar el flujo
		s.db.Create(&models.InvoiceEvent{
			InvoiceId: inv.ID,
			Type:      "EMIT_SEND_FAIL",
			Message:   fmt.Sprintf("envío a SIAT falló: %v", err),
			Payload:   mustJSON(map[string]any{"error": err.Error()}),
			CreatedAt: time.Now().UTC(),
		})
		return &inv, fmt.Errorf("emit: envío a SIAT: %w", err)
	}

	// Guardar código de recepción/respuesta en la factura
	code := extractSimpleResponseCode(resp)
	inv.SiatReceptionCode = &code
	if err := s.db.Save(&inv).Error; err != nil {
		return &inv, fmt.Errorf("emit: guardar respuesta siat: %w", err)
	}

	s.db.Create(&models.InvoiceEvent{
		InvoiceId: inv.ID,
		Type:      "EMIT_SEND_OK",
		Message:   "Factura enviada a SIAT (respuesta almacenada)",
		Payload:   mustJSON(map[string]any{"siat_response": string(resp)}),
		CreatedAt: time.Now().UTC(),
	})

	return &inv, nil
}

// sendToSiat: realiza un envío SOAP sencillo a la operación recepcionFactura.
// Nota: esta implementación envía el XML generado sin firma. Para producción
// se debe firmar con XMLDSig según la modalidad Electrónica.
func (s *EmissionService) sendToSiat(ctx context.Context, inv models.Invoice) ([]byte, error) {
	if s.client == nil {
		return nil, fmt.Errorf("siat client no inicializado")
	}

	// Envelope mínimo
	type recepcionSOAPRequestEnvelope struct {
		XMLName  xml.Name `xml:"soapenv:Envelope"`
		XmlnsSo  string   `xml:"xmlns:soapenv,attr"`
		XmlnsXsd string   `xml:"xmlns:xsd,attr,omitempty"`
		XmlnsXsi string   `xml:"xmlns:xsi,attr,omitempty"`
		Body     struct {
			Request struct {
				Factura string `xml:"factura"`
			} `xml:"recepcionFactura"`
		} `xml:"soapenv:Body"`
	}

	envelope := recepcionSOAPRequestEnvelope{
		XmlnsSo:  soapEnvelopeNamespace,
		XmlnsXsd: "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi: "http://www.w3.org/2001/XMLSchema-instance",
	}
	if inv.Xml != nil {
		envelope.Body.Request.Factura = *inv.Xml
	} else {
		envelope.Body.Request.Factura = ""
	}

	raw, err := s.client.Do(ctx, "recepcionFactura", envelope)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func extractSimpleResponseCode(raw []byte) string {
	// Intenta extraer una etiqueta <codigo> o devuelve el primer segmento
	s := string(raw)
	start := strings.Index(s, "<codigo>")
	if start >= 0 {
		end := strings.Index(s[start:], "</codigo>")
		if end > 0 {
			return s[start+8 : start+end]
		}
	}
	// Fallback: retornar fragmento inicial
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func mapToJSON(m map[string]any) ([]byte, error) {
	return json.Marshal(m)
}
