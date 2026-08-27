package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/siat"
)

// ErrSiatNoDisponible indica que el adaptador SIAT no pudo inicializarse
// (configuración ausente o inválida). La capa delivery lo mapea a 503.
var ErrSiatNoDisponible = errors.New("el servicio SIAT no está disponible")

// SiatUsecase concentra la lógica de negocio de las operaciones SIAT que no
// son emisión individual de facturas: códigos (CUIS/CUFD), eventos
// significativos, paquetes/lotes, compras, sincronización de catálogos y
// documentos de ajuste. Los handlers HTTP solo decodifican/encodean.
type SiatUsecase struct {
	companyRepo     domain.CompanyRepository
	pointOfSaleRepo domain.PointOfSaleRepository
	cufdRepo        domain.CufdRepository
	tipoPVRepo      domain.TipoPuntoVentaRepository
	catalogRepo     domain.CatalogRepository
	sinProductRepo  domain.SinProductRepository
	syncStateRepo   domain.CatalogSyncStateRepository
	contingencyRepo domain.ContingencyEventRepository
	sentPackageRepo domain.SentPackageRepository
	actividadRepo   domain.SiatActividadRepository
	leyendaRepo     domain.SiatLeyendaRepository
	docSectorRepo   domain.SiatActividadDocSectorRepository

	siatService *siat.Service
	credentials *CredentialService
	modalidad   int
}

func NewSiatUsecase(
	companyRepo domain.CompanyRepository,
	pointOfSaleRepo domain.PointOfSaleRepository,
	cufdRepo domain.CufdRepository,
	tipoPVRepo domain.TipoPuntoVentaRepository,
	catalogRepo domain.CatalogRepository,
	contingencyRepo domain.ContingencyEventRepository,
	sentPackageRepo domain.SentPackageRepository,
	siatService *siat.Service,
	modalidad int,
	extraRepos ...any,
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
	}
	// El servicio de credenciales comparte repos y SDK con el usecase; el
	// cliente se inyecta solo si el SDK está inicializado para que un nil
	// tipado no evada el guard interno.
	var credClient SiatCredentialClient
	if siatService != nil {
		credClient = siatService
	}
	uc.credentials = NewCredentialService(pointOfSaleRepo, cufdRepo, credClient, modalidad)
	for _, repo := range extraRepos {
		switch typed := repo.(type) {
		case domain.SinProductRepository:
			uc.sinProductRepo = typed
		case domain.CatalogSyncStateRepository:
			uc.syncStateRepo = typed
		case domain.SiatActividadRepository:
			uc.actividadRepo = typed
		case domain.SiatLeyendaRepository:
			uc.leyendaRepo = typed
		case domain.SiatActividadDocSectorRepository:
			uc.docSectorRepo = typed
		}
	}
	return uc
}

func (uc *SiatUsecase) requireService() error {
	if uc.siatService == nil {
		return ErrSiatNoDisponible
	}
	return nil
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

// resolveCodigoPuntoVenta prefiere el código registrado ante el SIAT.
func resolveCodigoPuntoVenta(pos *domain.PointOfSale) int {
	if pos.SiatCode != nil {
		return *pos.SiatCode
	}
	return pos.CodigoPuntoVenta
}

func (uc *SiatUsecase) effectiveModalidad() int {
	if uc.modalidad <= 0 {
		return siat.ModalidadElectronica
	}
	return uc.modalidad
}

// --- CUIS / CUFD ---

type CuisResultado struct {
	Success bool `json:"success"`
	Data    *siat.RespuestaCuis
}

// SolicitarCUIS fuerza la obtención de un CUIS nuevo (endpoint explícito).
func (uc *SiatUsecase) SolicitarCUIS(ctx context.Context, companyID, posID string) (*CuisResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	resp, err := uc.credentials.RefreshCuis(ctx, company, pointOfSale)
	if err != nil {
		return nil, err
	}
	return &CuisResultado{Success: resp.Transaccion, Data: resp}, nil
}

type CufdResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *siat.RespuestaCufd
}

func (uc *SiatUsecase) SolicitarCUFD(ctx context.Context, companyID, posID string) (*CufdResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	resp, _, err := uc.credentials.RefreshCufd(ctx, company, pointOfSale)
	if err != nil {
		return nil, err
	}
	return &CufdResultado{Company: company, PointOfSale: pointOfSale, Response: resp}, nil
}

// SetupResultado es la respuesta de POST /setup: credenciales resueltas,
// resultado de la sincronización y readiness final.
type SetupResultado struct {
	// Cuis es la respuesta del SIAT solo cuando se solicitó uno nuevo;
	// nil cuando el punto de venta ya tenía CUIS.
	Cuis       *siat.RespuestaCuis      `json:"cuis,omitempty"`
	Cufd       *domain.Cufd             `json:"cufd"`
	Operations []SincronizacionOpResult `json:"operations"`
	Errors     []SincronizacionOpError  `json:"errors,omitempty"`
	Readiness  *domain.CatalogReadiness `json:"readiness,omitempty"`
}

// Setup orquesta el alta completa de un punto de venta en una llamada:
// CUIS (lazy) → sincronización de catálogos → CUFD (lazy) → readiness.
// Es idempotente: re-ejecutarla reutiliza las credenciales vigentes y
// refresca los catálogos.
func (uc *SiatUsecase) Setup(ctx context.Context, companyID, posID string) (*SetupResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}

	out := &SetupResultado{}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		resp, err := uc.credentials.RefreshCuis(ctx, company, pointOfSale)
		if err != nil {
			return nil, err
		}
		out.Cuis = resp
	}

	syncRes, err := uc.Sincronizar(ctx, companyID, posID, "")
	if err != nil {
		return nil, err
	}
	out.Operations = syncRes.Operations
	out.Errors = syncRes.Errors

	cufd, err := uc.credentials.EnsureCufd(ctx, company, pointOfSale)
	if err != nil {
		return nil, err
	}
	out.Cufd = cufd

	if readiness, rerr := uc.CatalogReadiness(companyID, posID); rerr == nil {
		out.Readiness = readiness
	}
	return out, nil
}

