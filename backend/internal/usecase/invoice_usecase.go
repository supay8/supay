package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
	"gorm.io/gorm"
)

type InvoiceUsecase struct {
	invoiceRepo   domain.InvoiceRepository
	productRepo   domain.ProductRepository
	syncStateRepo domain.CatalogSyncStateRepository
	customerRepo  domain.CustomerRepository
	companyRepo   domain.CompanyRepository
	posRepo       domain.PointOfSaleRepository
	catalogRepo   domain.CatalogRepository
	cufdRepo      domain.CufdRepository
	leyendaRepo   domain.SiatLeyendaRepository
	docSectorRepo domain.SiatActividadDocSectorRepository
	siatService   SiatEmissionService
	credentials   CredentialProvider
	modalidad     int
}

func NewInvoiceUsecase(invoiceRepo domain.InvoiceRepository, customerRepo domain.CustomerRepository, companyRepo domain.CompanyRepository, posRepo domain.PointOfSaleRepository, catalogRepo domain.CatalogRepository, cufdRepo domain.CufdRepository, siatService SiatEmissionService, modalidad int, extras ...any) *InvoiceUsecase {
	uc := &InvoiceUsecase{
		invoiceRepo:  invoiceRepo,
		customerRepo: customerRepo,
		companyRepo:  companyRepo,
		posRepo:      posRepo,
		catalogRepo:  catalogRepo,
		cufdRepo:     cufdRepo,
		siatService:  siatService,
		modalidad:    modalidad,
	}
	for _, extra := range extras {
		switch typed := extra.(type) {
		case domain.ProductRepository:
			uc.productRepo = typed
		case domain.CatalogSyncStateRepository:
			uc.syncStateRepo = typed
		case domain.SiatLeyendaRepository:
			uc.leyendaRepo = typed
		case domain.SiatActividadDocSectorRepository:
			uc.docSectorRepo = typed
		case CredentialProvider:
			uc.credentials = typed
		}
	}
	return uc
}

type CreateInvoiceItemRequest struct {
	ProductID         string  `json:"product_id,omitempty"`
	SKU               string  `json:"sku,omitempty"`
	Code              string  `json:"code,omitempty"`
	Description       string  `json:"description,omitempty"`
	CodigoActividad   *string `json:"codigo_actividad,omitempty"`
	CodigoProductoSin *string `json:"codigo_producto_sin,omitempty"`
	UnitCode          *int    `json:"unit_code,omitempty"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unit_price"`
	Discount          float64 `json:"discount,omitempty"`
	// SectorData contiene los campos sectoriales del ítem (datos_sector_detalle)
	// validados contra CamposDetalle del perfil del documento-sector. Opcional.
	SectorData json.RawMessage `json:"datos_sector,omitempty"`
}

type CreateInvoiceInlineCustomer struct {
	DocumentType   string  `json:"document_type"`
	DocumentNumber string  `json:"document_number"`
	Name           string  `json:"name"`
	Complement     *string `json:"complement,omitempty"`
}

