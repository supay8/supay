package invoice

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/adapters/siat"
)

// invoiceCustomerDTO es la representación mínima del cliente en una factura.
type invoiceCustomerDTO struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	DocumentType   string  `json:"document_type"`
	DocumentNumber string  `json:"document_number"`
	Complement     *string `json:"complement,omitempty"`
}

// invoiceItemDTO es la representación de un ítem de factura.
type invoiceItemDTO struct {
	ID                string          `json:"id"`
	ProductID         *string         `json:"product_id,omitempty"`
	Code              string          `json:"code"`
	Description       string          `json:"description"`
	CodigoActividad   *string         `json:"codigo_actividad,omitempty"`
	CodigoProductoSin *string         `json:"codigo_producto_sin,omitempty"`
	UnitCode          *int            `json:"unit_code,omitempty"`
	Quantity          float64         `json:"quantity"`
	UnitPrice         float64         `json:"unit_price"`
	Discount          float64         `json:"discount"`
	Subtotal          float64         `json:"subtotal"`
	SectorData        json.RawMessage `json:"sector_data,omitempty"`
}

// invoiceDTO es la respuesta pública por defecto de los endpoints de facturas.
// Omite campos pesados (XML, entidades completas) a menos que se pidan con
// ?include=xml,company,point_of_sale,cufd,customer,archivo.
type invoiceDTO struct {
	ID                    string               `json:"id"`
	CompanyId             string               `json:"company_id"`
	CustomerId            string               `json:"customer_id"`
	PointOfSaleId         string               `json:"point_of_sale_id"`
	IdempotencyKey        *string              `json:"idempotency_key,omitempty"`
	CufdId                string               `json:"cufd_id,omitempty"`
	ContingencyEventId    *string              `json:"contingency_event_id,omitempty"`
	AjustaFacturaId       *string              `json:"ajusta_factura_id,omitempty"`
	InvoiceNumber         int                  `json:"invoice_number"`
	Status                domain.InvoiceStatus `json:"status"`
	Cuf                   *string              `json:"cuf,omitempty"`
	Subtotal              float64              `json:"subtotal"`
	Discount              float64              `json:"discount"`
	Total                 float64              `json:"total"`
	CodigoMetodoPago      int                  `json:"codigo_metodo_pago"`
	CodigoMoneda          int                  `json:"codigo_moneda"`
	TipoCambio            float64              `json:"tipo_cambio"`
	CodigoDocumentoSector int                  `json:"codigo_documento_sector"`
	CodigoTipoFactura     int                  `json:"codigo_tipo_factura"`
	Layout                string               `json:"layout,omitempty"`
	Modalidad             int                  `json:"modalidad"`
	NombreEstudiante      *string              `json:"nombre_estudiante,omitempty"`
	PeriodoFacturado      *string              `json:"periodo_facturado,omitempty"`
	SectorData            json.RawMessage      `json:"sector_data,omitempty"`
	EmissionType          string               `json:"emission_type"`
	IssueDate             time.Time            `json:"issue_date"`
	MotivoAnulacion       *int                 `json:"motivo_anulacion,omitempty"`
	FechaAnulacion        *time.Time           `json:"fecha_anulacion,omitempty"`
	SiatReceptionCode     *string              `json:"siat_reception_code,omitempty"`
	SiatMensajes          []siat.Mensaje       `json:"siat_mensajes,omitempty"`
	Customer              *invoiceCustomerDTO  `json:"customer"`
	Items                 []invoiceItemDTO     `json:"items"`
	CreatedAt             time.Time            `json:"created_at"`

	// Campos opt-in por ?include=
	Company     *domain.Company     `json:"company,omitempty"`
	PointOfSale *domain.PointOfSale `json:"point_of_sale,omitempty"`
	CufdRecord  *domain.Cufd        `json:"cufd_record,omitempty"`
	Xml         *string             `json:"xml,omitempty"`
	XmlHash     *string             `json:"xml_hash,omitempty"`
	Archivo     string              `json:"archivo,omitempty"`
	HashArchivo string              `json:"hash_archivo,omitempty"`
}

// parseIncludes convierte "xml,company" en un set de includes.
func parseIncludes(raw string) map[string]bool {
	includes := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		includes[part] = true
		if part == "all" {
			includes["xml"] = true
			includes["archivo"] = true
			includes["company"] = true
			includes["point_of_sale"] = true
			includes["cufd"] = true
		}
	}
	return includes
}

func toInvoiceItemDTO(it domain.InvoiceItem) invoiceItemDTO {
	return invoiceItemDTO{
		ID:                it.ID,
		ProductID:         it.ProductID,
		Code:              it.Code,
		Description:       it.Description,
		CodigoActividad:   it.CodigoActividad,
		CodigoProductoSin: it.CodigoProductoSin,
		UnitCode:          it.UnitCode,
		Quantity:          it.Quantity,
		UnitPrice:         it.UnitPrice,
		Discount:          it.Discount,
		Subtotal:          it.Subtotal,
		SectorData:        it.SectorData,
	}
}