// --- Evento significativo ---

type EventoSignificativoInput struct {
	CodigoMotivoEvento    int    `json:"codigo_motivo_evento"`
	Descripcion           string `json:"descripcion"`
	CufdEvento            string `json:"cufd_evento"`
	FechaHoraInicioEvento string `json:"fecha_hora_inicio_evento"`
	FechaHoraFinEvento    string `json:"fecha_hora_fin_evento"`
}

type EventoSignificativoResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *siat.ResultadoEventoSignificativo
}

// RegistrarEventoSignificativo registra una contingencia ante el SIAT
// (registroEventoSignificativo). El CUFD vigente se usa como cufdEvento salvo
// que el input lo sobrescriba (p.ej. el CUFD vencido durante la contingencia).
func (uc *SiatUsecase) RegistrarEventoSignificativo(ctx context.Context, companyID, posID string, body EventoSignificativoInput) (*EventoSignificativoResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}
	cufd, err := uc.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, domain.NewConflictError("El punto de venta no tiene CUFD vigente")
	}

	codigoMotivo := body.CodigoMotivoEvento
	if codigoMotivo <= 0 {
		codigoMotivo = siat.MotivoCorteInternet
	}
	descripcion := strings.TrimSpace(body.Descripcion)
	if descripcion == "" {
		descripcion = "Corte del servicio de internet"
	}
	cufdEvento := strings.TrimSpace(body.CufdEvento)
	if cufdEvento == "" {
		cufdEvento = cufd.Cufd
	}

	inicio, err := ParseFechaSiat(body.FechaHoraInicioEvento)
	if err != nil {
		return nil, domain.NewBadRequestError("fechaHoraInicioEvento inválida (use YYYY-MM-DDTHH:mm:ss.SSS)")
	}
	fin, err := ParseFechaSiat(body.FechaHoraFinEvento)
	if err != nil {
		return nil, domain.NewBadRequestError("fechaHoraFinEvento inválida (use YYYY-MM-DDTHH:mm:ss.SSS)")
	}

	req := siat.SolicitudEventoSignificativo{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      resolveCodigoPuntoVenta(pointOfSale),
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CufdEvento:            cufdEvento,
		CodigoMotivoEvento:    codigoMotivo,
		Descripcion:           descripcion,
		FechaHoraInicioEvento: inicio,
		FechaHoraFinEvento:    fin,
	}
	log.Println("req....")
	log.Println(req)
	result, err := uc.siatService.RegistrarEventoSignificativo(ctx, req)
	if err != nil {
		return nil, err
	}

	// Se persiste el evento registrado (con su codigoRecepcion del SIAT) para
	// que el envío de paquetes pueda resolver automáticamente el codigoEvento.
	if uc.contingencyRepo != nil && result != nil && result.Transaccion && result.CodigoRecepcion != "" {
		ev := &domain.ContingencyEvent{
			PointOfSaleID: pointOfSale.ID,
			Reason:        motivoEventoAReason(codigoMotivo),
			Description:   &descripcion,
			StartDate:     inicio,
			EndDate:       &fin,
			SiatEventCode: &result.CodigoRecepcion,
			IsSynced:      true,
		}
		if err := uc.contingencyRepo.Create(ev); err != nil {
			// El evento SIAT ya se registró exitosamente; la persistencia local es
			// complementaria. Un error aquí implica que la resolución automática de
			// codigoEvento no funcionará para envíos posteriores de paquetes.
			slog.Error("no se pudo persistir evento de contingencia",
				"siat_code", result.CodigoRecepcion, "pos_id", pointOfSale.ID, "error", err)
		}
	}

	return &EventoSignificativoResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// motivoEventoAReason traduce el codigoMotivoEvento del catálogo del SIAT a la
// razón de contingencia local (enum de models.ContingencyReason).
func motivoEventoAReason(motivo int) string {
	if motivo == siat.MotivoCorteInternet {
		return string(models.ReasonFallaInternet)
	}
	return string(models.ReasonOtro)
}

// --- Paquetes / lotes / compras / firma ---

type PaqueteInput struct {
	CodigoEvento  int                     `json:"codigoEvento"`
	Descripcion   string                  `json:"descripcion"`
	CodigoEmision int                     `json:"codigoEmision"`
	Archivo       string                  `json:"archivo,omitempty"`
	HashArchivo   string                  `json:"hashArchivo,omitempty"`
	Facturas      []siat.SolicitudFactura `json:"facturas"`
}

type MasivaInput struct {
	CodigoEmision int                     `json:"codigoEmision"`
	Archivo       string                  `json:"archivo,omitempty"`
	HashArchivo   string                  `json:"hashArchivo,omitempty"`
	Facturas      []siat.SolicitudFactura `json:"facturas"`
}

type ComprasInput struct {
	Descripcion      string    `json:"descripcion"`
	TipoCompra       int       `json:"tipoCompra"`
	Archivo          string    `json:"archivo"`
	HashArchivo      string    `json:"hashArchivo"`
	CantidadFacturas int       `json:"cantidadFacturas"`
	Gestion          int       `json:"gestion"`
	Periodo          int       `json:"periodo"`
	FechaEnvio       time.Time `json:"fechaEnvio"`
}

type PaqueteValidacionInput struct {
	CodigoRecepcion string `json:"codigo_recepcion"`
	CodigoEmision   int    `json:"codigo_emision"`
	CodigoDocSector int    `json:"codigo_documento_sector"`
	CodigoTipoFact  int    `json:"codigo_tipo_factura"`
}

type FirmaInput struct {
	Xml string `json:"xml"`
}

type PaqueteResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *siat.ResultadoPaquete
}

type ComprasResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *siat.ResultadoCompras
}

type FirmaResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *siat.ResultadoFirma
}

