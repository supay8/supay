package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"gorm.io/gorm"
)

type InvoiceUsecase struct {
	invoiceRepo          domain.InvoiceRepository
	syncStateRepo        domain.CatalogSyncStateRepository
	customerRepo         domain.CustomerRepository
	companyRepo          domain.CompanyRepository
	posRepo              domain.PointOfSaleRepository
	catalogRepo          domain.CatalogRepository
	cufdRepo             domain.CufdRepository
	leyendaRepo          domain.SiatLeyendaRepository
	docSectorRepo        domain.SiatActividadDocSectorRepository
	siatService          ports.FiscalService
	siatProvider         siat.SiatClientProvider
	credentials          CredentialProvider
	contingencyRepo      domain.ContingencyEventRepository
	modalidad            int
	pdfService           PdfGenerator
	fileService          *InvoiceFileService
	allowCustomIssueDate bool
}

func (uc *InvoiceUsecase) SetFileService(files *InvoiceFileService) { uc.fileService = files }

// SetContingencyRepository enables the official offline contingency fallback
// without expanding the constructor used by embedded consumers and tests.
func (uc *InvoiceUsecase) SetContingencyRepository(repo domain.ContingencyEventRepository) {
	uc.contingencyRepo = repo
}

func (uc *InvoiceUsecase) resolveEmissionService(ctx context.Context, companyID string) (ports.FiscalService, error) {
	if uc.siatProvider != nil && companyID != "" {
		if svc, err := uc.siatProvider.GetForCompany(ctx, companyID); err == nil {
			return siat.NewFiscalAdapter(svc), nil
		} else if uc.siatService == nil {
			return nil, err
		}
	}
	if uc.siatService == nil {
		return nil, ErrSiatNoDisponible
	}
	return uc.siatService, nil
}

func (uc *InvoiceUsecase) effectiveModalidadForCompany(company *domain.Company) int {
	if company != nil && company.Modalidad != 0 {
		return company.Modalidad
	}
	if uc.modalidad != 0 {
		return uc.modalidad
	}
	return siat.ModalidadElectronica
}

// PdfGenerator genera y persiste PDFs (interfaz para evitar import cycle con internal/pdf).
type PdfGenerator interface {
	GenerateAndPersist(ctx context.Context, invoiceID string)
}

func NewInvoiceUsecase(
	invoiceRepo domain.InvoiceRepository,
	customerRepo domain.CustomerRepository,
	companyRepo domain.CompanyRepository,
	posRepo domain.PointOfSaleRepository,
	catalogRepo domain.CatalogRepository,
	cufdRepo domain.CufdRepository,
	siatService ports.FiscalService,
	modalidad int,
	syncStateRepo domain.CatalogSyncStateRepository,
	leyendaRepo domain.SiatLeyendaRepository,
	docSectorRepo domain.SiatActividadDocSectorRepository,
	credentials CredentialProvider,
	pdfService PdfGenerator,
	allowCustomIssueDate bool,
	siatProvider siat.SiatClientProvider,
) *InvoiceUsecase {
	return &InvoiceUsecase{
		invoiceRepo:          invoiceRepo,
		customerRepo:         customerRepo,
		companyRepo:          companyRepo,
		posRepo:              posRepo,
		catalogRepo:          catalogRepo,
		cufdRepo:             cufdRepo,
		siatService:          siatService,
		modalidad:            modalidad,
		syncStateRepo:        syncStateRepo,
		leyendaRepo:          leyendaRepo,
		docSectorRepo:        docSectorRepo,
		credentials:          credentials,
		pdfService:           pdfService,
		allowCustomIssueDate: allowCustomIssueDate,
		siatProvider:         siatProvider,
	}
}

