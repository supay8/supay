package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

// MinimalInvoiceRequest is the public v1 contract. Additional options are
// optional so existing six-field clients remain compatible.
type MinimalInvoiceRequest struct {
	PointOfSaleID      string                 `json:"point_of_sale_id"`
	Customer           MinimalInvoiceCustomer `json:"customer"`
	Items              []MinimalInvoiceItem   `json:"items"`
	InvoiceType        string                 `json:"invoice_type,omitempty"`
	Sector             string                 `json:"sector,omitempty"`
	Data               json.RawMessage        `json:"data,omitempty"`
	Layout             string                 `json:"layout,omitempty"`
	ReferenceInvoiceID *string                `json:"reference_invoice_id,omitempty"`
	Payment            *MinimalInvoicePayment `json:"payment,omitempty"`
	Total              *float64               `json:"total,omitempty"`
}

type MinimalInvoicePayment struct {
	MethodCode   int     `json:"method_code"`
	CurrencyCode int     `json:"currency_code"`
	ExchangeRate float64 `json:"exchange_rate"`
}

type MinimalInvoiceCustomer struct {
	ID             string  `json:"id,omitempty"`
	DocumentType   string  `json:"document_type,omitempty"`
	DocumentNumber string  `json:"document_number,omitempty"`
	Complement     *string `json:"complement,omitempty"`
	Name           string  `json:"name,omitempty"`
	Email          string  `json:"email,omitempty"`
}