// buildSolicitudPaquete reúne la identidad del contribuyente (empresa + punto
// de venta + CUIS/CUFD vigente) y la mezcla con los datos específicos del
// paquete enviados en el body (codigoEvento, descripcion, codigoEmision y
// facturas).
func (uc *SiatUsecase) buildSolicitudPaquete(companyID, posID string, body PaqueteInput) (*siat.SolicitudPaqueteFactura, *domain.Company, *domain.PointOfSale, error) {
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, nil, nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, nil, nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}
	cufd, err := uc.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, nil, nil, domain.NewConflictError("El punto de venta no tiene CUFD vigente")
	}

	modalidad := uc.effectiveModalidad()
	var facturasProcesadas []siat.SolicitudFactura
	for _, f := range body.Facturas {
		f.CodigoAmbiente = company.Ambiente.CodigoAmbiente()
		f.CodigoSistema = company.CodigoSistema
		f.Nit = company.Nit
		f.Modalidad = modalidad
		f.CodigoSucursal = pointOfSale.CodigoSucursal
		f.CodigoPuntoVenta = resolveCodigoPuntoVenta(pointOfSale)
		f.Cuis = *pointOfSale.Cuis
		f.Cufd = cufd.Cufd
		f.CodigoControl = cufd.ControlCode

		if f.FechaEmision.IsZero() {
			f.FechaEmision = time.Now().In(siat.LaPaz)
		}
		facturasProcesadas = append(facturasProcesadas, f)
	}

	// Resolver documento-sector desde la primera factura del paquete o desde la
	// empresa (catálogo actividadesDocumentoSector).  El hardcode "1" provocaba
	// que facturas del sector educativo (11) se enviaran con sector incorrecto.
	codigoDocSector := 0
	codigoTipoFact := 0
	if len(facturasProcesadas) > 0 {
		codigoDocSector = facturasProcesadas[0].CodigoDocumentoSector
		codigoTipoFact = facturasProcesadas[0].CodigoTipoFactura
	}
	if codigoDocSector <= 0 {
		codigoDocSector, err = uc.ResolveDocumentoSector(company)
		if err != nil {
			return nil, nil, nil, domain.NewBadRequestError(err.Error())
		}
	}
	if codigoTipoFact <= 0 {
		codigoTipoFact = 1
	}

	req := &siat.SolicitudPaqueteFactura{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		Modalidad:             modalidad,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      resolveCodigoPuntoVenta(pointOfSale),
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		CodigoDocumentoSector: codigoDocSector,
		CodigoTipoFactura:     codigoTipoFact,
		CodigoEmision:         body.CodigoEmision,
		CodigoEvento:          int64(body.CodigoEvento),
		Descripcion:           body.Descripcion,
		Archivo:               body.Archivo,
		HashArchivo:           body.HashArchivo,
		Facturas:              facturasProcesadas,
	}
	return req, company, pointOfSale, nil
}

// EnviarPaquete envía al SIAT un paquete de facturas (recepcionPaqueteFactura),
// en el contexto de un evento significativo registrado (codigoEvento).
func (uc *SiatUsecase) EnviarPaquete(ctx context.Context, companyID, posID string, body PaqueteInput) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if len(body.Facturas) == 0 {
		return nil, domain.NewBadRequestError("El paquete debe contener al menos una factura en el campo facturas")
	}
	req, company, pointOfSale, err := uc.buildSolicitudPaquete(companyID, posID, body)
	if err != nil {
		return nil, err
	}

	// codigoEvento debe ser el codigoRecepcion del último evento significativo
	// registrado en el SIAT; si no se envía, se resuelve automáticamente desde
	// el último evento persistido del punto de venta. Un código inventado hace
	// que el SIAT lo rechace (código 942).
	if req.CodigoEvento <= 0 && uc.contingencyRepo != nil {
		if ev, evErr := uc.contingencyRepo.GetLatestByPointOfSale(pointOfSale.ID); evErr == nil && ev.SiatEventCode != nil {
			if code, cErr := strconv.ParseInt(*ev.SiatEventCode, 10, 64); cErr == nil && code > 0 {
				req.CodigoEvento = code
			}
		}
	}
	if req.CodigoEvento <= 0 {
		return nil, domain.NewBadRequestError("codigoEvento es obligatorio: registre primero un evento significativo (POST /evento-significativo/{companyId}/{pointOfSaleId}) y use el codigoRecepcion de la respuesta, o envíelo vacío para tomar el último evento registrado")
	}

	result, err := uc.siatService.EnviarPaqueteFactura(ctx, *req)
	if err != nil {
		return nil, err
	}
	uc.persistSentPackage(company, pointOfSale, result.CodigoRecepcion, domain.PackageTypePaquete, req.CodigoDocumentoSector, int(req.CodigoEmision), len(req.Facturas))
	return &PaqueteResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// ValidarPaquete consulta al SIAT la validación de un paquete ya enviado
// (validacionRecepcionPaqueteFactura).
func (uc *SiatUsecase) ValidarPaquete(ctx context.Context, companyID, posID string, body PaqueteValidacionInput) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.CodigoRecepcion) == "" {
		return nil, domain.NewBadRequestError("codigoRecepcion es obligatorio (código devuelto por el envío del paquete)")
	}

	req, company, pointOfSale, err := uc.buildSolicitudPaquete(companyID, posID, PaqueteInput{
		CodigoEmision: body.CodigoEmision,
	})
	if err != nil {
		return nil, err
	}
	if body.CodigoDocSector > 0 {
		req.CodigoDocumentoSector = body.CodigoDocSector
	}
	if body.CodigoTipoFact > 0 {
		req.CodigoTipoFactura = body.CodigoTipoFact
	}

	result, err := uc.siatService.ValidarPaqueteFactura(ctx, *req, body.CodigoRecepcion)
	if err != nil {
		return nil, err
	}
	return &PaqueteResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// buildSolicitudMasiva reúne la identidad del contribuyente y los datos del lote.
