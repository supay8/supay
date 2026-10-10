package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

// ErrSiatNoDisponible indica que el adaptador SIAT no pudo inicializarse
// (configuración ausente o inválida). La capa delivery lo mapea a 503.
var ErrSiatNoDisponible = errors.New("el servicio SIAT no está disponible")

// SiatUsecase concentra la lógica de negocio de las operaciones SIAT que no
// son emisión individual de facturas: códigos (CUIS/CUFD), eventos
// significativos, paquetes/lotes, compras, sincronización de catálogos y
// documentos de ajuste. Los handlers HTTP solo decodifican/encodean.
type SiatUsecase struct {
	companyRepo     ports.CompanyRepository
	pointOfSaleRepo ports.PointOfSaleRepository
	cufdRepo        ports.CufdRepository
	tipoPVRepo      ports.TipoPuntoVentaRepository
	catalogRepo     ports.CatalogRepository
	sinProductRepo  ports.SinProductRepository
	syncStateRepo   ports.CatalogSyncStateRepository
	contingencyRepo ports.ContingencyEventRepository
	sentPackageRepo ports.SentPackageRepository
	actividadRepo   ports.SiatActividadRepository
	leyendaRepo     ports.SiatLeyendaRepository
	docSectorRepo   ports.SiatActividadDocSectorRepository
	invoiceRepo     ports.InvoiceRepository
	fileService     *InvoiceFileService
	emailDispatcher ports.InvoiceEmailTaskDispatcher

	siatService  ports.FiscalOperations
	siatProvider ports.FiscalServiceProvider
	credentials  *CredentialService
	modalidad    int
}

func (uc *SiatUsecase) SetFileService(files *InvoiceFileService) { uc.fileService = files }

func (uc *SiatUsecase) SetEmailDispatcher(dispatcher ports.InvoiceEmailTaskDispatcher) {
	uc.emailDispatcher = dispatcher
}

func NewSiatUsecase(
	companyRepo ports.CompanyRepository,
	pointOfSaleRepo ports.PointOfSaleRepository,
	cufdRepo ports.CufdRepository,
	tipoPVRepo ports.TipoPuntoVentaRepository,
	catalogRepo ports.CatalogRepository,
	contingencyRepo ports.ContingencyEventRepository,
	sentPackageRepo ports.SentPackageRepository,
	siatService ports.FiscalOperations,
	modalidad int,
	sinProductRepo ports.SinProductRepository,
	syncStateRepo ports.CatalogSyncStateRepository,
	actividadRepo ports.SiatActividadRepository,
	leyendaRepo ports.SiatLeyendaRepository,
	docSectorRepo ports.SiatActividadDocSectorRepository,
	invoiceRepo ports.InvoiceRepository,
	siatProvider ports.FiscalServiceProvider,
) *SiatUsecase {
	uc := &SiatUsecase{
		companyRepo:     companyRepo,
		pointOfSaleRepo: pointOfSaleRepo,
		cufdRepo:        cufdRepo,
		tipoPVRepo:      tipoPVRepo,
		catalogRepo:     catalogRepo,
		contingencyRepo: contingencyRepo,
		sentPackageRepo: sentPackageRepo,
		siatService:     siatService,
		modalidad:       modalidad,
		sinProductRepo:  sinProductRepo,
		syncStateRepo:   syncStateRepo,
		actividadRepo:   actividadRepo,
		leyendaRepo:     leyendaRepo,
		docSectorRepo:   docSectorRepo,
		invoiceRepo:     invoiceRepo,
		siatProvider:    siatProvider,
	}
	// El servicio de credenciales comparte repos y SDK con el usecase; el
	// cliente se inyecta solo si el SDK está inicializado para que un nil
	// tipado no evada el guard interno.
	var credClient ports.FiscalOperations
	if siatService != nil {
		credClient = siatService
	}
	uc.credentials = NewCredentialService(pointOfSaleRepo, cufdRepo, credClient, modalidad)
	if siatProvider != nil && uc.credentials != nil {
		uc.credentials.SetProvider(siatProvider)
	}
	return uc
}

func (uc *SiatUsecase) requireService() error {
	if uc.siatProvider != nil {
		return nil
	}
	if uc.siatService == nil {
		return ErrSiatNoDisponible
	}
	return nil
}

func (uc *SiatUsecase) resolveService(ctx context.Context, companyID string) (ports.FiscalOperations, error) {
	if uc.siatProvider != nil && companyID != "" {
		if svc, err := uc.siatProvider.GetForCompany(ctx, companyID); err == nil {
			return svc, nil
		} else if uc.siatService == nil {
			return nil, err
		}
	}
	if uc.siatService == nil {
		return nil, ErrSiatNoDisponible
	}
	return uc.siatService, nil
}

func (uc *SiatUsecase) effectiveModalidadForCompany(company *domain.Company) int {
	if company != nil && company.Modalidad != 0 {
		return company.Modalidad
	}
	return uc.effectiveModalidad()
}

func (uc *SiatUsecase) actividadesHabilitadas(company *domain.Company) map[string]bool {
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
	if company != nil && company.CodigoActividad != nil {
		principal := strings.TrimSpace(*company.CodigoActividad)
		if principal != "" {
			habilitadas[principal] = true
		}
	}
	return habilitadas
}

// LoadCompanyAndPointOfSale valida que el punto de venta pertenezca a la empresa.
func (uc *SiatUsecase) LoadCompanyAndPointOfSale(companyID, pointOfSaleID string) (*domain.Company, *domain.PointOfSale, error) {
	if companyID == "" || pointOfSaleID == "" {
		return nil, nil, domain.NewBadRequestError("companyId y pointOfSaleId son obligatorios")
	}

	company, err := uc.companyRepo.GetByID(companyID)
	if err != nil {
		return nil, nil, domain.NewNotFoundError("Empresa no encontrada")
	}

	pointOfSale, err := uc.pointOfSaleRepo.GetByID(pointOfSaleID)
	if err != nil {
		return nil, nil, domain.NewNotFoundError("Punto de venta no encontrado")
	}

	if pointOfSale.CompanyId != company.ID {
		return nil, nil, domain.NewBadRequestError("El punto de venta no pertenece a la empresa indicada")
	}

	return company, pointOfSale, nil
}

func (uc *SiatUsecase) LoadInvoicesIDs(companyID string, invoiceIDs []string) ([]*domain.Invoice, error) {
	invoices, err := uc.invoiceRepo.GetByIDs(companyID, invoiceIDs)
	if err != nil {
		return nil, domain.NewBadRequestError("Error al obtener facturas por IDs: " + err.Error())
	}
	if len(invoices) != len(invoiceIDs) {
		return nil, domain.NewBadRequestError("No se encontraron todas las facturas por IDs proporcionadas")
	}

	return invoices, nil
}

// resolveCodigoPuntoVenta prefiere el código registrado ante el SIAT.
func resolveCodigoPuntoVenta(pos *domain.PointOfSale) int {
	if pos.SiatCode != nil {
		return *pos.SiatCode
	}
	return pos.CodigoPuntoVenta
}

func (uc *SiatUsecase) effectiveModalidad() int {
	if uc.modalidad <= 0 {
		return fiscal.ModalidadElectronica
	}
	return uc.modalidad
}
