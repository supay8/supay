package usecase

import (
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
	invoiceRepo  domain.InvoiceRepository
	productRepo  domain.ProductRepository
	customerRepo domain.CustomerRepository
	companyRepo  domain.CompanyRepository
	posRepo      domain.PointOfSaleRepository
	catalogRepo  domain.CatalogRepository
	cufdRepo     domain.CufdRepository
	siatService  SiatEmissionService
	modalidad    int
}

func NewInvoiceUsecase(invoiceRepo domain.InvoiceRepository, customerRepo domain.CustomerRepository, companyRepo domain.CompanyRepository, posRepo domain.PointOfSaleRepository, catalogRepo domain.CatalogRepository, cufdRepo domain.CufdRepository, siatService SiatEmissionService, modalidad int, productRepos ...domain.ProductRepository) *InvoiceUsecase {
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
	if len(productRepos) > 0 {
		uc.productRepo = productRepos[0]
	}
	return uc
}

type CreateInvoiceItemRequest struct {
	ProductID         string  `json:"product_id,omitempty"`
	SKU               string  `json:"sku,omitempty"`
	Code              string  `json:"code"`
	Description       string  `json:"description"`
	CodigoActividad   *string `json:"codigo_actividad,omitempty"`
	CodigoProductoSin *string `json:"codigo_producto_sin,omitempty"`
	UnitCode          *int    `json:"unit_code,omitempty"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unit_price"`
	Discount          float64 `json:"discount,omitempty"`
}