func (uc *SiatUsecase) buildSolicitudMasiva(companyID, posID string, body MasivaInput) (*siat.SolicitudMasivaFactura, *domain.Company, *domain.PointOfSale, error) {
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, nil, nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, nil, nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}
	cufd, err := uc.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, nil, nil, domain.NewConflictError("El punto de venta no tiene CUFD vigente")
	}

	modalidad := uc.effectiveModalidad()

	// Resolver documento-sector desde la primera factura del lote o desde la
	// empresa (catálogo actividadesDocumentoSector).
	codigoDocSector := 0
	codigoTipoFact := 0
	if len(body.Facturas) > 0 {
		codigoDocSector = body.Facturas[0].CodigoDocumentoSector
		codigoTipoFact = body.Facturas[0].CodigoTipoFactura
	}
	if codigoDocSector <= 0 {
		codigoDocSector, err = uc.ResolveDocumentoSector(company)
		if err != nil {
			return nil, nil, nil, domain.NewBadRequestError(err.Error())
		}
	}
	if codigoTipoFact <= 0 {
		codigoTipoFact = 1
	}

	req := &siat.SolicitudMasivaFactura{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		Modalidad:             modalidad,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      resolveCodigoPuntoVenta(pointOfSale),
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		CodigoDocumentoSector: codigoDocSector,
		CodigoTipoFactura:     codigoTipoFact,
		CodigoEmision:         body.CodigoEmision,
		Archivo:               body.Archivo,
		HashArchivo:           body.HashArchivo,
		Facturas:              body.Facturas,
	}
	return req, company, pointOfSale, nil
}

// EnviarMasiva envía al SIAT un lote de facturas por emisión masiva
// (recepcionMasivaFactura, codigoEmision = 3).
func (uc *SiatUsecase) EnviarMasiva(ctx context.Context, companyID, posID string, body MasivaInput) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if len(body.Facturas) == 0 {
		return nil, domain.NewBadRequestError("El lote debe contener al menos una factura en el campo facturas")
	}

	req, company, pointOfSale, err := uc.buildSolicitudMasiva(companyID, posID, body)
	if err != nil {
		return nil, err
	}
	result, err := uc.siatService.EnviarMasivaFacturas(ctx, *req)
	if err != nil {
		return nil, err
	}
	uc.persistSentPackage(company, pointOfSale, result.CodigoRecepcion, domain.PackageTypeMasiva, req.CodigoDocumentoSector, req.CodigoEmision, len(req.Facturas))
	return &PaqueteResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// ValidarMasiva consulta al SIAT la validación de un lote ya enviado
// (validacionRecepcionMasivaFactura).
func (uc *SiatUsecase) ValidarMasiva(ctx context.Context, companyID, posID string, body PaqueteValidacionInput) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.CodigoRecepcion) == "" {
		return nil, domain.NewBadRequestError("codigoRecepcion es obligatorio (código devuelto por el envío del lote)")
	}

	req, company, pointOfSale, err := uc.buildSolicitudMasiva(companyID, posID, MasivaInput{
		CodigoEmision: body.CodigoEmision,
	})
	if err != nil {
		return nil, err
	}
	if body.CodigoDocSector > 0 {
		req.CodigoDocumentoSector = body.CodigoDocSector
	}
	if body.CodigoTipoFact > 0 {
		req.CodigoTipoFactura = body.CodigoTipoFact
	}

	result, err := uc.siatService.ValidarMasivaFacturas(ctx, *req, body.CodigoRecepcion)
	if err != nil {
		return nil, err
	}
	return &PaqueteResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// EnviarCompras registra en el SIAT un paquete de facturas de compras
// (recepcionPaqueteCompras, Etapa XI). codigoPuntoVenta no aplica en el
// servicio de compras.
func (uc *SiatUsecase) EnviarCompras(ctx context.Context, companyID, posID string, body ComprasInput) (*ComprasResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Archivo) == "" || strings.TrimSpace(body.HashArchivo) == "" {
		return nil, domain.NewBadRequestError("archivo y hashArchivo son obligatorios (Base64 del TAR.GZ y su SHA-256)")
	}

	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}
	cufd, err := uc.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, domain.NewConflictError("El punto de venta no tiene CUFD vigente")
	}

	req := siat.SolicitudCompras{
		Descripcion:      body.Descripcion,
		TipoCompra:       body.TipoCompra,
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoPuntoVenta: 0,
		Cuis:             *pointOfSale.Cuis,
		Cufd:             cufd.Cufd,
		Archivo:          body.Archivo,
		HashArchivo:      body.HashArchivo,
		CantidadFacturas: body.CantidadFacturas,
		Gestion:          body.Gestion,
		Periodo:          body.Periodo,
		FechaEnvio:       body.FechaEnvio,
	}

	result, err := uc.siatService.EnviarCompras(ctx, req)
	if err != nil {
		return nil, err
	}
	uc.persistSentPackage(company, pointOfSale, result.CodigoRecepcion, domain.PackageTypeCompras, 19, 0, body.CantidadFacturas)
	return &ComprasResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// FirmarFactura firma digitalmente el XML de una factura con el certificado de
// la empresa (Etapa VIII - Firma Digital).
func (uc *SiatUsecase) FirmarFactura(ctx context.Context, companyID, posID string, body FirmaInput) (*FirmaResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Xml) == "" {
		return nil, domain.NewBadRequestError("xml es obligatorio (la cadena del XML de la factura a firmar)")
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	result, err := uc.siatService.FirmarFacturaXML(ctx, body.Xml)
	if err != nil {
		return nil, err
	}
	return &FirmaResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// persistSentPackage guarda el registro de auditoría del envío al SIAT. Un
// fallo no revierte el envío pero sí se loguea: perder la trazabilidad fiscal
// (codigoRecepcion ↔ facturas) sin rastro es inaceptable.
func (uc *SiatUsecase) persistSentPackage(company *domain.Company, pos *domain.PointOfSale, codigoRecepcion string, tipo domain.SentPackageType, docSector, codigoEmision, cantidad int) {
	if uc.sentPackageRepo == nil || codigoRecepcion == "" {
		return
	}
	pkg := &domain.SentPackage{
		CompanyId:             company.ID,
		PointOfSaleId:         pos.ID,
		CodigoRecepcion:       codigoRecepcion,
		Type:                  tipo,
		CodigoDocumentoSector: docSector,
		CodigoEmision:         codigoEmision,
		CantidadFacturas:      cantidad,
		Status:                domain.PackageStatusPending,
	}
	if err := uc.sentPackageRepo.Create(pkg); err != nil {
		slog.Error("no se pudo persistir el registro de paquete enviado",
			"codigo_recepcion", codigoRecepcion, "pos_id", pos.ID, "tipo", tipo, "error", err)
	}
}

// --- Sincronización ---

type SincronizacionOpResult struct {
	Operation   string `json:"operation"`
	Transaccion bool   `json:"transaccion"`
	Codigos     int    `json:"codigos"`
	Status      string `json:"status"`
	RowsSaved   int    `json:"rows_saved"`
	FechaHora   string `json:"fechaHora,omitempty"`
}

type SincronizacionOpError struct {
	Operation string `json:"operation"`
	Error     string `json:"error"`
}

type SincronizacionResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Operations  []SincronizacionOpResult
	Errors      []SincronizacionOpError
}

