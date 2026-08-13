package usecase

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type InvoiceUsecase struct {
	invoiceRepo  domain.InvoiceRepository
	customerRepo domain.CustomerRepository
	companyRepo  domain.CompanyRepository
	posRepo      domain.PointOfSaleRepository
	catalogRepo  domain.CatalogRepository
	siatService  SiatEmissionService
	modalidad    int
}

func NewInvoiceUsecase(invoiceRepo domain.InvoiceRepository, customerRepo domain.CustomerRepository, companyRepo domain.CompanyRepository, posRepo domain.PointOfSaleRepository, catalogRepo domain.CatalogRepository, siatService SiatEmissionService, modalidad int) *InvoiceUsecase {
	return &InvoiceUsecase{
		invoiceRepo:  invoiceRepo,
		customerRepo: customerRepo,
		companyRepo:  companyRepo,
		posRepo:      posRepo,
		catalogRepo:  catalogRepo,
		siatService:  siatService,
		modalidad:    modalidad,
	}
}

type CreateInvoiceItemRequest struct {
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
	CompanyId        string                     `json:"company_id"`
	PointOfSaleId    string                     `json:"point_of_sale_id"`
	CustomerId       string                     `json:"customer_id"`
	CodigoMetodoPago int                        `json:"codigo_metodo_pago,omitempty"`
	CodigoMoneda     int                        `json:"codigo_moneda,omitempty"`
	TipoCambio       float64                    `json:"tipo_cambio,omitempty"`
	IssueDate        *time.Time                 `json:"issue_date,omitempty"`
	Items            []CreateInvoiceItemRequest `json:"items"`
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
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

	if _, err := uc.companyRepo.GetByID(req.CompanyId); err != nil {
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
	issueDate := now.UTC()
	if req.IssueDate != nil {
		issueDate = req.IssueDate.UTC()
	}

	inv := &domain.Invoice{
		CompanyId:        req.CompanyId,
		CustomerId:       req.CustomerId,
		PointOfSaleId:    req.PointOfSaleId,
		CufdId:           activeCufd.ID,
		EmissionType:     "EN_LINEA",
		CodigoMetodoPago: metodoPago,
		CodigoMoneda:     moneda,
		TipoCambio:       tipoCambio,
		IssueDate:        issueDate,
		Status:           domain.InvoicePending,
	}

	var subtotal float64
	for _, it := range req.Items {
		if strings.TrimSpace(it.Code) == "" {
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
			Code:              it.Code,
			Description:       it.Description,
			CodigoActividad:   it.CodigoActividad,
			CodigoProductoSin: it.CodigoProductoSin,
			UnitCode:          it.UnitCode,
			Quantity:          it.Quantity,
			UnitPrice:         it.UnitPrice,
			Discount:          it.Discount,
			Subtotal:          itemSubtotal,
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

func (uc *InvoiceUsecase) ListByPointOfSale(pointOfSaleID string) ([]*domain.Invoice, error) {
	return uc.invoiceRepo.ListByPointOfSale(pointOfSaleID)
}