type CreateInvoiceRequest struct {
	// CompanyId es opcional: si se omite se deriva del point_of_sale_id.
	CompanyId        string  `json:"company_id,omitempty"`
	PointOfSaleId    string  `json:"point_of_sale_id"`
	CustomerId       string  `json:"customer_id,omitempty"`
	Customer         *CreateInvoiceInlineCustomer `json:"customer,omitempty"`
	InvoiceType      string  `json:"invoice_type,omitempty"`
	CodigoMetodoPago int     `json:"codigo_metodo_pago,omitempty"`
	CodigoMoneda     int     `json:"codigo_moneda,omitempty"`
	TipoCambio       float64 `json:"tipo_cambio,omitempty"`
	// CodigoDocumentoSector: documento-sector del SIAT (1 compraventa, 11
	// educativo, 24 nota crédito/débito, ...). Si se omite se resuelve desde la
	// actividad económica de la empresa.
	CodigoDocumentoSector int                        `json:"codigo_documento_sector,omitempty"`
	Layout                string                     `json:"layout,omitempty"`
	Modalidad             int                        `json:"modalidad,omitempty"`
	Archivo               string                     `json:"archivo,omitempty"`
	HashArchivo           string                     `json:"hash_archivo,omitempty"`
	Cuf                   string                     `json:"cuf,omitempty"`
	CodigoTipoFactura     int                        `json:"codigo_tipo_factura,omitempty"`
	NombreEstudiante      *string                    `json:"nombre_estudiante,omitempty"`
	PeriodoFacturado      *string                    `json:"periodo_facturado,omitempty"`
	DatosSector           json.RawMessage            `json:"datos_sector,omitempty"`
	ReferenciaFacturaId   *string                    `json:"referencia_factura_id,omitempty"`
	IssueDate             *time.Time                 `json:"issue_date,omitempty"`
	Items                 []CreateInvoiceItemRequest `json:"items"`
	// Emit en true crea la factura y la emite al SIAT en una sola llamada.
	Emit bool `json:"emit,omitempty"`
	// IdempotencyKey se recibe vía header Idempotency-Key (no viaja en JSON).
	IdempotencyKey string `json:"-"`
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func cadenaOpcional(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

// resolveCustomer resuelve el cliente de la factura: por ID existente o por
// datos inline (busca por documento y crea si no existe).
func (uc *InvoiceUsecase) resolveCustomer(companyID, customerID string, inline *CreateInvoiceInlineCustomer) (*domain.Customer, error) {
	if customerID != "" {
		customer, err := uc.customerRepo.GetByID(customerID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, domain.NewNotFoundError("cliente no encontrado")
			}
			return nil, err
		}
		if customer.CompanyId != companyID {
			return nil, domain.NewBadRequestError("el cliente no pertenece a la empresa")
		}
		return customer, nil
	}

	if inline == nil {
		return nil, domain.NewBadRequestError("debe indicar customer_id o customer")
	}
	inline.DocumentType = strings.ToUpper(strings.TrimSpace(inline.DocumentType))
	inline.DocumentNumber = strings.TrimSpace(inline.DocumentNumber)
	inline.Name = strings.TrimSpace(inline.Name)
	if inline.DocumentType == "" || inline.DocumentNumber == "" {
		return nil, domain.NewBadRequestError("el tipo y número de documento del cliente son obligatorios")
	}
	if !validDocumentType(inline.DocumentType) {
		return nil, domain.NewBadRequestError("tipo de documento inválido (CI, CEX, PAS, NIT, OD)")
	}
	if inline.Name == "" {
		return nil, domain.NewBadRequestError("el nombre del cliente es obligatorio")
	}

	customer, err := uc.customerRepo.GetByCompanyAndDocument(companyID, inline.DocumentType, inline.DocumentNumber)
	if err == nil && customer != nil {
		return customer, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	customer = &domain.Customer{
		CompanyId:      companyID,
		DocumentType:   inline.DocumentType,
		DocumentNumber: inline.DocumentNumber,
		Name:           inline.Name,
		Complement:     inline.Complement,
	}
	if err := uc.customerRepo.Create(customer); err != nil {
		if errors.Is(err, domain.ErrCustomerDocumentConflict) {
			// Race: otro request creó el cliente entre el Get y el Create.
			existing, err2 := uc.customerRepo.GetByCompanyAndDocument(companyID, inline.DocumentType, inline.DocumentNumber)
			if err2 == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return customer, nil
}

func (uc *InvoiceUsecase) Create(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error) {
	if req.PointOfSaleId == "" {
		return nil, domain.NewBadRequestError("el point_of_sale_id es obligatorio")
	}
	if req.CustomerId == "" && req.Customer == nil {
		return nil, domain.NewBadRequestError("debe indicar customer_id o customer")
	}
	if req.CustomerId != "" && req.Customer != nil {
		return nil, domain.NewBadRequestError("indique customer_id o customer, no ambos")
	}

	pos, err := uc.posRepo.GetByID(req.PointOfSaleId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("punto de venta no encontrado")
		}
		return nil, err
	}
	if req.CompanyId == "" {
		req.CompanyId = pos.CompanyId
	}
	if pos.CompanyId != req.CompanyId {
		return nil, domain.NewBadRequestError("el punto de venta no pertenece a la empresa")
	}
	if !pos.IsActive {
		return nil, domain.NewBadRequestError("el punto de venta está inactivo")
	}

	refFactura, err := uc.autofillDocumentoAjusteDescuento(&req, req.CompanyId)
	if err != nil {
		return nil, err
	}

	if len(req.Items) == 0 {
		return nil, domain.NewBadRequestError("la factura debe tener al menos un ítem")
	}

	if req.IdempotencyKey != "" {
		if len(req.IdempotencyKey) > 100 {
			return nil, domain.NewBadRequestError("Idempotency-Key no puede exceder 100 caracteres")
		}
		existing, err := uc.invoiceRepo.GetByIdempotencyKey(req.PointOfSaleId, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			existing.IdempotencyKey = &req.IdempotencyKey
			return existing, nil
		}
	}

	company, err := uc.companyRepo.GetByID(req.CompanyId)
	if err != nil {
		return nil, domain.NewNotFoundError("empresa no encontrada")
	}

	if err := uc.ensureCatalogReadiness(req.CompanyId, req.PointOfSaleId); err != nil {
		return nil, err
	}

	customer, err := uc.resolveCustomer(req.CompanyId, req.CustomerId, req.Customer)
	if err != nil {
		return nil, err
	}

	// Todo borrador requiere un CUFD vigente para el punto de venta; la
	// emisión usará ese mismo CUFD (codigoControl incluido en el CUF). Con el
	// servicio de credenciales configurado el CUFD se resuelve lazy
	// (solicitándolo al SIAT si no existe); sin él se exige uno previo.
	now := time.Now()
	var activeCufd *domain.Cufd
	if uc.credentials != nil {
		activeCufd, err = uc.credentials.EnsureCufd(ctx, company, pos)
		if err != nil {
			return nil, err
		}
	} else {
		activeCufd, err = uc.invoiceRepo.FindActiveCufdForPointOfSale(req.PointOfSaleId, now)
		if err != nil {
			return nil, domain.NewConflictError("no hay un cufd vigente para el punto de venta; solicítelo primero")
		}
	}

	metodoPago := req.CodigoMetodoPago
	if metodoPago <= 0 {
		metodoPago = 1
	}
	moneda := req.CodigoMoneda
	if moneda <= 0 {
		moneda = 1
	}
	tipoCambio := req.TipoCambio
	if tipoCambio <= 0 {
		tipoCambio = 1
	}

	resolvedMappings, resolvedProducts, productIDs, productCodes, err := uc.resolveProductMappings(req)
	if err != nil {
		return nil, err
	}

	// Documento-sector: explícito o resuelto desde la actividad económica de la
	// empresa (catálogo actividadesDocumentoSector). Las actividades de
	// enseñanza (p.ej. 8549100) requieren el sector 11 FSEDU.
	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector, err = uc.resolveInvoiceSector(req.InvoiceType, mappingSectors(resolvedMappings), company)
		if err != nil {
			return nil, err
		}
	}
	for index, mapping := range resolvedMappings {
		if mapping.CodigoDocumentoSector != sector {
			return nil, domain.NewBadRequestError(fmt.Sprintf("el producto del ítem %d no está mapeado al sector %d", index+1, sector))
		}
	}

	// El perfil del documento-sector valida datos_sector (fail-fast, antes de
	// tocar la base) y deriva el tipoFacturaDocumento real (1 con crédito, 2 sin
	// crédito, 3 nota crédito/débito).
	perfil, err := siat.PerfilSectorLayout(sector, req.Layout)
	if err != nil {
		return nil, domain.NewBadRequestError(fmt.Sprintf("documento-sector %d no soportado: %v", sector, err))
	}
	modalidad := req.Modalidad
	if modalidad <= 0 {
		modalidad = uc.modalidad
	}
	if modalidad <= 0 {
		modalidad = siat.ModalidadElectronica
	}
	if err := perfil.ValidarModalidad(modalidad); err != nil {
		return nil, err
	}
	// Deduplicación legacy para sectores con DetallePar (47/48): si el cliente
	// envió el par manual para bypassear minOccurs=2, colapsar a ítems lógicos
	// antes de persistir (builder_reflex generará el par automáticamente).
	if perfil.DetallePar {
		if dedup := deduplicarItemsPar(req.Items); len(dedup) < len(req.Items) {
			req.Items = dedup
		}
	}
	if !perfil.HasBuilder() && (strings.TrimSpace(req.Archivo) == "" || strings.TrimSpace(req.HashArchivo) == "" || strings.TrimSpace(req.Cuf) == "") {
		return nil, domain.NewBadRequestError(fmt.Sprintf("el sector %d requiere archivo, hash_archivo y cuf", sector))
	}
	valoresSector, err := perfil.PrepararDatosSector(siat.SolicitudFactura{
		DatosSector:      req.DatosSector,
		NombreEstudiante: cadenaOpcional(req.NombreEstudiante),
		PeriodoFacturado: cadenaOpcional(req.PeriodoFacturado),
	})
	if err != nil {
		return nil, err
	}
	sectorDataJSON, err := json.Marshal(valoresSector)
	if err != nil {
		return nil, fmt.Errorf("no se pudo serializar datos_sector: %w", err)
	}
	tipoFactura := perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)

	// Los documentos de ajuste (24/29/47/48) deben referenciar la factura
	// original que corrigen.
	var ajustaFacturaId *string
	if perfil.EsAjuste() {
		if req.ReferenciaFacturaId == nil || strings.TrimSpace(*req.ReferenciaFacturaId) == "" {
			return nil, domain.NewBadRequestError("los documentos de ajuste requieren referencia_factura_id (factura original)")
		}
		ref := refFactura
		if ref == nil {
			var loadErr error
			ref, loadErr = uc.invoiceRepo.GetByID(strings.TrimSpace(*req.ReferenciaFacturaId))
			if loadErr != nil {
				return nil, domain.NewNotFoundError("la factura referenciada no existe")
			}
		}
		if ref.CompanyId != req.CompanyId {
			return nil, domain.NewBadRequestError("la factura referenciada pertenece a otra empresa")
		}
		if ref.Cuf == nil || *ref.Cuf == "" {
			return nil, domain.NewConflictError("la factura referenciada aún no tiene cuf; emítala antes de ajustarla")
		}
		if ref.InvoiceNumber <= 0 {
			return nil, domain.NewConflictError("la factura referenciada no tiene número correlativo válido; no se puede crear el ajuste")
		}
		ajustaFacturaId = &ref.ID
	}

	// La fecha de emisión debe expresarse en hora local de Bolivia (UTC-4): el
	// SIAT serializa la hora de pared sin zona y la interpreta como hora local.
	// Se usa el mismo instante que time.Now(), solo cambia la representación.
	issueDate := now.In(siat.LaPaz)
	if req.IssueDate != nil {
		issueDate = req.IssueDate.In(siat.LaPaz)
	}

	inv := &domain.Invoice{
		CompanyId:             req.CompanyId,
		CustomerId:            customer.ID,
		PointOfSaleId:         req.PointOfSaleId,
		CufdId:                activeCufd.ID,
		EmissionType:          "EN_LINEA",
		CodigoMetodoPago:      metodoPago,
		CodigoMoneda:          moneda,
		TipoCambio:            tipoCambio,
		CodigoDocumentoSector: sector,
		Layout:                req.Layout,
		Modalidad:             modalidad,
		Archivo:               strings.TrimSpace(req.Archivo),
		HashArchivo:           strings.TrimSpace(req.HashArchivo),
		CodigoTipoFactura:     tipoFactura,
		NombreEstudiante:      req.NombreEstudiante,
		PeriodoFacturado:      req.PeriodoFacturado,
		SectorData:            sectorDataJSON,
		AjustaFacturaId:       ajustaFacturaId,
		IssueDate:             issueDate,
		Status:                domain.InvoicePending,
	}
	if req.IdempotencyKey != "" {
		inv.IdempotencyKey = &req.IdempotencyKey
	}
	if strings.TrimSpace(req.Cuf) != "" {
		cuf := strings.TrimSpace(req.Cuf)
		inv.Cuf = &cuf
	}

	var subtotal float64
	for index, it := range req.Items {
		code := strings.TrimSpace(it.Code)
		if code == "" {
			code = productCodes[index]
		}
		if code == "" {
			return nil, domain.NewBadRequestError("el código del ítem es obligatorio")
		}
		description := strings.TrimSpace(it.Description)
		if description == "" {
			if product, ok := resolvedProducts[index]; ok && product != nil {
				description = product.Name
			}
		}
		if description == "" {
			return nil, domain.NewBadRequestError("la descripción del ítem es obligatoria")
		}
		if it.Quantity <= 0 {
			return nil, domain.NewBadRequestError("la cantidad del ítem debe ser mayor a cero")
		}
		if it.UnitPrice < 0 {
			return nil, domain.NewBadRequestError("el precio unitario no puede ser negativo")
		}
		if it.Discount < 0 {
			return nil, domain.NewBadRequestError("el descuento del ítem no puede ser negativo")
		}
		// Validación fail-fast de los datos sectoriales del ítem contra
		// CamposDetalle del perfil. Si el perfil declara campos requeridos, la
		// ausencia de datos_sector en el ítem también debe fallar aquí.
		if _, err := perfil.ValidarDatosDetalle(it.SectorData); err != nil {
			return nil, domain.NewBadRequestError(fmt.Sprintf("ítem %d: %v", index+1, err))
		}
		itemSubtotal := round2(it.Quantity*it.UnitPrice - it.Discount)
		if itemSubtotal < 0 {
			return nil, domain.NewBadRequestError("el descuento del ítem no puede superar el monto")
		}
		item := domain.InvoiceItem{
			ProductID:         productIDs[index],
			Code:              code,
			Description:       description,
			CodigoActividad:   it.CodigoActividad,
			CodigoProductoSin: it.CodigoProductoSin,
			UnitCode:          it.UnitCode,
			Quantity:          it.Quantity,
			UnitPrice:         it.UnitPrice,
			Discount:          it.Discount,
			Subtotal:          itemSubtotal,
			SectorData:        it.SectorData,
		}
		if mapping, ok := resolvedMappings[index]; ok {
			codigoActividad := mapping.CodigoActividad
			codigoSin := strconv.FormatInt(mapping.CodigoProductoSin, 10)
			unidad := mapping.UnidadMedida
			item.CodigoActividad = &codigoActividad
			item.CodigoProductoSin = &codigoSin
			item.UnitCode = &unidad
		}
		inv.Items = append(inv.Items, item)
		subtotal += itemSubtotal
	}
	inv.Subtotal = round2(subtotal)
	inv.Total = inv.Subtotal

	if err := uc.invoiceRepo.Create(inv); err != nil {
		// Race de idempotencia: otro request creó primero la factura con la
		// misma Idempotency-Key (idx_invoice_idem_key). Se devuelve la
		// existente para que el cliente reciba el replay en vez de un 500.
		if req.IdempotencyKey != "" && isUniqueViolation(err) {
			if existing, err2 := uc.invoiceRepo.GetByIdempotencyKey(req.PointOfSaleId, req.IdempotencyKey); err2 == nil && existing != nil {
				existing.IdempotencyKey = &req.IdempotencyKey
				return existing, nil
			}
		}
		return nil, err
	}

	return inv, nil
}

// isUniqueViolation reconoce errores de restricción única de PostgreSQL
// (SQLSTATE 23505), mismo patrón usado por el repositorio de puntos de venta.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

func (uc *InvoiceUsecase) ensureCatalogReadiness(companyID, pointOfSaleID string) error {
	if uc.syncStateRepo == nil {
		return nil
	}
	states, err := uc.syncStateRepo.List(companyID, pointOfSaleID)
	if err != nil {
		return fmt.Errorf("no se pudo verificar readiness de catálogos: %w", err)
	}
	ready := make(map[string]bool, len(states))
	for _, state := range states {
		ready[state.Operation] = state.Status == "SUCCESS" && state.SyncedAt != nil && time.Since(*state.SyncedAt) <= catalogSyncMaxAge
	}
	for _, required := range []string{"actividades", "productosServicios", "actividadesDocumentoSector", "unidadMedida", "tipoMoneda", "tipoMetodoPago", "leyendasFactura"} {
		if !ready[required] {
			return domain.NewConflictError(fmt.Sprintf("catálogos SIAT incompletos; sincronice %s antes de crear la factura", required))
		}
	}
	return nil
}

func (uc *InvoiceUsecase) GetByID(id string) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}
	return inv, nil
}