const catalogSyncMaxAge = 24 * time.Hour

// Sincronizar baja catálogos del SIAT. Con opRaw vacío sincroniza todas las
// operaciones del SDK; con ?operation=X solo esa.
func (uc *SiatUsecase) Sincronizar(ctx context.Context, companyID, posID, opRaw string) (*SincronizacionResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}

	// El código de punto de venta que usa el CUIS debe ser el mismo en todas
	// las operaciones (preferir el código registrado ante SIAT).
	req := siat.SolicitudSincronizacion{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoPuntoVenta: pointOfSale.CodigoPuntoVenta,
		Cuis:             *pointOfSale.Cuis,
	}

	out := &SincronizacionResultado{Company: company, PointOfSale: pointOfSale}

	if opRaw != "" {
		op, ok := siat.ParseSincronizacionOp(opRaw)
		if !ok {
			return nil, domain.NewBadRequestError("Operación de sincronización desconocida: " + opRaw)
		}
		result, err := uc.siatService.Sincronizar(ctx, req, op)
		if err != nil {
			_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: company.ID, PointOfSaleID: pointOfSale.ID, Operation: string(op), Status: "FAILED", Error: err.Error()})
			return nil, err
		}
		if perr := uc.PersistSincronizacionAt(company.ID, pointOfSale.ID, op, result); perr != nil {
			return nil, perr
		}
		out.Operations = append(out.Operations, toSincronizacionOpResult(op, result))
		return out, nil
	}
	for _, op := range siat.SincronizacionOperations {
		result, err := uc.siatService.Sincronizar(ctx, req, op)
		if err != nil {
			_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: company.ID, PointOfSaleID: pointOfSale.ID, Operation: string(op), Status: "FAILED", Error: err.Error()})
			out.Errors = append(out.Errors, SincronizacionOpError{Operation: string(op), Error: err.Error()})
			continue
		}
		if perr := uc.PersistSincronizacionAt(company.ID, pointOfSale.ID, op, result); perr != nil {
			out.Errors = append(out.Errors, SincronizacionOpError{Operation: string(op), Error: "persistencia: " + perr.Error()})
		}
		out.Operations = append(out.Operations, toSincronizacionOpResult(op, result))
	}

	return out, nil
}

func toSincronizacionOpResult(op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) SincronizacionOpResult {
	out := SincronizacionOpResult{
		Operation:   string(op),
		Transaccion: res.Transaccion,
		Codigos:     len(res.Codigos),
		Status:      syncStatus(op, res),
		RowsSaved:   syncRows(op, res),
	}
	if !res.FechaHora.IsZero() {
		out.FechaHora = res.FechaHora.Format(time.RFC3339Nano)
	}
	return out
}

func syncRows(op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) int {
	switch op {
	case siat.OpActividades:
		if len(res.Actividades) > 0 {
			return len(res.Actividades)
		}
	case siat.OpLeyendasFactura:
		if len(res.Leyendas) > 0 {
			return len(res.Leyendas)
		}
	case siat.OpActividadesDocumentoSector:
		if len(res.ActividadesDocSector) > 0 {
			return len(res.ActividadesDocSector)
		}
	case siat.OpProductosServicios:
		if len(res.Productos) > 0 {
			return len(res.Productos)
		}
		return len(res.Codigos)
	}
	return len(res.Codigos)
}

func syncStatus(op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) string {
	if !res.Transaccion {
		return "FAILED"
	}
	if isCriticalSyncOperation(op) && syncRows(op, res) == 0 {
		return "EMPTY"
	}
	return "SUCCESS"
}

func isCriticalSyncOperation(op siat.SincronizacionOp) bool {
	switch op {
	case siat.OpActividades, siat.OpProductosServicios, siat.OpActividadesDocumentoSector, siat.OpUnidadMedida, siat.OpTipoMoneda, siat.OpTipoMetodoPago, siat.OpLeyendasFactura:
		return true
	default:
		return false
	}
}

// PersistSincronizacion guarda el resultado de una operación. Los productos
// SIN se guardan en sin_products; los catálogos paramétricos siguen en
// catalogs y los tipos de punto de venta tienen su repositorio dedicado.
func (uc *SiatUsecase) PersistSincronizacion(companyID string, op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) error {
	return uc.PersistSincronizacionAt(companyID, "", op, res)
}

