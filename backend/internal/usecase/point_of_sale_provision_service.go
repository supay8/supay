package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	siat "github.com/brandsrx/supay/internal/siat"
	"gorm.io/datatypes"
)

type CreatePOSInput struct {
	LocalCode            *int   `json:"local_code,omitempty"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	CodigoTipoPuntoVenta int    `json:"codigo_tipo_punto_venta"`
}

type PointOfSaleProvisionService struct {
	branchRepo        domain.BranchRepository
	posRepo           domain.PointOfSaleRepository
	companyRepo       domain.CompanyRepository
	cuisSvc           *siat.CuisService
	cufdSvc           *siat.CufdService
	cuisRepo          domain.CuisRepository
	cufdRepo          domain.CufdRepository
	tipoPVRepo        domain.TipoPuntoVentaRepository
	posSvc            *siat.PuntoVentaService
	sincronizacionSvc *siat.SincronizacionService
	modalidad         int
}

func NewPointOfSaleProvisionService(
	branchRepo domain.BranchRepository,
	posRepo domain.PointOfSaleRepository,
	companyRepo domain.CompanyRepository,
	siatClient *siat.Client,
	cuisSvc *siat.CuisService,
	cufdSvc *siat.CufdService,
	cuisRepo domain.CuisRepository,
	cufdRepo domain.CufdRepository,
	tipoPVRepo domain.TipoPuntoVentaRepository,
	modalidad int,
) *PointOfSaleProvisionService {
	return &PointOfSaleProvisionService{
		branchRepo:        branchRepo,
		posRepo:           posRepo,
		companyRepo:       companyRepo,
		cuisSvc:           cuisSvc,
		cufdSvc:           cufdSvc,
		cuisRepo:          cuisRepo,
		cufdRepo:          cufdRepo,
		tipoPVRepo:        tipoPVRepo,
		posSvc:            siat.NewPuntoVentaService(siatClient),
		sincronizacionSvc: siat.NewSincronizacionService(siatClient),
		modalidad:         modalidad,
	}
}

// CreateOperationalPOS implementa el alta de un punto de venta con registro oficial
// ante el SIAT:
//
//  1. Crea el punto de venta local en estado CREATING (empresa y codigoSucursal
//     se derivan de la sucursal).
//  2. Paso A: obtiene el CUIS de la sucursal (codigoPuntoVenta=0, transitorio)
//     y sincroniza el catálogo de tipos de punto de venta
//     (sincronizarParametricaTipoPuntoVenta).
//  3. Paso A: valida el codigoTipoPuntoVenta contra el catálogo oficial.
//  4. Paso B: envía la solicitud SOAP registroPuntoVenta.
//  5. Paso C: solo si el SIAT confirma (transaccion=true y codigoPuntoVenta>0)
//     persiste el código oficial, habilita el punto de venta y solicita el CUIS
//     propio del punto de venta (codigoPuntoVenta oficial) y su CUFD.
//     Si el SIAT rechaza, no se pide CUIS/CUFD para ese punto.
func (s *PointOfSaleProvisionService) CreateOperationalPOS(ctx context.Context, branchID string, input CreatePOSInput) (*domain.PointOfSale, error) {
	// 1. Verify branch and company
	branch, err := s.branchRepo.GetByID(branchID)
	if err != nil {
		return nil, fmt.Errorf("branch not found: %w", err)
	}
	company, err := s.companyRepo.GetByID(branch.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("company lookup: %w", err)
	}

	codigoSucursal := branch.CodigoSucursal
	nombrePuntoVenta := strings.TrimSpace(input.Name)
	if nombrePuntoVenta == "" {
		return nil, fmt.Errorf("name es obligatorio")
	}
	descripcion := strings.TrimSpace(input.Description)
	if descripcion == "" {
		descripcion = "Punto de venta Supay"
	}

	// 2. Create local POS record (status CREATING)
	pos := &domain.PointOfSale{
		CompanyId:      company.ID,
		BranchId:       &branchID,
		CodigoSucursal: codigoSucursal,
		Description:    descripcion,
		IsActive:       true,
		Status:         "CREATING",
		CreatedAt:      time.Now(),
	}
	if input.LocalCode != nil && *input.LocalCode > 0 {
		pos.CodigoPuntoVenta = *input.LocalCode
	}
	if err := s.posRepo.Create(pos); err != nil {
		return nil, fmt.Errorf("create pos: %w", err)
	}

	fail := func(err error) (*domain.PointOfSale, error) {
		pos.Status = "ERROR"
		if pos.SiatError == nil {
			msg := err.Error()
			pos.SiatError = &msg
		}
		_ = s.posRepo.Update(pos)
		return pos, err
	}

	ambiente := company.Ambiente.CodigoAmbiente()
	modalidad := s.modalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	// 3. Paso A: obtain sucursal-level CUIS (transient; required by sincronizacion
	//    and registroPuntoVenta). It is NOT persisted: it belongs to the sucursal,
	//    not to this point of sale.
	cuisReq := siat.SolicitudCuis{
		CodigoAmbiente:   ambiente,
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   codigoSucursal,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: 0,
	}
	cuisResp, err := s.cuisSvc.SolicitarCUIS(ctx, cuisReq)
	if err != nil {
		return fail(fmt.Errorf("siat solicitar cuis sucursal: %w", err))
	}
	cuisSucursal := cuisResp.Codigo

	// 4. Paso A: sync official catalog of punto de venta types
	syncReq := siat.SolicitudSincronizacion{
		CodigoAmbiente: ambiente,
		CodigoSistema:  company.CodigoSistema,
		CodigoSucursal: codigoSucursal,
		Cuis:           cuisSucursal,
		Nit:            company.Nit,
	}
	syncResp, err := s.sincronizacionSvc.SincronizarTipoPuntoVenta(ctx, syncReq)
	if err != nil {
		return fail(fmt.Errorf("siat sincronizar tipo punto venta: %w", err))
	}
	if !syncResp.Transaccion {
		return fail(fmt.Errorf("siat sincronizar tipo punto venta rechazado: %s", describeMensajes(syncResp.Mensajes)))
	}
	now := time.Now()
	tipos := make([]domain.TipoPuntoVenta, 0, len(syncResp.ListaCodigos))
	for _, t := range syncResp.ListaCodigos {
		tipos = append(tipos, domain.TipoPuntoVenta{
			CodigoClasificador: t.CodigoClasificador,
			Descripcion:        t.Descripcion,
		})
	}
	if err := s.tipoPVRepo.Replace(company.ID, tipos, now); err != nil {
		return fail(fmt.Errorf("persist catalogo tipo punto venta: %w", err))
	}

	// 5. Paso A: validate/select codigoTipoPuntoVenta from official catalog
	tipo, err := s.resolveTipoPuntoVenta(company.ID, input.CodigoTipoPuntoVenta)
	if err != nil {
		return fail(err)
	}

	// 6. Paso B: register the point of sale with SIAT
	regReq := siat.RegistroPuntoVentaRequest{
		CodigoAmbiente:       ambiente,
		CodigoModalidad:      modalidad,
		Nit:                  company.Nit,
		CodigoSistema:        company.CodigoSistema,
		CodigoSucursal:       codigoSucursal,
		NombrePuntoVenta:     nombrePuntoVenta,
		CodigoTipoPuntoVenta: tipo,
		Cuis:                 cuisSucursal,
		Descripcion:          descripcion,
	}
	regResp, err := s.posSvc.Registrar(ctx, regReq)
	if err != nil {
		return fail(fmt.Errorf("siat registro punto venta: %w", err))
	}
	s.storeSiatExchange(pos, regResp.RawRequest, regResp.RawResponse, "")
	pos.SiatTransaccion = regResp.Transaccion

	// 7. Paso C: validate official response. Only SIAT success enables the POS.
	if !regResp.Transaccion || regResp.CodigoPuntoVenta <= 0 {
		msg := describeMensajes(regResp.Mensajes)
		if msg == "" {
			msg = "el SIAT no asignó un código de punto de venta"
		}
		s.storeSiatExchange(pos, regResp.RawRequest, regResp.RawResponse, msg)
		pos.Status = "ERROR"
		_ = s.posRepo.Update(pos)
		return pos, fmt.Errorf("siat registro punto venta rechazado: %s", msg)
	}

	// 8. Persist official code and enable the POS
	officialCode := regResp.CodigoPuntoVenta
	tipoPV := tipo
	registeredAt := time.Now()
	pos.SiatCode = &officialCode
	pos.TipoPuntoVenta = &tipoPV
	pos.SiatRegisteredAt = &registeredAt
	pos.SiatTransaccion = true
	pos.SiatError = nil
	pos.Status = "OPERATIVE"
	if err := s.posRepo.Update(pos); err != nil {
		return nil, fmt.Errorf("update pos siat code: %w", err)
	}

	// 9. Paso D: request a POS-scoped CUIS with the official code, persist it, and
	//    request the CUFD with that CUIS.
	posCuisReq := siat.SolicitudCuis{
		CodigoAmbiente:   ambiente,
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   codigoSucursal,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: officialCode,
	}
	posCuisResp, err := s.cuisSvc.SolicitarCUIS(ctx, posCuisReq)
	if err != nil {
		return pos, fmt.Errorf("punto de venta registrado (%d) pero falló la solicitud de CUIS: %w", officialCode, err)
	}
	cuis := &domain.Cuis{
		PointOfSaleID: pos.ID,
		Cuis:          posCuisResp.Codigo,
		ValidFrom:     now,
		ValidTo:       posCuisResp.FechaVigencia.Time,
		Active:        true,
		CreatedAt:     now,
	}
	if err := s.cuisRepo.Create(cuis); err != nil {
		return pos, fmt.Errorf("punto de venta registrado (%d) pero no se pudo persistir el CUIS: %w", officialCode, err)
	}
	pos.Cuis = &posCuisResp.Codigo
	pos.CuisCreatedAt = &now

	cufdReq := siat.SolicitudCufd{
		CodigoAmbiente:   ambiente,
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   codigoSucursal,
		Cuis:             posCuisResp.Codigo,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: officialCode,
	}
	cufdResp, err := s.cufdSvc.SolicitarCUFD(ctx, cufdReq)
	if err != nil {
		_ = s.posRepo.Update(pos)
		return pos, fmt.Errorf("punto de venta registrado (%d) pero falló la solicitud de CUFD: %w", officialCode, err)
	}
	cufd := &domain.Cufd{
		PointOfSaleID: pos.ID,
		Cufd:          cufdResp.Codigo,
		ControlCode:   cufdResp.CodigoControl,
		Direccion:     cufdResp.Direccion,
		ValidFrom:     now,
		ValidTo:       cufdResp.FechaVigencia.Time,
		Active:        true,
		CreatedAt:     now,
	}
	if err := s.cufdRepo.Create(cufd); err != nil {
		_ = s.posRepo.Update(pos)
		return pos, fmt.Errorf("punto de venta registrado (%d) pero no se pudo persistir el CUFD: %w", officialCode, err)
	}

	if err := s.posRepo.Update(pos); err != nil {
		return pos, err
	}
	return pos, nil
}

// SyncTipoPuntoVentaCatalog sincroniza a demanda el catálogo oficial de tipos de
// punto de venta para una empresa y lo persiste (operación
// sincronizarParametricaTipoPuntoVenta). Devuelve el catálogo vigente.
func (s *PointOfSaleProvisionService) SyncTipoPuntoVentaCatalog(ctx context.Context, companyID string, codigoSucursal int) ([]*domain.TipoPuntoVenta, error) {
	company, err := s.companyRepo.GetByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company lookup: %w", err)
	}
	if codigoSucursal < 0 {
		codigoSucursal = 0
	}
	ambiente := company.Ambiente.CodigoAmbiente()
	modalidad := s.modalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	// El CUIS de sucursal se solicita bajo demanda y no se persiste: el catálogo
	// es a nivel empresa y solo necesita el CUIS como dato de autenticación.
	cuisReq := siat.SolicitudCuis{
		CodigoAmbiente:   ambiente,
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   codigoSucursal,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: 0,
	}
	cuisResp, err := s.cuisSvc.SolicitarCUIS(ctx, cuisReq)
	if err != nil {
		return nil, fmt.Errorf("siat solicitar cuis: %w", err)
	}

	syncReq := siat.SolicitudSincronizacion{
		CodigoAmbiente: ambiente,
		CodigoSistema:  company.CodigoSistema,
		CodigoSucursal: codigoSucursal,
		Cuis:           cuisResp.Codigo,
		Nit:            company.Nit,
	}
	syncResp, err := s.sincronizacionSvc.SincronizarTipoPuntoVenta(ctx, syncReq)
	if err != nil {
		return nil, err
	}
	if !syncResp.Transaccion {
		return nil, fmt.Errorf("siat sincronizar tipo punto venta rechazado: %s", describeMensajes(syncResp.Mensajes))
	}

	tipos := make([]domain.TipoPuntoVenta, 0, len(syncResp.ListaCodigos))
	for _, t := range syncResp.ListaCodigos {
		tipos = append(tipos, domain.TipoPuntoVenta{
			CodigoClasificador: t.CodigoClasificador,
			Descripcion:        t.Descripcion,
		})
	}
	if err := s.tipoPVRepo.Replace(company.ID, tipos, time.Now()); err != nil {
		return nil, fmt.Errorf("persist catalogo tipo punto venta: %w", err)
	}
	return s.tipoPVRepo.List(company.ID)
}

// ListByBranch devuelve los puntos de venta de una sucursal.
func (s *PointOfSaleProvisionService) ListByBranch(branchID string) ([]*domain.PointOfSale, error) {
	if _, err := s.branchRepo.GetByID(branchID); err != nil {
		return nil, fmt.Errorf("branch not found: %w", err)
	}
	return s.posRepo.ListByBranch(branchID)
}

// PointOfSaleStatus es el estado tributario de un punto de venta.
type PointOfSaleStatus struct {
	BranchCode          int        `json:"branch_code"`
	SIATPointOfSaleCode *int       `json:"siat_point_of_sale_code"`
	LocalCode           int        `json:"local_code"`
	Cuis                string     `json:"cuis"`
	Cufd                string     `json:"cufd"`
	CufdExpiresAt       *time.Time `json:"cufd_expires_at"`
	Status              string     `json:"status"`
}

// GetOperationalStatus devuelve el estado tributario del punto de venta
// combinando el registro local, la sucursal y los CUIS/CUFD vigentes.
func (s *PointOfSaleProvisionService) GetOperationalStatus(posID string) (*PointOfSaleStatus, error) {
	pos, err := s.posRepo.GetByID(posID)
	if err != nil {
		return nil, fmt.Errorf("point of sale not found: %w", err)
	}

	branchCode := pos.CodigoSucursal
	if pos.BranchId != nil {
		if branch, err := s.branchRepo.GetByID(*pos.BranchId); err == nil {
			branchCode = branch.CodigoSucursal
		}
	}

	st := &PointOfSaleStatus{
		BranchCode:          branchCode,
		SIATPointOfSaleCode: pos.SiatCode,
		LocalCode:           pos.CodigoPuntoVenta,
		Status:              pos.Status,
	}

	if cuis, err := s.cuisRepo.GetActiveByPos(posID); err == nil && cuis != nil {
		st.Cuis = cuis.Cuis
	}
	if cufd, err := s.cufdRepo.GetActiveByPos(posID); err == nil && cufd != nil {
		st.Cufd = cufd.Cufd
		validTo := cufd.ValidTo
		st.CufdExpiresAt = &validTo
	}
	return st, nil
}

// resolveTipoPuntoVenta valida el codigoTipoPuntoVenta contra el catálogo oficial
// sincronizado; si no se indicó ninguno, usa el primer clasificador del catálogo.
func (s *PointOfSaleProvisionService) resolveTipoPuntoVenta(companyID string, requested int) (int, error) {
	if requested > 0 {
		if _, err := s.tipoPVRepo.FindByClasificador(companyID, requested); err != nil {
			return 0, fmt.Errorf("%w: %d", domain.ErrTipoPuntoVentaInvalido, requested)
		}
		return requested, nil
	}

	list, err := s.tipoPVRepo.List(companyID)
	if err != nil || len(list) == 0 {
		return 0, domain.ErrTipoPuntoVentaEmpty
	}
	return list[0].CodigoClasificador, nil
}

// storeSiatExchange persiste el intercambio SOAP (request/response crudos) y, si
// hay error, el mensaje y su timestamp para diagnóstico.
func (s *PointOfSaleProvisionService) storeSiatExchange(pos *domain.PointOfSale, requestXML, responseXML, errMsg string) {
	payload := map[string]any{
		"request":  requestXML,
		"response": responseXML,
	}
	if errMsg != "" {
		payload["error"] = errMsg
		payload["error_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	if b, err := json.Marshal(payload); err == nil {
		j := datatypes.JSON(b)
		pos.SiatResponse = &j
	}
	if errMsg != "" {
		pos.SiatError = &errMsg
	}
}

func describeMensajes(msgs []siat.Mensaje) string {
	if len(msgs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m.Codigo != "" {
			parts = append(parts, m.Codigo+": "+m.Descripcion)
		} else {
			parts = append(parts, m.Descripcion)
		}
	}
	return strings.Join(parts, "; ")
}