type MinimalInvoiceItem struct {
	SKU      string          `json:"sku"`
	Quantity float64         `json:"quantity"`
	Price    float64         `json:"price"`
	Discount float64         `json:"discount,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// InvoicePreview exposes the fields inferred by the API without persisting a
// customer or an invoice.
type InvoicePreview struct {
	PointOfSaleID         string                 `json:"point_of_sale_id"`
	Customer              InvoicePreviewCustomer `json:"customer"`
	Items                 []InvoicePreviewItem   `json:"items"`
	InvoiceType           string                 `json:"invoice_type"`
	CodigoDocumentoSector int                    `json:"codigo_documento_sector"`
	CodigoMetodoPago      int                    `json:"codigo_metodo_pago"`
	CodigoMoneda          int                    `json:"codigo_moneda"`
	TipoCambio            float64                `json:"tipo_cambio"`
	Subtotal              float64                `json:"subtotal"`
	Total                 float64                `json:"total"`
	Layout                string                 `json:"layout,omitempty"`
	ReferenceInvoiceID    *string                `json:"reference_invoice_id,omitempty"`
	Data                  json.RawMessage        `json:"data,omitempty"`
}

type InvoicePreviewCustomer struct {
	ID             string  `json:"id,omitempty"`
	DocumentType   string  `json:"document_type"`
	DocumentNumber string  `json:"document_number"`
	Complement     *string `json:"complement,omitempty"`
	Name           string  `json:"name"`
	Email          string  `json:"email,omitempty"`
}

type InvoicePreviewItem struct {
	ProductID             string  `json:"product_id"`
	SKU                   string  `json:"sku"`
	Description           string  `json:"description"`
	Quantity              float64 `json:"quantity"`
	UnitPrice             float64 `json:"unit_price"`
	Discount              float64 `json:"discount"`
	Subtotal              float64 `json:"subtotal"`
	CodigoActividad       string  `json:"codigo_actividad"`
	CodigoProductoSin     int64   `json:"codigo_producto_sin"`
	UnidadMedida          int     `json:"unidad_medida"`
	CodigoDocumentoSector int     `json:"codigo_documento_sector"`
}

type SimplifiedInvoice struct {
	Request CreateInvoiceRequest
	Preview InvoicePreview
}

// InvoiceRequestSimplifier translates the public six-field request into the
// complete internal request used by the existing invoice use case.
type InvoiceRequestSimplifier struct {
	uc *InvoiceUsecase
}

func NewInvoiceRequestSimplifier(uc *InvoiceUsecase) *InvoiceRequestSimplifier {
	return &InvoiceRequestSimplifier{uc: uc}
}

func (s *InvoiceRequestSimplifier) Simplify(ctx context.Context, input MinimalInvoiceRequest) (*SimplifiedInvoice, error) {
	if s == nil || s.uc == nil {
		return nil, domain.NewConflictError("el simplificador de facturas no está configurado")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pointOfSaleID := strings.TrimSpace(input.PointOfSaleID)
	if pointOfSaleID == "" {
		return nil, domain.NewBadRequestError("point_of_sale_id es obligatorio")
	}
	pos, err := s.uc.posRepo.GetByID(pointOfSaleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("punto de venta no encontrado")
		}
		return nil, err
	}
	if !pos.IsActive {
		return nil, domain.NewBadRequestError("el punto de venta está inactivo")
	}
	company, err := s.uc.companyRepo.GetByID(pos.CompanyId)
	if err != nil {
		return nil, domain.NewNotFoundError("empresa no encontrada")
	}
	if err := s.uc.ensureCatalogReadiness(company.ID, pos.ID); err != nil {
		return nil, err
	}

	invoiceType, err := normalizeInvoiceType(input.InvoiceType)
	if err != nil {
		return nil, err
	}
	sector, err := resolveSectorAlias(input.Sector)
	if err != nil {
		return nil, err
	}

	request := CreateInvoiceRequest{
		CompanyId:             company.ID,
		PointOfSaleId:         pos.ID,
		InvoiceType:           invoiceType,
		CodigoDocumentoSector: sector,
		CodigoMetodoPago:      1,
		CodigoMoneda:          1,
		TipoCambio:            1,
		DatosSector:           input.Data,
		Layout:                strings.TrimSpace(input.Layout),
		ReferenciaFacturaId:   input.ReferenceInvoiceID,
		Modalidad:             s.uc.effectiveModalidadForCompany(company),
		Total:                 input.Total,
	}
	if input.Payment != nil {
		if input.Payment.MethodCode <= 0 || input.Payment.CurrencyCode <= 0 || input.Payment.ExchangeRate <= 0 {
			return nil, domain.NewBadRequestError("payment requiere method_code, currency_code y exchange_rate mayores a cero")
		}
		request.CodigoMetodoPago = input.Payment.MethodCode
		request.CodigoMoneda = input.Payment.CurrencyCode
		request.TipoCambio = input.Payment.ExchangeRate
	}
	if sector == 0 && (invoiceType == "credit_note" || invoiceType == "debit_note") {
		request.CodigoDocumentoSector = siat.SectorNotaCreditoDebito
	}

	customer, err := s.resolveCustomer(company.ID, input.Customer, &request)
	if err != nil {
		return nil, err
	}
	if len(input.Items) == 0 && input.ReferenceInvoiceID == nil && input.Total == nil {
		return nil, domain.NewBadRequestError("items debe contener al menos un producto")
	}
	request.Items = make([]CreateInvoiceItemRequest, 0, len(input.Items))
	for index, item := range input.Items {
		if strings.TrimSpace(item.SKU) == "" {
			return nil, domain.NewBadRequestError(fmt.Sprintf("items[%d].sku es obligatorio", index))
		}
		if item.Quantity <= 0 {
			return nil, domain.NewBadRequestError(fmt.Sprintf("items[%d].quantity debe ser mayor a cero", index))
		}
		if item.Price < 0 || item.Discount < 0 || item.Discount > item.Quantity*item.Price {
			return nil, domain.NewBadRequestError(fmt.Sprintf("items[%d] tiene precio o descuento inválido", index))
		}
		request.Items = append(request.Items, CreateInvoiceItemRequest{
			SKU:        strings.TrimSpace(item.SKU),
			Quantity:   item.Quantity,
			UnitPrice:  item.Price,
			Discount:   item.Discount,
			SectorData: item.Data,
		})
	}

	if request.CodigoDocumentoSector > 0 {
		p, err := siat.PerfilSectorLayout(request.CodigoDocumentoSector, request.Layout)
		if err != nil {
			return nil, domain.NewBadRequestError(err.Error())
		}
		if !p.HasBuilder() {
			return nil, domain.NewBadRequestError("sector 33 no tiene constructor en go-siat; use /invoices con archivo, hash_archivo y cuf")
		}
		if err := p.ValidarModalidad(request.Modalidad); err != nil {
			return nil, domain.NewBadRequestError(err.Error())
		}
		if p.EsAjuste() && (request.ReferenciaFacturaId == nil || strings.TrimSpace(*request.ReferenciaFacturaId) == "") {
			return nil, domain.NewBadRequestError("reference_invoice_id es obligatorio para documentos de ajuste")
		}
	}
	reference, err := s.uc.autofillDocumentoAjusteDescuento(&request, company.ID)
	if err != nil {
		return nil, err
	}
	if reference != nil && reference.CustomerId != "" && reference.CustomerId != request.CustomerId {
		return nil, domain.NewBadRequestError("customer debe coincidir con el cliente de la factura referenciada")
	}
	mappings, products, _, _, err := s.uc.resolveProductMappings(request)
	if err != nil {
		return nil, err
	}
	if request.CodigoDocumentoSector == 0 {
		request.CodigoDocumentoSector, err = s.uc.resolveInvoiceSector(request.InvoiceType, mappingSectors(mappings), company)
		if err != nil {
			return nil, err
		}
		mappings, products, _, _, err = s.uc.resolveProductMappings(request)
		if err != nil {
			return nil, err
		}
	}

	profile, err := siat.PerfilSectorLayout(request.CodigoDocumentoSector, request.Layout)
	if err != nil {
		return nil, domain.NewBadRequestError(fmt.Sprintf("documento-sector %d no soportado: %v", request.CodigoDocumentoSector, err))
	}
	if err := profile.ValidarModalidad(s.uc.effectiveModalidadForCompany(company)); err != nil {
		return nil, domain.NewBadRequestError(err.Error())
	}
	if !profile.HasBuilder() {
		return nil, domain.NewBadRequestError("sector 33 no tiene constructor en go-siat; use /invoices con archivo, hash_archivo y cuf")
	}
	if profile.DetalleUnico && len(request.Items) != 1 {
		return nil, domain.NewBadRequestError(fmt.Sprintf("el sector %d requiere exactamente un ítem", profile.Codigo))
	}
	if len(request.Items) == 0 && profile.ConDetalle {
		return nil, domain.NewBadRequestError("items debe contener al menos un producto")
	}
	if input.Total != nil && (profile.ConDetalle || len(request.Items) > 0 || *input.Total < 0) {
		return nil, domain.NewBadRequestError("total solo se admite sin items en sectores sin detalle y debe ser no negativo")
	}
	if !profile.ConDetalle && len(request.Items) == 0 && input.Total == nil {
		return nil, domain.NewBadRequestError("total es obligatorio para documentos sin detalle")
	}
	if profile.EsAjuste() && (request.ReferenciaFacturaId == nil || strings.TrimSpace(*request.ReferenciaFacturaId) == "") {
		return nil, domain.NewBadRequestError("reference_invoice_id es obligatorio para documentos de ajuste")
	}
	if !profile.EsAjuste() && request.ReferenciaFacturaId != nil {
		return nil, domain.NewBadRequestError("reference_invoice_id solo corresponde a documentos de ajuste")
	}
	if !profile.EsAjuste() && (request.InvoiceType == "credit_note" || request.InvoiceType == "debit_note") {
		return nil, domain.NewBadRequestError("invoice_type de nota requiere un sector de documento de ajuste")
	}
	values, err := profile.PrepararDatosSector(siat.SolicitudFactura{DatosSector: request.DatosSector})
	if err != nil {
		return nil, domain.NewBadRequestError(err.Error())
	}
	request.DatosSector, err = json.Marshal(values)
	if err != nil {
		return nil, err
	}

	preview := InvoicePreview{
		PointOfSaleID:         pos.ID,
		Customer:              customer,
		InvoiceType:           request.InvoiceType,
		CodigoDocumentoSector: request.CodigoDocumentoSector,
		CodigoMetodoPago:      request.CodigoMetodoPago,
		CodigoMoneda:          request.CodigoMoneda,
		TipoCambio:            request.TipoCambio,
		Items:                 make([]InvoicePreviewItem, 0, len(request.Items)),
		Layout:                request.Layout,
		ReferenceInvoiceID:    request.ReferenciaFacturaId,
		Data:                  request.DatosSector,
	}
	for index := range request.Items {
		item := &request.Items[index]
		item.Quantity, item.UnitPrice, item.Discount = siat.NormalizarImportesItem(profile.Codigo, item.Quantity, item.UnitPrice, item.Discount)
		if item.Quantity <= 0 {
			return nil, domain.NewBadRequestError("quantity debe ser mayor a cero con la precisión del sector")
		}
		mapping, ok := mappings[index]
		if !ok {
			return nil, domain.NewConflictError(fmt.Sprintf("el producto del ítem %d no tiene mapeo fiscal", index+1))
		}
		product := products[index]
		if product == nil {
			return nil, domain.NewNotFoundError(fmt.Sprintf("el producto del ítem %d no existe", index+1))
		}
		if _, err := profile.ValidarDatosDetalle(request.Items[index].SectorData); err != nil {
			return nil, domain.NewBadRequestError(fmt.Sprintf("ítem %d: %v", index+1, err))
		}
		activity := mapping.CodigoActividad
		sinCode := strconv.FormatInt(mapping.CodigoProductoSin, 10)
		unit := mapping.UnidadMedida
		productID := product.ID
		request.Items[index].ProductID = product.ID
		request.Items[index].Code = product.SKU
		request.Items[index].Description = product.Name
		request.Items[index].CodigoActividad = &activity
		request.Items[index].CodigoProductoSin = &sinCode
		request.Items[index].UnitCode = &unit
		subtotal := round2(request.Items[index].Quantity*request.Items[index].UnitPrice - request.Items[index].Discount)
		preview.Subtotal += subtotal
		preview.Items = append(preview.Items, InvoicePreviewItem{
			ProductID:             productID,
			SKU:                   product.SKU,
			Description:           product.Name,
			Quantity:              request.Items[index].Quantity,
			UnitPrice:             request.Items[index].UnitPrice,
			Discount:              request.Items[index].Discount,
			Subtotal:              subtotal,
			CodigoActividad:       mapping.CodigoActividad,
			CodigoProductoSin:     mapping.CodigoProductoSin,
			UnidadMedida:          mapping.UnidadMedida,
			CodigoDocumentoSector: mapping.CodigoDocumentoSector,
		})
	}
	preview.Subtotal = round2(preview.Subtotal)
	preview.Total = preview.Subtotal
	if input.Total != nil {
		preview.Subtotal = round2(*input.Total)
		preview.Total = preview.Subtotal
	}
	preview.Total, err = siat.TotalDocumento(preview.Subtotal, request.DatosSector)
	if err != nil {
		return nil, domain.NewBadRequestError(err.Error())
	}

	return &SimplifiedInvoice{Request: request, Preview: preview}, nil
}

func (s *InvoiceRequestSimplifier) resolveCustomer(companyID string, input MinimalInvoiceCustomer, request *CreateInvoiceRequest) (InvoicePreviewCustomer, error) {
	if s.uc.customerRepo == nil {
		return InvoicePreviewCustomer{}, domain.NewConflictError("el catálogo de clientes no está configurado")
	}
	var customer *domain.Customer
	var err error
	if id := strings.TrimSpace(input.ID); id != "" {
		customer, err = s.uc.customerRepo.GetByID(id)
		if err != nil || customer == nil {
			return InvoicePreviewCustomer{}, domain.NewNotFoundError("cliente no encontrado")
		}
		if customer.CompanyId != companyID {
			return InvoicePreviewCustomer{}, domain.NewBadRequestError("el cliente no pertenece a la empresa")
		}
	} else {
		documentType, aliasErr := normalizeDocumentType(input.DocumentType)
		if aliasErr != nil {
			return InvoicePreviewCustomer{}, aliasErr
		}
		documentNumber := strings.TrimSpace(input.DocumentNumber)
		if documentNumber == "" {
			return InvoicePreviewCustomer{}, domain.NewBadRequestError("customer.document_number es obligatorio cuando no se envía customer.id")
		}
		customer, err = s.uc.customerRepo.GetByCompanyAndDocument(companyID, documentType, documentNumber)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return InvoicePreviewCustomer{}, err
		}
		if customer == nil {
			name := strings.TrimSpace(input.Name)
			if name == "" {
				return InvoicePreviewCustomer{}, domain.NewBadRequestError("customer.name es obligatorio para registrar un cliente nuevo")
			}
			request.ClientDocumentType = documentType
			request.ClientDocumentNumber = documentNumber
			request.ClientComplement = input.Complement
			request.ClientName = name
			request.ClientEmail = strings.TrimSpace(input.Email)
			return InvoicePreviewCustomer{DocumentType: documentType, DocumentNumber: documentNumber, Complement: input.Complement, Name: name, Email: request.ClientEmail}, nil
		}
	}

	request.CustomerId = customer.ID
	email := ""
	if customer.Email != nil {
		email = *customer.Email
	}
	return InvoicePreviewCustomer{
		ID: customer.ID, DocumentType: customer.DocumentType, DocumentNumber: customer.DocumentNumber,
		Complement: customer.Complement, Name: customer.Name, Email: email,
	}, nil
}

func normalizeDocumentType(value string) (string, error) {
	alias := normalizeAlias(value)
	if alias == "" {
		return "CI", nil
	}
	types := map[string]string{
		"1": "CI", "ci": "CI", "cedula": "CI", "cedula_identidad": "CI",
		"2": "CEX", "cex": "CEX", "extranjero": "CEX",
		"3": "PAS", "pas": "PAS", "pasaporte": "PAS",
		"4": "OD", "od": "OD", "otro": "OD",
		"5": "NIT", "nit": "NIT",
	}
	if result, ok := types[alias]; ok {
		return result, nil
	}
	return "", domain.NewBadRequestError(fmt.Sprintf("customer.document_type %q no soportado", value))
}

func normalizeInvoiceType(value string) (string, error) {
	alias := normalizeAlias(value)
	switch alias {
	case "", "sale", "venta", "compraventa", "compra_venta":
		return "sale", nil
	case "education", "educacion", "educativo":
		return "education", nil
	case "credit_note", "nota_credito":
		return "credit_note", nil
	case "debit_note", "nota_debito":
		return "debit_note", nil
	default:
		return "", domain.NewBadRequestError(fmt.Sprintf("invoice_type %q no soportado por el contrato mínimo", value))
	}
}

func resolveSectorAlias(value string) (int, error) {
	alias := normalizeAlias(value)
	if alias == "" || alias == "auto" {
		return 0, nil
	}
	if code, err := strconv.Atoi(alias); err == nil && code > 0 {
		return code, nil
	}
	aliases := map[string]int{
		"sale": 1, "venta": 1, "compraventa": 1, "compra_venta": 1,
		"education": 11, "educacion": 11, "educativo": 11,
	}
	if code, ok := aliases[alias]; ok {
		return code, nil
	}
	return 0, domain.NewBadRequestError(fmt.Sprintf("sector %q no reconocido; use auto, un código SIAT o un alias documentado", value))
}

func normalizeAlias(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n",
		"-", "_", " ", "_",
	)
	return replacer.Replace(value)
}

func (uc *InvoiceUsecase) PreviewSimplified(ctx context.Context, request MinimalInvoiceRequest) (*InvoicePreview, error) {
	result, err := NewInvoiceRequestSimplifier(uc).Simplify(ctx, request)
	if err != nil {
		return nil, err
	}
	return &result.Preview, nil
}

func (uc *InvoiceUsecase) CreateSimplified(ctx context.Context, request MinimalInvoiceRequest, idempotencyKey string) (*domain.Invoice, error) {
	result, err := NewInvoiceRequestSimplifier(uc).Simplify(ctx, request)
	if err != nil {
		return nil, err
	}
	result.Request.IdempotencyKey = strings.TrimSpace(idempotencyKey)
	return uc.Create(ctx, result.Request)
}

func (uc *InvoiceUsecase) EmitSimplified(ctx context.Context, request MinimalInvoiceRequest, idempotencyKey string) (*domain.Invoice, error) {
	sector, err := resolveSectorAlias(request.Sector)
	if err != nil {
		return nil, err
	}
	if sector == 30 {
		return nil, domain.NewBadRequestError("el sector 30 requiere emisión masiva; use /v1/siat/masiva/{companyId}/{pointOfSaleId}")
	}
	invoice, err := uc.CreateSimplified(ctx, request, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if invoice.Status != domain.InvoicePending {
		return invoice, nil
	}
	return uc.Emit(ctx, invoice.ID)
}