func (uc *SiatUsecase) PersistSincronizacionAt(companyID, pointOfSaleID string, op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) error {
	if res == nil || !res.Transaccion {
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: "FAILED", Error: "SIAT no confirmó la sincronización"})
		return nil
	}
	now := time.Now().In(siat.LaPaz)
	status := syncStatus(op, res)
	rowsSaved := syncRows(op, res)
	if op == siat.OpProductosServicios {
		if uc.sinProductRepo == nil {
			err := errors.New("repositorio de productos SIN no configurado")
			_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: "FAILED", Error: err.Error()})
			return err
		}
		products := make([]domain.SinProduct, 0, len(res.Productos))
		for _, p := range res.Productos {
			products = append(products, domain.SinProduct{CompanyID: companyID, CodigoProductoSin: p.CodigoProductoSin, CodigoActividad: p.CodigoActividad, Descripcion: p.Descripcion, Active: true, SyncedAt: now})
		}
		// Compatibilidad con respuestas construidas por integraciones antiguas
		// que solo llenaban Codigos.
		if len(products) == 0 {
			for _, p := range res.Codigos {
				products = append(products, domain.SinProduct{CompanyID: companyID, CodigoProductoSin: int64(p.CodigoClasificador), Descripcion: p.Descripcion, Active: true, SyncedAt: now})
			}
			rowsSaved = len(products)
		}
		if err := uc.sinProductRepo.Replace(companyID, products, now); err != nil {
			_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: "FAILED", Error: err.Error()})
			return err
		}
		return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
	}

	if op == siat.OpTipoPuntoVenta {
		tipos := make([]domain.TipoPuntoVenta, 0, len(res.Codigos))
		for _, c := range res.Codigos {
			tipos = append(tipos, domain.TipoPuntoVenta{
				CodigoClasificador: c.CodigoClasificador,
				Descripcion:        c.Descripcion,
			})
		}
		if err := uc.tipoPVRepo.Replace(companyID, tipos, now); err != nil {
			_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: "FAILED", Error: err.Error()})
			return err
		}
		return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
	}

	switch op {
	case siat.OpFechaHora, siat.OpVerificarComunicacion:
		return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
	case siat.OpActividades:
		return uc.persistActividades(companyID, pointOfSaleID, op, res, status, rowsSaved, now)
	case siat.OpLeyendasFactura:
		return uc.persistLeyendas(companyID, pointOfSaleID, op, res, status, rowsSaved, now)
	case siat.OpActividadesDocumentoSector:
		return uc.persistActividadesDocSector(companyID, pointOfSaleID, op, res, status, rowsSaved, now)
	}

	items := make([]domain.CatalogItem, 0, len(res.Codigos))
	for _, c := range res.Codigos {
		items = append(items, domain.CatalogItem{
			Codigo:      c.CodigoClasificador,
			Descripcion: c.Descripcion,
			Tipo:        string(op),
		})
	}
	if err := uc.catalogRepo.Replace(companyID, string(op), items, now); err != nil {
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: "FAILED", Error: err.Error()})
		return err
	}
	return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
}

// persistActividades guarda el catálogo CAEB completo (código + descripción +
// tipo de actividad) en su tabla dedicada.
func (uc *SiatUsecase) persistActividades(companyID, posID string, op siat.SincronizacionOp, res *siat.RespuestaSincronizacion, status string, rowsSaved int, now time.Time) error {
	if uc.actividadRepo == nil {
		err := errors.New("repositorio de actividades no configurado")
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: "FAILED", Error: err.Error()})
		return err
	}
	items := make([]domain.SiatActividad, 0, len(res.Actividades))
	for _, a := range res.Actividades {
		items = append(items, domain.SiatActividad{
			CodigoCaeb:    strings.TrimSpace(a.CodigoCaeb),
			Descripcion:   a.Descripcion,
			TipoActividad: a.TipoActividad,
		})
	}
	if err := uc.actividadRepo.Replace(companyID, items, now); err != nil {
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: "FAILED", Error: err.Error()})
		return err
	}
	return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
}

// persistLeyendas guarda las leyendas de factura asociadas por actividad en su
// tabla dedicada.
func (uc *SiatUsecase) persistLeyendas(companyID, posID string, op siat.SincronizacionOp, res *siat.RespuestaSincronizacion, status string, rowsSaved int, now time.Time) error {
	if uc.leyendaRepo == nil {
		err := errors.New("repositorio de leyendas no configurado")
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: "FAILED", Error: err.Error()})
		return err
	}
	items := make([]domain.SiatLeyenda, 0, len(res.Leyendas))
	for _, l := range res.Leyendas {
		items = append(items, domain.SiatLeyenda{
			CodigoActividad:    l.CodigoActividad,
			DescripcionLeyenda: l.DescripcionLeyenda,
		})
	}
	if err := uc.leyendaRepo.Replace(companyID, items, now); err != nil {
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: "FAILED", Error: err.Error()})
		return err
	}
	return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
}

// persistActividadesDocSector guarda la relación actividad ↔ documento-sector
// con sus columnas reales; es la fuente para resolver el sector de emisión.
func (uc *SiatUsecase) persistActividadesDocSector(companyID, posID string, op siat.SincronizacionOp, res *siat.RespuestaSincronizacion, status string, rowsSaved int, now time.Time) error {
	if uc.docSectorRepo == nil {
		err := errors.New("repositorio actividadesDocumentoSector no configurado")
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: "FAILED", Error: err.Error()})
		return err
	}
	items := make([]domain.SiatActividadDocSector, 0, len(res.ActividadesDocSector))
	for _, r := range res.ActividadesDocSector {
		items = append(items, domain.SiatActividadDocSector{
			CodigoActividad:       strings.TrimSpace(r.CodigoActividad),
			CodigoDocumentoSector: r.CodigoDocumentoSector,
			TipoDocumentoSector:   r.TipoDocumentoSector,
		})
	}
	if err := uc.docSectorRepo.Replace(companyID, items, now); err != nil {
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: "FAILED", Error: err.Error()})
		return err
	}
	return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: posID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
}

func (uc *SiatUsecase) saveSyncState(state domain.CatalogSyncState) error {
	if uc.syncStateRepo == nil || state.PointOfSaleID == "" {
		return nil
	}
	return uc.syncStateRepo.Upsert(state)
}