type CreateInvoiceRequest struct {
	CompanyId        string  `json:"company_id"`
	PointOfSaleId    string  `json:"point_of_sale_id"`
	CustomerId       string  `json:"customer_id"`
	InvoiceType      string  `json:"invoice_type,omitempty"`
	CodigoMetodoPago int     `json:"codigo_metodo_pago,omitempty"`
	CodigoMoneda     int     `json:"codigo_moneda,omitempty"`
	TipoCambio       float64 `json:"tipo_cambio,omitempty"`
	// CodigoDocumentoSector: documento-sector del SIAT (1 compraventa, 11
	// educativo, 24 nota crédito/débito, ...). Si se omite se resuelve desde la
	// actividad económica de la empresa.
	CodigoDocumentoSector int                        `json:"codigo_documento_sector,omitempty"`
	CodigoTipoFactura     int                        `json:"codigo_tipo_factura,omitempty"`
	NombreEstudiante      *string                    `json:"nombre_estudiante,omitempty"`
	PeriodoFacturado      *string                    `json:"periodo_facturado,omitempty"`
	DatosSector           json.RawMessage            `json:"datos_sector,omitempty"`
	ReferenciaFacturaId   *string                    `json:"referencia_factura_id,omitempty"`
	IssueDate             *time.Time                 `json:"issue_date,omitempty"`
	Items                 []CreateInvoiceItemRequest `json:"items"`
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

func (uc *InvoiceUsecase) Create(req CreateInvoiceRequest) (*domain.Invoice, error) {
	if req.CompanyId == "" {
		return nil, errors.New("el company_id es obligatorio")
	}
	if req.PointOfSaleId == "" {
		return nil, errors.New("el point_of_sale_id es obligatorio")
	}
	if req.CustomerId == "" {
		return nil, errors.New("el customer_id es obligatorio")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("la factura debe tener al menos un ítem")
	}

	company, err := uc.companyRepo.GetByID(req.CompanyId)
	if err != nil {
		return nil, errors.New("empresa no encontrada")
	}

	pos, err := uc.posRepo.GetByID(req.PointOfSaleId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("punto de venta no encontrado")
		}
		return nil, err
	}
	if pos.CompanyId != req.CompanyId {
		return nil, errors.New("el punto de venta no pertenece a la empresa")
	}
	if !pos.IsActive {
		return nil, errors.New("el punto de venta está inactivo")
	}

	customer, err := uc.customerRepo.GetByID(req.CustomerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("cliente no encontrado")
		}
		return nil, err
	}
	if customer.CompanyId != req.CompanyId {
		return nil, errors.New("el cliente no pertenece a la empresa")
	}

	// Todo borrador requiere un CUFD vigente para el punto de venta; la
	// emisión usará ese mismo CUFD (codigoControl incluido en el CUF).
	now := time.Now()
	activeCufd, err := uc.invoiceRepo.FindActiveCufdForPointOfSale(req.PointOfSaleId, now)
	if err != nil {
		return nil, errors.New("no hay un CUFD vigente para el punto de venta; solicítelo primero (POST /siat/cufd/...)")
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

	resolvedMappings, productIDs, productCodes, err := uc.resolveProductMappings(req)
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
			return nil, fmt.Errorf("el producto del ítem %d no está mapeado al sector %d", index+1, sector)
		}
	}

	// El perfil del documento-sector valida datos_sector (fail-fast, antes de
	// tocar la base) y deriva el tipoFacturaDocumento real (1 con crédito, 2 sin
	// crédito, 3 nota crédito/débito).
	perfil, err := siat.PerfilSector(sector)
	if err != nil {
		return nil, fmt.Errorf("documento-sector %d no soportado: %w", sector, err)
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
			return nil, errors.New("los documentos de ajuste requieren referencia_factura_id (factura original)")
		}
		ref, err := uc.invoiceRepo.GetByID(strings.TrimSpace(*req.ReferenciaFacturaId))
		if err != nil {
			return nil, errors.New("la factura referenciada no existe")
		}
		if ref.CompanyId != req.CompanyId {
			return nil, errors.New("la factura referenciada pertenece a otra empresa")
		}
		if ref.Cuf == nil || *ref.Cuf == "" {
			return nil, errors.New("la factura referenciada aún no tiene CUF; emítala antes de ajustarla")
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
		CustomerId:            req.CustomerId,
		PointOfSaleId:         req.PointOfSaleId,
		CufdId:                activeCufd.ID,
		EmissionType:          "EN_LINEA",
		CodigoMetodoPago:      metodoPago,
		CodigoMoneda:          moneda,
		TipoCambio:            tipoCambio,
		CodigoDocumentoSector: sector,
		CodigoTipoFactura:     tipoFactura,
		NombreEstudiante:      req.NombreEstudiante,
		PeriodoFacturado:      req.PeriodoFacturado,
		SectorData:            sectorDataJSON,
		AjustaFacturaId:       ajustaFacturaId,
		IssueDate:             issueDate,
		Status:                domain.InvoicePending,
	}

	var subtotal float64
	for index, it := range req.Items {
		code := strings.TrimSpace(it.Code)
		if code == "" {
			code = productCodes[index]
		}
		if code == "" {
			return nil, errors.New("el código del ítem es obligatorio")
		}
		if strings.TrimSpace(it.Description) == "" {
			return nil, errors.New("la descripción del ítem es obligatoria")
		}
		if it.Quantity <= 0 {
			return nil, errors.New("la cantidad del ítem debe ser mayor a cero")
		}
		if it.UnitPrice < 0 {
			return nil, errors.New("el precio unitario no puede ser negativo")
		}
		if it.Discount < 0 {
			return nil, errors.New("el descuento del ítem no puede ser negativo")
		}
		itemSubtotal := round2(it.Quantity*it.UnitPrice - it.Discount)
		if itemSubtotal < 0 {
			return nil, errors.New("el descuento del ítem no puede superar el monto")
		}
		item := domain.InvoiceItem{
			ProductID:         productIDs[index],
			Code:              code,
			Description:       it.Description,
			CodigoActividad:   it.CodigoActividad,
			CodigoProductoSin: it.CodigoProductoSin,
			UnitCode:          it.UnitCode,
			Quantity:          it.Quantity,
			UnitPrice:         it.UnitPrice,
			Discount:          it.Discount,
			Subtotal:          itemSubtotal,
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
		return nil, err
	}

	return inv, nil
}