// resolveProductMappings transforma product_id/SKU en los códigos fiscales
// congelados dentro del borrador. Los campos legacy siguen pasando por el
// flujo anterior cuando el ítem no identifica un producto interno.
func (uc *InvoiceUsecase) resolveProductMappings(req CreateInvoiceRequest) (map[int]domain.ProductMapping, map[int]*domain.Product, map[int]*string, map[int]string, error) {
	resolved := make(map[int]domain.ProductMapping)
	resolvedProducts := make(map[int]*domain.Product)
	productIDs := make(map[int]*string)
	productCodes := make(map[int]string)
	for index, item := range req.Items {
		if strings.TrimSpace(item.ProductID) == "" && strings.TrimSpace(item.SKU) == "" {
			continue
		}
		if uc.productRepo == nil {
			return nil, nil, nil, nil, domain.NewConflictError("el catálogo de productos no está configurado; sincronice y configure los productos internos")
		}
		var product *domain.Product
		var err error
		if strings.TrimSpace(item.ProductID) != "" {
			product, err = uc.productRepo.GetByID(req.CompanyId, strings.TrimSpace(item.ProductID))
		} else {
			product, err = uc.productRepo.GetBySKU(req.CompanyId, strings.TrimSpace(item.SKU))
		}
		if err != nil || product == nil {
			return nil, nil, nil, nil, domain.NewNotFoundError(fmt.Sprintf("el producto del ítem %d no existe o está inactivo", index+1))
		}
		mappings := make([]domain.ProductMapping, 0, len(product.Mappings))
		for _, mapping := range product.Mappings {
			if !mapping.Active || mapping.CodigoProductoSin <= 0 || mapping.CodigoActividad == "" || mapping.CodigoDocumentoSector <= 0 || mapping.UnidadMedida <= 0 {
				continue
			}
			if req.CodigoDocumentoSector > 0 && mapping.CodigoDocumentoSector != req.CodigoDocumentoSector {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(req.InvoiceType), "education") && mapping.CodigoDocumentoSector != siat.SectorEducativo && mapping.CodigoDocumentoSector != 46 {
				continue
			}
			mappings = append(mappings, mapping)
		}
		if len(mappings) == 0 {
			return nil, nil, nil, nil, domain.NewConflictError(fmt.Sprintf("el producto del ítem %d no tiene un mapeo fiscal vigente", index+1))
		}
		if len(mappings) > 1 {
			defaults := make([]domain.ProductMapping, 0, len(mappings))
			for _, mapping := range mappings {
				if mapping.IsDefault {
					defaults = append(defaults, mapping)
				}
			}
			if len(defaults) == 1 {
				mappings = defaults
			} else {
				return nil, nil, nil, nil, domain.NewConflictError(fmt.Sprintf("el producto del ítem %d tiene múltiples sectores; configure un mapeo predeterminado o indique invoice_type", index+1))
			}
		}
		resolved[index] = mappings[0]
		resolvedProducts[index] = product
		productID := product.ID
		productIDs[index] = &productID
		productCodes[index] = product.SKU
	}
	return resolved, resolvedProducts, productIDs, productCodes, nil
}