func (uc *SiatUsecase) CatalogReadiness(companyID, pointOfSaleID string) (*domain.CatalogReadiness, error) {
	if uc.syncStateRepo == nil {
		return nil, errors.New("repositorio de estado de sincronización no configurado")
	}
	states, err := uc.syncStateRepo.List(companyID, pointOfSaleID)
	if err != nil {
		return nil, err
	}
	required := []siat.SincronizacionOp{siat.OpActividades, siat.OpProductosServicios, siat.OpActividadesDocumentoSector, siat.OpUnidadMedida, siat.OpTipoMoneda, siat.OpTipoMetodoPago, siat.OpLeyendasFactura}
	byOperation := make(map[string]domain.CatalogSyncState, len(states))
	outStates := make([]domain.CatalogSyncState, 0, len(states))
	for _, state := range states {
		if state.Status == "SUCCESS" && (state.SyncedAt == nil || time.Since(*state.SyncedAt) > catalogSyncMaxAge) {
			state.Status = "STALE"
		}
		byOperation[state.Operation] = *state
		outStates = append(outStates, *state)
	}
	missing := make([]string, 0)
	for _, operation := range required {
		state, ok := byOperation[string(operation)]
		if !ok || state.Status != "SUCCESS" {
			missing = append(missing, string(operation))
		}
	}
	return &domain.CatalogReadiness{Ready: len(missing) == 0, Missing: missing, States: outStates}, nil
}

func (uc *SiatUsecase) ListSinProducts(companyID, query string, limit, offset int) ([]*domain.SinProduct, int64, error) {
	res, err := uc.ListProductosSinQuery(companyID, query, 0, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return res.Items, res.Total, nil
}

// ListActivitesDocumentSectors mantiene el contrato legado (paginado en memoria).
func (uc *SiatUsecase) ListActivitesDocumentSectors(companyID, query string, limit, offset int) ([]*domain.SiatActividadDocSector, int64, error) {
	if uc.docSectorRepo == nil {
		return nil, 0, errors.New("repositorio actividadesDocumentoSector no configurado")
	}
	items, err := uc.docSectorRepo.List(companyID)
	if err != nil {
		return nil, 0, err
	}
	term := strings.TrimSpace(strings.ToLower(query))
	if term != "" {
		filtered := make([]*domain.SiatActividadDocSector, 0, len(items))
		for _, it := range items {
			haystack := strings.ToLower(it.CodigoActividad + " " + it.TipoDocumentoSector + " " + strconv.Itoa(it.CodigoDocumentoSector))
			if strings.Contains(haystack, term) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	total := int64(len(items))
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*domain.SiatActividadDocSector{}, total, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], total, nil
}

// --- Lectura de catálogos sincronizados ---

// CatalogoResultado agrupa los elementos de un catálogo sincronizado para el
// endpoint de lectura. Items es polimórfico: paramétricas devuelven
// {codigo, descripcion}, actividades {codigo_caeb, descripcion,
// tipo_actividad}, etc., fiel a la estructura que entrega el SIAT.
type CatalogoResultado struct {
	Tipo     string `json:"tipo"`
	Cantidad int    `json:"cantidad"`
	Items    []any  `json:"items"`
}

func toCatalogoResultado(tipo string, items []any) *CatalogoResultado {
	return &CatalogoResultado{Tipo: tipo, Cantidad: len(items), Items: items}
}

// ListCatalog devuelve el catálogo sincronizado de la empresa para el tipo
// indicado (p.ej. "actividades", "leyendasFactura", "tipoMoneda"). Con tipo
// vacío o "all" devuelve todos los catálogos almacenados agrupados por tipo.
func (uc *SiatUsecase) ListCatalog(companyID, tipo string) (any, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("companyId es obligatorio")
	}
	tipo = strings.TrimSpace(tipo)
	if tipo == "" || strings.EqualFold(tipo, "all") {
		return uc.ListAllCatalogs(companyID)
	}
	items, err := uc.listCatalogByTipo(companyID, tipo)
	if err != nil {
		return nil, err
	}
	return toCatalogoResultado(tipo, items), nil
}

// ListAllCatalogs consulta cada catálogo almacenado; un catálogo sin repos
// configurado o con error se omite (la respuesta incluye lo disponible).
func (uc *SiatUsecase) ListAllCatalogs(companyID string) (any, error) {
	out := make(map[string]*CatalogoResultado)
	for _, op := range siat.SincronizacionOperations {
		switch op {
		case siat.OpFechaHora, siat.OpVerificarComunicacion:
			continue // operativos, no almacenan catálogo
		}
		items, err := uc.listCatalogByTipo(companyID, string(op))
		if err != nil {
			continue
		}
		out[string(op)] = toCatalogoResultado(string(op), items)
	}
	return out, nil
}

func (uc *SiatUsecase) listCatalogByTipo(companyID, tipo string) ([]any, error) {
	op := siat.SincronizacionOp(tipo)
	switch op {
	case siat.OpActividades:
		if uc.actividadRepo == nil {
			return nil, errors.New("catálogo de actividades no disponible")
		}
		items, err := uc.actividadRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case siat.OpLeyendasFactura:
		if uc.leyendaRepo == nil {
			return nil, errors.New("catálogo de leyendas no disponible")
		}
		items, err := uc.leyendaRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case siat.OpActividadesDocumentoSector:
		if uc.docSectorRepo == nil {
			return nil, errors.New("catálogo actividadesDocumentoSector no disponible")
		}
		items, err := uc.docSectorRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case siat.OpProductosServicios:
		if uc.sinProductRepo == nil {
			return nil, errors.New("catálogo de productos SIN no disponible")
		}
		items, err := uc.sinProductRepo.ListAll(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case siat.OpTipoPuntoVenta:
		if uc.tipoPVRepo == nil {
			return nil, errors.New("catálogo de tipos de punto de venta no disponible")
		}
		items, err := uc.tipoPVRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil
	}

	if _, ok := siat.ParseSincronizacionOp(tipo); !ok {
		return nil, domain.NewBadRequestError("Catálogo desconocido: " + tipo)
	}
	if op == siat.OpFechaHora || op == siat.OpVerificarComunicacion {
		return nil, domain.NewBadRequestError("La operación " + tipo + " no almacena catálogo")
	}
	parametricas, err := uc.catalogRepo.List(companyID, tipo)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(parametricas))
	for i, item := range parametricas {
		out[i] = item
	}
	return out, nil
}

// ResolveDocumentoSector determina el documento-sector del SIAT para la
// actividad económica de la empresa consultando el catálogo sincronizado
// actividadesDocumentoSector (tabla siat_actividades_doc_sector). Prefiere la
// factura de compraventa (FCV); si la actividad solo está asociada a sectores
// educativos (FSEDU), usa ese sector.
func (uc *SiatUsecase) ResolveDocumentoSector(company *domain.Company) (int, error) {
	if company == nil {
		return 0, errors.New("no se puede resolver documento-sector sin empresa")
	}
	if uc.docSectorRepo == nil {
		return 0, errors.New("no existe repositorio de actividadesDocumentoSector; sincronice la lista de actividades")
	}
	if company.CodigoActividad == nil {
		return 0, fmt.Errorf("la empresa %s no tiene codigo_actividad; sincronice actividadesDocumentoSector", company.ID)
	}
	actividad := strings.TrimSpace(*company.CodigoActividad)
	if actividad == "" {
		return 0, fmt.Errorf("la empresa %s no tiene codigo_actividad; sincronice actividadesDocumentoSector", company.ID)
	}
	items, err := uc.docSectorRepo.ListByActividad(company.ID, actividad)
	if err != nil {
		return 0, fmt.Errorf("no se pudo resolver documento-sector para actividad %s: %w", actividad, err)
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("actividad %s no sincronizada en actividadesDocumentoSector; ejecute la sincronizacion antes de emitir", actividad)
	}
	found := 0
	for _, item := range items {
		switch strings.TrimSpace(item.TipoDocumentoSector) {
		case "FCV":
			if item.CodigoDocumentoSector > 0 {
				return item.CodigoDocumentoSector, nil
			}
		case "FSEDU":
			found = item.CodigoDocumentoSector
		}
	}
	if found > 0 {
		return found, nil
	}
	return 0, fmt.Errorf("actividad %s no tiene una relacion de documento-sector soportada; sincronice nuevamente el catalogo", actividad)
}

// --- Documentos de ajuste (NC/ND) ---

type DocumentoAjusteInput struct {
	NumeroFactura         int64               `json:"numeroFactura"`
	CufFacturaOriginal    string              `json:"cufFacturaOriginal"`
	CodigoDocumentoSector int                 `json:"codigoDocumentoSector"`
	Layout                string              `json:"layout,omitempty"`
	CodigoTipoFactura     int                 `json:"codigoTipoFactura"`
	TipoNota              int                 `json:"tipoNota"`
	Motivo                string              `json:"motivo"`
	CodigoMetodoPago      int                 `json:"codigoMetodoPago"`
	CodigoMoneda          int                 `json:"codigoMoneda"`
	TipoCambio            float64             `json:"tipoCambio"`
	MontoTotal            float64             `json:"montoTotal"`
	Leyenda               string              `json:"leyenda"`
	Cliente               siat.ClienteFactura `json:"cliente"`
	Items                 []siat.ItemFactura  `json:"items"`
}

type DocumentoAjusteResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *siat.ResultadoDocumentoAjuste
}