type CreateInvoiceItemRequest struct {
	// SKU es un alias de compatibilidad para Code. No consulta ningún catálogo
	// interno: todos los datos del producto se congelan en InvoiceItem.
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

type FlexibleTime time.Time

func (ft FlexibleTime) MarshalJSON() ([]byte, error) {
	t := time.Time(ft)
	return json.Marshal(t.Format(time.RFC3339Nano))
}

func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	// quoted string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		// maybe number or RFC3339 without quotes? try raw time
		var t time.Time
		if err2 := json.Unmarshal(data, &t); err2 == nil {
			*ft = FlexibleTime(t)
			return nil
		}
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := parseFlexibleTime(s)
	if err != nil {
		return err
	}
	*ft = FlexibleTime(t)
	return nil
}

func (ft FlexibleTime) Time() time.Time { return time.Time(ft) }

func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, siat.LaPaz); err == nil {
			return t, nil
		}
	}
	// fallback with SIAT helper (LaPaz .000)
	if t, err := time.ParseInLocation("2006-01-02T15:04:05.000", s, siat.LaPaz); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("formato de fecha no soportado %q: use RFC3339 o YYYY-MM-DDTHH:mm:ss.SSS", s)
}

type CreateInvoiceRequest struct {
	Total *float64 `json:"total,omitempty"`
	// CompanyId es opcional: si se omite se deriva del point_of_sale_id.
	CompanyId     string `json:"company_id,omitempty"`
	PointOfSaleId string `json:"point_of_sale_id"`
	// CustomerId se conserva solo para llamadas internas antiguas. La API no
	// puede usar customers como fuente de datos fiscales.
	CustomerId string `json:"-"`
	// Customer inline legacy (tests): preferir ClientDocument*.
	Customer *CreateInvoiceInlineCustomer `json:"customer,omitempty"`
	// Receiver deprecated: compatibilidad con tests viejos (mapear a ClientDocument*).
	Receiver *CreateInvoiceReceiver `json:"receiver,omitempty"`
	// CustomerId / Customer: toda factura referencia un Customer (única
	// fuente de verdad de los datos fiscales del receptor). El modo receiver
	// (receptor directo sin cliente) fue eliminado.
	ClientDocumentType   string  `json:"client_document_type,omitempty"`
	ClientDocumentNumber string  `json:"client_document_number,omitempty"`
	ClientName           string  `json:"client_name,omitempty"`
	ClientEmail          string  `json:"client_email,omitempty"`
	ClientComplement     *string `json:"client_complement,omitempty"`
	InvoiceType          string  `json:"invoice_type,omitempty"`
	CodigoMetodoPago     int     `json:"codigo_metodo_pago,omitempty"`
	CodigoMoneda         int     `json:"codigo_moneda,omitempty"`
	TipoCambio           float64 `json:"tipo_cambio,omitempty"`
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
	IssueDate             *FlexibleTime              `json:"issue_date,omitempty"`
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

// CreateInvoiceReceiver deprecated: kept for test compatibility. Maps int code to string type.
type CreateInvoiceReceiver struct {
	DocumentType   int     `json:"document_type"`
	DocumentNumber string  `json:"document_number"`
	Complement     *string `json:"complement,omitempty"`
	Name           string  `json:"name"`
	Email          *string `json:"email,omitempty"`
}

func documentTypeCodeToString(code int) string {
	switch code {
	case 1:
		return "CI"
	case 2:
		return "CEX"
	case 3:
		return "PAS"
	case 4:
		return "NIT"
	case 5:
		return "OD"
	default:
		return "CI"
	}
}

func documentTypeStringToCode(docType string) int {
	switch strings.ToUpper(strings.TrimSpace(docType)) {
	case "CI":
		return 1
	case "CEX":
		return 2
	case "PAS":
		return 3
	case "NIT":
		return 4
	case "OD":
		return 5
	default:
		return 0
	}
}

func generateCodigoCliente(docType, docNumber string) string {
	return strings.ToUpper(strings.TrimSpace(docType)) + strings.TrimSpace(docNumber)
}

// resolveCustomer construye el snapshot recibido y reutiliza una fila histórica
// solo cuando toda la identidad coincide. Nunca reemplaza el nombre asociado a
// un CI/NIT ni usa customers para completar campos omitidos.
func (uc *InvoiceUsecase) resolveCustomer(companyID string, req CreateInvoiceRequest) (*domain.Customer, error) {
	documentNumber := strings.TrimSpace(req.ClientDocumentNumber)
	name := strings.TrimSpace(req.ClientName)
	if documentNumber == "" || name == "" {
		return nil, domain.NewBadRequestError("cliente requerido: client_document_number y client_name son obligatorios")
	}
	documentType := strings.ToUpper(strings.TrimSpace(req.ClientDocumentType))
	if documentType == "" {
		documentType = "CI"
	}
	if documentTypeCodeToString(documentTypeStringToCode(documentType)) != documentType {
		return nil, domain.NewBadRequestError("tipo de documento inválido (CI, CEX, PAS, NIT, OD)")
	}
	email := strings.TrimSpace(req.ClientEmail)
	existingCustomer, err := uc.customerRepo.GetByCompanyAndFiscalIdentity(companyID, documentType, documentNumber, req.ClientComplement, name, email)
	if err == nil && existingCustomer != nil {
		return existingCustomer, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	newCustomer := &domain.Customer{
		CompanyId:      companyID,
		DocumentType:   documentType,
		DocumentNumber: documentNumber,
		Name:           name,
		Complement:     req.ClientComplement,
		Email:          emailPtr,
		CodigoCliente:  generateCodigoCliente(documentType, documentNumber),
	}
	return newCustomer, nil
}
func (uc *InvoiceUsecase) Create(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error) {
	if req.PointOfSaleId == "" {
		return nil, domain.NewBadRequestError("el point_of_sale_id es obligatorio")
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

	if len(req.Items) == 0 && req.CodigoDocumentoSector != 30 {
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

	// Alias legacy: si viene Customer inline, mapear a ClientDocument*
	if req.Customer != nil {
		if strings.TrimSpace(req.ClientDocumentType) == "" {
			req.ClientDocumentType = req.Customer.DocumentType
		}
		if strings.TrimSpace(req.ClientDocumentNumber) == "" {
			req.ClientDocumentNumber = req.Customer.DocumentNumber
		}
		if strings.TrimSpace(req.ClientName) == "" {
			req.ClientName = req.Customer.Name
		}
		if req.ClientComplement == nil {
			req.ClientComplement = req.Customer.Complement
		}
	}
	if req.Receiver != nil {
		if strings.TrimSpace(req.ClientDocumentType) == "" {
			req.ClientDocumentType = documentTypeCodeToString(req.Receiver.DocumentType)
		}
		if strings.TrimSpace(req.ClientDocumentNumber) == "" {
			req.ClientDocumentNumber = req.Receiver.DocumentNumber
		}
		if strings.TrimSpace(req.ClientName) == "" {
			req.ClientName = req.Receiver.Name
		}
		if req.ClientComplement == nil {
			req.ClientComplement = req.Receiver.Complement
		}
		if strings.TrimSpace(req.ClientEmail) == "" && req.Receiver.Email != nil {
			req.ClientEmail = *req.Receiver.Email
		}
	}
	if strings.TrimSpace(req.CustomerId) != "" && strings.TrimSpace(req.ClientDocumentNumber) == "" {
		return nil, domain.NewBadRequestError("customer_id no puede usarse como fuente fiscal; envíe los datos completos del receptor")
	}
	customer, err := uc.resolveCustomer(req.CompanyId, req)
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

	// Documento-sector: explícito o resuelto desde la actividad económica de la
	// empresa (catálogo actividadesDocumentoSector). Las actividades de
	// enseñanza (p.ej. 8549100) requieren el sector 11 FSEDU.
	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector, err = uc.resolveInvoiceSector(req.InvoiceType, company)
		if err != nil {
			return nil, err
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
		return nil, domain.NewBadRequestError(err.Error())
	}
	if req.Total != nil && (perfil.ConDetalle || len(req.Items) > 0 || *req.Total < 0) {
		return nil, domain.NewBadRequestError("total solo se admite sin items en sectores sin detalle y debe ser no negativo")
	}
	if !perfil.ConDetalle && len(req.Items) == 0 && req.Total == nil {
		return nil, domain.NewBadRequestError("total es obligatorio para documentos sin detalle")
	}
	if perfil.DetalleUnico && len(req.Items) != 1 {
		return nil, domain.NewBadRequestError(fmt.Sprintf("el sector %d requiere exactamente un ítem", sector))
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
		return nil, domain.NewBadRequestError(err.Error())
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
		if !sameCustomerSnapshot(ref.Customer, *customer) {
			return nil, domain.NewBadRequestError("el receptor debe coincidir con el snapshot de la factura referenciada")
		}
		if ref.Cuf == nil || *ref.Cuf == "" {
			return nil, domain.NewConflictError("la factura referenciada aún no tiene cuf; emítala antes de ajustarla")
		}
		if ref.InvoiceNumber <= 0 {
			return nil, domain.NewConflictError("la factura referenciada no tiene número correlativo válido; no se puede crear el ajuste")
		}
		ajustaFacturaId = &ref.ID
	}

	// La fecha de emisión debe expresarse en hora local de Bolivia (UTC-4).
	// En desarrollo (ALLOW_CUSTOM_ISSUE_DATE=true, ambientes PILOTO) se permite
	// fijar issue_date arbitrario para simular ventas offline durante contingencia.
	// En producción este flag debe ser false y se usa now.
	issueDate := now.In(siat.LaPaz)
	if req.IssueDate != nil {
		if !uc.allowCustomIssueDate {
			slog.Warn("issue_date custom rechazado; ALLOW_CUSTOM_ISSUE_DATE=false", "requested", req.IssueDate.Time())
			return nil, domain.NewBadRequestError("issue_date personalizado solo permitido en entorno de desarrollo (ALLOW_CUSTOM_ISSUE_DATE=true)")
		}
		t := req.IssueDate.Time()
		issueDate = t.In(siat.LaPaz)
		slog.Info("issue_date custom aplicado (dev/contingencia)", "requested", t, "effective", issueDate)
	}

	inv := &domain.Invoice{
		CompanyId:             req.CompanyId,
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
		Customer:              *customer,
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
		it.Quantity, it.UnitPrice, it.Discount = siat.NormalizarImportesItem(sector, it.Quantity, it.UnitPrice, it.Discount)
		code := strings.TrimSpace(it.Code)
		if code == "" {
			code = strings.TrimSpace(it.SKU)
		}
		if code == "" {
			return nil, domain.NewBadRequestError("el código del ítem es obligatorio")
		}
		description := strings.TrimSpace(it.Description)
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
		// Multiactividad controlada: valida herencia o existencia en habilitadas (no bloqueante)
		actForItem := ""
		if item.CodigoActividad != nil {
			actForItem = *item.CodigoActividad
		}
		actValidada := uc.validarActividadItem(company, actForItem, index)
		if actValidada != "" {
			item.CodigoActividad = &actValidada
		} else if item.CodigoActividad != nil && strings.TrimSpace(*item.CodigoActividad) == "" {
			// Si validación retornó vacío (empresa sin principal), limpiar
			item.CodigoActividad = nil
		}
		if item.CodigoActividad == nil || strings.TrimSpace(*item.CodigoActividad) == "" ||
			item.CodigoProductoSin == nil || strings.TrimSpace(*item.CodigoProductoSin) == "" ||
			item.UnitCode == nil || *item.UnitCode <= 0 {
			return nil, domain.NewBadRequestError(fmt.Sprintf("ítem %d requiere codigo_actividad, codigo_producto_sin y unit_code; estos valores se guardan como snapshot fiscal", index+1))
		}
		inv.Items = append(inv.Items, item)
		subtotal += itemSubtotal
	}
	inv.Subtotal = round2(subtotal)
	inv.Total = inv.Subtotal
	if req.Total != nil {
		inv.Subtotal = round2(*req.Total)
		inv.Total = inv.Subtotal
	}
	inv.Total, err = siat.TotalDocumento(inv.Subtotal, inv.SectorData)
	if err != nil {
		return nil, domain.NewBadRequestError(err.Error())
	}

	// La dimensión histórica se inserta exclusivamente como parte de este flujo,
	// una vez validado el snapshot completo. La factura copia esos datos y no
	// vuelve a leerlos desde customers.
	if customer.ID == "" {
		if err := uc.customerRepo.Create(customer); err != nil {
			return nil, err
		}
	}
	inv.CustomerId = customer.ID
	inv.Customer = *customer
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

func (uc *InvoiceUsecase) resolveInvoiceSector(invoiceType string, company *domain.Company) (int, error) {
	typeName := strings.ToLower(strings.TrimSpace(invoiceType))
	switch typeName {
	case "credit_note", "debit_note":
		return siat.SectorNotaCreditoDebito, nil
	case "education":
		return siat.SectorEducativo, nil
	case "sale", "":
		actividad := ""
		if company != nil && company.CodigoActividad != nil {
			actividad = strings.TrimSpace(*company.CodigoActividad)
		}
		return uc.resolveDocumentoSector(company.ID, actividad)
	default:
		return 0, domain.NewBadRequestError(fmt.Sprintf("invoice_type %q no soportado", invoiceType))
	}
}

func sameCustomerSnapshot(a, b domain.Customer) bool {
	return strings.EqualFold(strings.TrimSpace(a.DocumentType), strings.TrimSpace(b.DocumentType)) &&
		strings.TrimSpace(a.DocumentNumber) == strings.TrimSpace(b.DocumentNumber) &&
		cadenaOpcional(a.Complement) == cadenaOpcional(b.Complement) &&
		strings.TrimSpace(a.Name) == strings.TrimSpace(b.Name)
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

// actividadesHabilitadas devuelve el conjunto de códigos de actividad
// habilitados para la empresa según el catálogo sincronizado
// actividadesDocumentoSector. Si el repo no está configurado o está vacío,
// retorna al menos la actividad principal de la empresa para no bloquear
// emisión en entornos sin sincronización completa. Usado para validación
// controlada de multiactividad (8549910 principal, 8550100 secundaria, etc.)
// evitando el rechazo 1017.
func (uc *InvoiceUsecase) actividadesHabilitadas(company *domain.Company) map[string]bool {
	habilitadas := make(map[string]bool)
	if uc.docSectorRepo != nil && company != nil {
		if items, err := uc.docSectorRepo.List(company.ID); err == nil {
			for _, it := range items {
				code := strings.TrimSpace(it.CodigoActividad)
				if code != "" {
					habilitadas[code] = true
				}
			}
		}
	}
	// Siempre incluir la actividad principal como fallback (padrón SIN)
	if company != nil && company.CodigoActividad != nil {
		principal := strings.TrimSpace(*company.CodigoActividad)
		if principal != "" {
			habilitadas[principal] = true
		}
	}
	return habilitadas
}

// validarActividadItem verifica que la actividad del ítem esté habilitada;
// si no viene definida hereda la principal. No bloquea (Warn) para permitir
// multiactividad controlada registrada en padrón.
func (uc *InvoiceUsecase) validarActividadItem(company *domain.Company, itemActividad string, idx int) string {
	actividadPrincipal := ""
	if company != nil && company.CodigoActividad != nil {
		actividadPrincipal = strings.TrimSpace(*company.CodigoActividad)
	}
	act := strings.TrimSpace(itemActividad)
	if act == "" {
		// Hereda por defecto la actividad principal
		return actividadPrincipal
	}
	habilitadas := uc.actividadesHabilitadas(company)
	if len(habilitadas) > 0 && !habilitadas[act] {
		slog.Warn("multiactividad: actividad del ítem no está en habilitadas del padrón, se permite con advertencia (evitar 1017)",
			"item", idx+1, "actividad_item", act, "actividad_principal", actividadPrincipal,
			"habilitadas", habilitadas)
		// No se bloquea: empresa puede tener actividad secundaria registrada fuera de docSector (ej. 8550100 consultoría)
		// pero se deja traza para sincronizar catálogos.
	}
	return act
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