func mappingSectors(mappings map[int]domain.ProductMapping) map[int]bool {
	sectors := make(map[int]bool)
	for _, mapping := range mappings {
		sectors[mapping.CodigoDocumentoSector] = true
	}
	return sectors
}

func (uc *InvoiceUsecase) resolveInvoiceSector(invoiceType string, candidates map[int]bool, company *domain.Company) (int, error) {
	typeName := strings.ToLower(strings.TrimSpace(invoiceType))
	switch typeName {
	case "credit_note", "debit_note":
		return siat.SectorNotaCreditoDebito, nil
	case "education":
		for sector := range candidates {
			if sector == siat.SectorEducativo || sector == 46 {
				return sector, nil
			}
		}
		return 0, domain.NewBadRequestError("invoice_type education requiere un producto mapeado al sector educativo")
	case "sale", "":
		if len(candidates) == 1 {
			for sector := range candidates {
				return sector, nil
			}
		}
		if len(candidates) > 1 {
			return 0, domain.NewConflictError("los productos pertenecen a sectores distintos; indique invoice_type y configure un mapeo compatible")
		}
		actividad := ""
		if company != nil && company.CodigoActividad != nil {
			actividad = strings.TrimSpace(*company.CodigoActividad)
		}
		return uc.resolveDocumentoSector(company.ID, actividad)
	default:
		return 0, domain.NewBadRequestError(fmt.Sprintf("invoice_type %q no soportado", invoiceType))
	}
}