// EmitirDocumentoAjuste emite un documento de ajuste (nota de crédito o
// débito) ante el SIAT. La identidad SIAT (CUIS/CUFD/código de control/
// modalidad) se resuelve SIEMPRE desde la base de datos — nunca del request —
// para impedir emitir NC/ND con un CUFD vencido o de otro punto de venta.
func (uc *SiatUsecase) EmitirDocumentoAjuste(ctx context.Context, companyID, posID string, body DocumentoAjusteInput) (*DocumentoAjusteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.CufFacturaOriginal) == "" {
		return nil, domain.NewBadRequestError("cufFacturaOriginal es obligatorio para documentos de ajuste")
	}
	if body.TipoNota != 1 && body.TipoNota != 2 {
		return nil, domain.NewBadRequestError("tipoNota debe ser 1 (nota de crédito) o 2 (nota de débito)")
	}

	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}
	cufd, err := uc.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, domain.NewConflictError("El punto de venta no tiene CUFD vigente")
	}

	usuario := "SUPAY"
	if company.UsuarioSiat != "" {
		usuario = company.UsuarioSiat
	}

	req := siat.SolicitudDocumentoAjuste{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		Modalidad:             uc.effectiveModalidad(),
		NumeroFactura:         body.NumeroFactura,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      pointOfSale.CodigoPuntoVenta,
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		FechaEmision:          time.Now().In(siat.LaPaz),
		Usuario:               usuario,
		TipoNota:              siat.TipoNota(body.TipoNota),
		CufFacturaOriginal:    body.CufFacturaOriginal,
		CodigoDocumentoSector: body.CodigoDocumentoSector,
		Layout:                body.Layout,
		CodigoTipoFactura:     body.CodigoTipoFactura,
		RazonSocialEmisor:     company.BusinessName,
		Municipio:             company.Municipio,
		Direccion:             company.Direccion,
		Telefono:              &company.Telefono,
		Cliente:               body.Cliente,
		CodigoMetodoPago:      body.CodigoMetodoPago,
		CodigoMoneda:          body.CodigoMoneda,
		TipoCambio:            body.TipoCambio,
		MontoTotal:            body.MontoTotal,
		Leyenda:               body.Leyenda,
		Motivo:                body.Motivo,
		Items:                 body.Items,
	}

	result, err := uc.siatService.EmitirDocumentoAjuste(ctx, req)
	if err != nil {
		return nil, err
	}
	return &DocumentoAjusteResultado{Company: company, PointOfSale: pointOfSale, Response: result}, nil
}

// ParseFechaSiat parsea una fecha/hora del SIAT en formato UTC extendido sin
// zona horaria (YYYY-MM-DDTHH:mm:ss.SSS). Si el valor está vacío usa la fecha
// y hora actual en Bolivia.
func ParseFechaSiat(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().In(siat.LaPaz), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04:05.000", value, siat.LaPaz)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}