func (uc *InvoiceUsecase) GetByID(id string) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("factura no encontrada")
		}
		return nil, err
	}
	return inv, nil
}

// resolveProductMappings transforma product_id/SKU en los códigos fiscales
// congelados dentro del borrador. Los campos legacy siguen pasando por el
// flujo anterior cuando el ítem no identifica un producto interno.
func (uc *InvoiceUsecase) resolveProductMappings(req CreateInvoiceRequest) (map[int]domain.ProductMapping, map[int]*string, map[int]string, error) {
	resolved := make(map[int]domain.ProductMapping)
	productIDs := make(map[int]*string)
	productCodes := make(map[int]string)
	for index, item := range req.Items {
		if strings.TrimSpace(item.ProductID) == "" && strings.TrimSpace(item.SKU) == "" {
			continue
		}
		if uc.productRepo == nil {
			return nil, nil, nil, errors.New("el catálogo de productos no está configurado; sincronice y configure los productos internos")
		}
		var product *domain.Product
		var err error
		if strings.TrimSpace(item.ProductID) != "" {
			product, err = uc.productRepo.GetByID(req.CompanyId, strings.TrimSpace(item.ProductID))
		} else {
			product, err = uc.productRepo.GetBySKU(req.CompanyId, strings.TrimSpace(item.SKU))
		}
		if err != nil || product == nil {
			return nil, nil, nil, fmt.Errorf("el producto del ítem %d no existe o está inactivo", index+1)
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
			return nil, nil, nil, fmt.Errorf("el producto del ítem %d no tiene un mapeo fiscal vigente", index+1)
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
				return nil, nil, nil, fmt.Errorf("el producto del ítem %d tiene múltiples sectores; configure un mapeo predeterminado o indique invoice_type", index+1)
			}
		}
		resolved[index] = mappings[0]
		productID := product.ID
		productIDs[index] = &productID
		productCodes[index] = product.SKU
	}
	return resolved, productIDs, productCodes, nil
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
		return 0, errors.New("invoice_type education requiere un producto mapeado al sector educativo")
	case "sale", "":
		if len(candidates) == 1 {
			for sector := range candidates {
				return sector, nil
			}
		}
		if len(candidates) > 1 {
			return 0, errors.New("los productos pertenecen a sectores distintos; indique invoice_type y configure un mapeo compatible")
		}
		actividad := ""
		if company != nil && company.CodigoActividad != nil {
			actividad = strings.TrimSpace(*company.CodigoActividad)
		}
		return uc.resolveDocumentoSector(company.ID, actividad), nil
	default:
		return 0, fmt.Errorf("invoice_type %q no soportado", invoiceType)
	}
}

func (uc *InvoiceUsecase) ListByPointOfSale(pointOfSaleID string) ([]*domain.Invoice, error) {
	return uc.invoiceRepo.ListByPointOfSale(pointOfSaleID)
}

// resolveDocumentoSector determina el documento-sector del SIAT para la
// actividad económica de la empresa consultando el catálogo sincronizado
// actividadesDocumentoSector (descripción "actividad|FCV|FSEDU|NCD|NCDDE").
// Prefiere la factura de compraventa (FCV); si la actividad solo está asociada
// a sectores educativos (p.ej. 8549100 -> FSEDU), usa ese sector.
func (uc *InvoiceUsecase) resolveDocumentoSector(companyID, actividad string) int {
	if uc.catalogRepo != nil {
		if items, err := uc.catalogRepo.List(companyID, "actividadesDocumentoSector"); err == nil {
			found := 0
			for _, item := range items {
				fields := strings.Split(item.Descripcion, "|")
				if len(fields) < 2 || strings.TrimSpace(fields[0]) != actividad {
					continue
				}
				tipo := strings.TrimSpace(fields[1])
				if tipo == "FCV" {
					return item.Codigo
				}
				if tipo == "FSEDU" {
					found = item.Codigo
				}
			}
			if found > 0 {
				return found
			}
		}
	}
	return 1
}