func (uc *InvoiceUsecase) ListByPointOfSale(pointOfSaleID string) ([]*domain.Invoice, error) {
	return uc.invoiceRepo.ListByPointOfSale(pointOfSaleID)
}

// Limites de paginación del listado de facturas.
const (
	invoiceListDefaultLimit = 50
	invoiceListMaxLimit     = 200
)

// ListInvoices devuelve el listado paginado de facturas de un punto de venta
// (sin xml/archivo), con filtro opcional por estado y rango de emisión.
func (uc *InvoiceUsecase) ListInvoices(filter domain.InvoiceListFilter) ([]*domain.Invoice, int64, error) {
	if strings.TrimSpace(filter.PointOfSaleID) == "" {
		return nil, 0, domain.NewBadRequestError("point_of_sale_id es obligatorio")
	}
	if filter.Status != nil && !filter.Status.Valid() {
		return nil, 0, domain.NewBadRequestError(fmt.Sprintf("estado %q no es válido; valores: %s", *filter.Status, domain.InvoiceStatuses()))
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return nil, 0, domain.NewBadRequestError("from no puede ser posterior a to")
	}
	if filter.Limit <= 0 {
		filter.Limit = invoiceListDefaultLimit
	}
	if filter.Limit > invoiceListMaxLimit {
		filter.Limit = invoiceListMaxLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return uc.invoiceRepo.ListFiltered(filter)
}

// SectoresHabilitados devuelve qué códigos de documento-sector puede emitir
// la empresa según el catálogo sincronizado actividadesDocumentoSector.
func (uc *InvoiceUsecase) SectoresHabilitados(companyID string) (map[int]bool, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	if uc.docSectorRepo == nil {
		return nil, domain.NewConflictError("no se pudo consultar actividadesDocumentoSector; sincronice los catálogos")
	}
	items, err := uc.docSectorRepo.List(companyID)
	if err != nil {
		return nil, err
	}
	habilitados := make(map[int]bool, len(items))
	for _, item := range items {
		habilitados[item.CodigoDocumentoSector] = true
	}
	return habilitados, nil
}

// resolveDocumentoSector determina el documento-sector del SIAT para la
// actividad económica de la empresa consultando el catálogo sincronizado
// actividadesDocumentoSector (tabla siat_actividades_doc_sector). Prefiere la
// factura de compraventa (FCV); si la actividad solo está asociada a sectores
// educativos (p.ej. 8549100 -> FSEDU), usa ese sector.
func (uc *InvoiceUsecase) resolveDocumentoSector(companyID, actividad string) (int, error) {
	actividad = strings.TrimSpace(actividad)
	if actividad == "" {
		return 0, domain.NewConflictError("no se puede resolver documento-sector: falta el codigo de actividad económica de la empresa")
	}
	if uc.docSectorRepo == nil {
		return 0, domain.NewConflictError("no se puede resolver documento-sector: sincronice actividadesDocumentoSector")
	}
	items, err := uc.docSectorRepo.ListByActividad(companyID, actividad)
	if err != nil {
		return 0, fmt.Errorf("no se pudo consultar actividadesDocumentoSector: %w", err)
	}
	found := 0
	for _, item := range items {
		switch strings.TrimSpace(item.TipoDocumentoSector) {
		case "FCV":
			return item.CodigoDocumentoSector, nil
		case "FSEDU":
			found = item.CodigoDocumentoSector
		}
	}
	if found > 0 {
		return found, nil
	}
	return 0, domain.NewConflictError(fmt.Sprintf("la actividad %s no tiene documento-sector sincronizado; sincronice actividadesDocumentoSector", actividad))
}