func toInvoiceDTO(inv *domain.Invoice, includes map[string]bool) invoiceDTO {
	if includes == nil {
		includes = map[string]bool{}
	}

	// El cliente siempre está presente: es la única fuente de verdad de los
	// datos fiscales del receptor (el snapshot receiver fue eliminado).
	customerDTO := &invoiceCustomerDTO{
		ID:             inv.Customer.ID,
		Name:           inv.Customer.Name,
		DocumentType:   inv.Customer.DocumentType,
		DocumentNumber: inv.Customer.DocumentNumber,
		Complement:     inv.Customer.Complement,
	}

	dto := invoiceDTO{
		ID:                    inv.ID,
		CompanyId:             inv.CompanyId,
		CustomerId:            inv.CustomerId,
		PointOfSaleId:         inv.PointOfSaleId,
		IdempotencyKey:        inv.IdempotencyKey,
		CufdId:                inv.CufdId,
		ContingencyEventId:    inv.ContingencyEventId,
		AjustaFacturaId:       inv.AjustaFacturaId,
		InvoiceNumber:         inv.InvoiceNumber,
		Status:                inv.Status,
		Cuf:                   inv.Cuf,
		Subtotal:              inv.Subtotal,
		Discount:              inv.Discount,
		Total:                 inv.Total,
		CodigoMetodoPago:      inv.CodigoMetodoPago,
		CodigoMoneda:          inv.CodigoMoneda,
		TipoCambio:            inv.TipoCambio,
		CodigoDocumentoSector: inv.CodigoDocumentoSector,
		CodigoTipoFactura:     inv.CodigoTipoFactura,
		Layout:                inv.Layout,
		Modalidad:             inv.Modalidad,
		NombreEstudiante:      inv.NombreEstudiante,
		PeriodoFacturado:      inv.PeriodoFacturado,
		SectorData:            inv.SectorData,
		EmissionType:          inv.EmissionType,
		IssueDate:             inv.IssueDate,
		MotivoAnulacion:       inv.MotivoAnulacion,
		FechaAnulacion:        inv.FechaAnulacion,
		SiatReceptionCode:     inv.SiatReceptionCode,
		CreatedAt:             inv.CreatedAt,
		Customer:              customerDTO,
		Items:                 make([]invoiceItemDTO, 0, len(inv.Items)),
	}

	if inv.SiatMensajes != nil && *inv.SiatMensajes != "" {
		var msgs []siat.Mensaje
		if err := json.Unmarshal([]byte(*inv.SiatMensajes), &msgs); err == nil {
			dto.SiatMensajes = msgs
		}
	}

	for _, it := range inv.Items {
		dto.Items = append(dto.Items, toInvoiceItemDTO(it))
	}

	if includes["company"] {
		dto.Company = &inv.Company
	}
	if includes["point_of_sale"] {
		dto.PointOfSale = &inv.PointOfSale
	}
	if includes["cufd"] {
		dto.CufdRecord = &inv.CufdRecord
	}
	if includes["xml"] {
		dto.Xml = inv.Xml
		dto.XmlHash = inv.XmlHash
	}
	if includes["archivo"] {
		dto.Archivo = inv.Archivo
		dto.HashArchivo = inv.HashArchivo
	}

	return dto
}

// sectorCampoDTO describe un campo datos_sector.
type sectorCampoDTO struct {
	JSON      string `json:"json"`
	Requerido bool   `json:"requerido"`
	Tipo      string `json:"tipo"`
	Etiqueta  string `json:"etiqueta,omitempty"`
	Ejemplo   string `json:"ejemplo,omitempty"`
}

// sectorDTO es la representación de un perfil de documento-sector.
type sectorDTO struct {
	Codigo             int    `json:"codigo"`
	Nombre             string `json:"nombre"`
	TipoDocumento      int    `json:"tipo_documento"`
	Operacion          string `json:"operacion"`
	Fachada            string `json:"fachada"`
	Layout             string `json:"layout,omitempty"`
	Modalidades        []int  `json:"modalidades,omitempty"`
	Soportado          bool   `json:"soportado"`
	TieneBuilder       bool   `json:"tiene_builder"`
	RequiereArchivo    bool   `json:"requiere_archivo"`
	ConDetalle         bool   `json:"con_detalle"`
	DetalleUnico       bool   `json:"detalle_unico"`
	MontoSujetoIvaCero bool   `json:"monto_sujeto_iva_cero"`
	Ajuste             bool   `json:"es_ajuste"`
	// Habilitado indica si la empresa (company_id) puede emitir el sector
	// según su catálogo sincronizado actividadesDocumentoSector. nil cuando
	// la consulta no filtra por empresa.
	Habilitado    *bool            `json:"habilitado,omitempty"`
	Campos        []sectorCampoDTO `json:"campos_datos_sector"`
	CamposDetalle []sectorCampoDTO `json:"campos_datos_sector_detalle,omitempty"`
}

// humanizarClave convierte una clave snake_case en una etiqueta legible
// (periodo_facturado → "Periodo Facturado").
func humanizarClave(clave string) string {
	words := strings.Split(clave, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func aSectorCampoDTO(c siat.CampoSector) sectorCampoDTO {
	dto := sectorCampoDTO{
		JSON:      c.JSON,
		Requerido: c.Requerido,
		Tipo:      c.Tipo,
		Etiqueta:  c.Etiqueta,
		Ejemplo:   c.Ejemplo,
	}
	if dto.Etiqueta == "" {
		dto.Etiqueta = humanizarClave(c.JSON)
	}
	return dto
}
