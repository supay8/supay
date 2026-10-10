package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

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

func toSincronizacionOpResult(op ports.FiscalSyncOperation, res *ports.FiscalSyncResult) SincronizacionOpResult {
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

func syncRows(op ports.FiscalSyncOperation, res *ports.FiscalSyncResult) int {
	switch op {
	case ports.OpActividades:
		if len(res.Actividades) > 0 {
			return len(res.Actividades)
		}
	case ports.OpLeyendasFactura:
		if len(res.Leyendas) > 0 {
			return len(res.Leyendas)
		}
	case ports.OpActividadesDocumentoSector:
		if len(res.ActividadesDocSector) > 0 {
			return len(res.ActividadesDocSector)
		}
	case ports.OpProductosServicios:
		if len(res.Productos) > 0 {
			return len(res.Productos)
		}
		return len(res.Codigos)
	}
	return len(res.Codigos)
}

func syncStatus(op ports.FiscalSyncOperation, res *ports.FiscalSyncResult) string {
	if !res.Transaccion {
		return "FAILED"
	}
	if isCriticalSyncOperation(op) && syncRows(op, res) == 0 {
		return "EMPTY"
	}
	return "SUCCESS"
}

func isCriticalSyncOperation(op ports.FiscalSyncOperation) bool {
	switch op {
	case ports.OpActividades, ports.OpProductosServicios, ports.OpActividadesDocumentoSector, ports.OpUnidadMedida, ports.OpTipoMoneda, ports.OpTipoMetodoPago, ports.OpLeyendasFactura:
		return true
	default:
		return false
	}
}

// PersistSincronizacion guarda el resultado de una operación. Los productos
// SIN se guardan en sin_products; los catálogos paramétricos siguen en
// catalogs y los tipos de punto de venta tienen su repositorio dedicado.
func (uc *SiatUsecase) PersistSincronizacion(companyID string, op ports.FiscalSyncOperation, res *ports.FiscalSyncResult) error {
	return uc.PersistSincronizacionAt(companyID, "", op, res)
}

func (uc *SiatUsecase) PersistSincronizacionAt(companyID, pointOfSaleID string, op ports.FiscalSyncOperation, res *ports.FiscalSyncResult) error {
	if res == nil || !res.Transaccion {
		_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: "FAILED", Error: "SIAT no confirmó la sincronización"})
		return nil
	}
	now := time.Now().In(fiscal.LaPaz)
	status := syncStatus(op, res)
	rowsSaved := syncRows(op, res)
	if op == ports.OpProductosServicios {
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

	if op == ports.OpTipoPuntoVenta {
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
	case ports.OpFechaHora, ports.OpVerificarComunicacion:
		return uc.saveSyncState(domain.CatalogSyncState{CompanyID: companyID, PointOfSaleID: pointOfSaleID, Operation: string(op), Status: status, RowsSaved: rowsSaved, SyncedAt: &now})
	case ports.OpActividades:
		return uc.persistActividades(companyID, pointOfSaleID, op, res, status, rowsSaved, now)
	case ports.OpLeyendasFactura:
		return uc.persistLeyendas(companyID, pointOfSaleID, op, res, status, rowsSaved, now)
	case ports.OpActividadesDocumentoSector:
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
func (uc *SiatUsecase) persistActividades(companyID, posID string, op ports.FiscalSyncOperation, res *ports.FiscalSyncResult, status string, rowsSaved int, now time.Time) error {
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
func (uc *SiatUsecase) persistLeyendas(companyID, posID string, op ports.FiscalSyncOperation, res *ports.FiscalSyncResult, status string, rowsSaved int, now time.Time) error {
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
func (uc *SiatUsecase) persistActividadesDocSector(companyID, posID string, op ports.FiscalSyncOperation, res *ports.FiscalSyncResult, status string, rowsSaved int, now time.Time) error {
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
	required := []ports.FiscalSyncOperation{ports.OpActividades, ports.OpProductosServicios, ports.OpActividadesDocumentoSector, ports.OpUnidadMedida, ports.OpTipoMoneda, ports.OpTipoMetodoPago, ports.OpLeyendasFactura}
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

	svc, err := uc.resolveService(ctx, company.ID)
	if err != nil {
		return nil, err
	}
	// CodigoSistema viene del config global (infra), no del company.
	// Si queda vacío la sincronización falla Validate con "codigoSistema es obligatorio".
	codigoSistema := ""
	if fa, ok := svc.(ports.FiscalIdentity); ok {
		codigoSistema = fa.CodigoSistema()
	}
	// El código de punto de venta que usa el CUIS debe ser el mismo en todas
	// las operaciones (preferir el código registrado ante SIAT).
	req := ports.FiscalSyncRequest{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    codigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoPuntoVenta: pointOfSale.CodigoPuntoVenta,
		Cuis:             *pointOfSale.Cuis,
	}
	out := &SincronizacionResultado{Company: company, PointOfSale: pointOfSale}

	if opRaw != "" {
		op, ok := ports.ParseFiscalSyncOperation(opRaw)
		if !ok {
			return nil, domain.NewBadRequestError("Operación de sincronización desconocida: " + opRaw)
		}
		result, err := svc.Synchronize(ctx, req, op)
		if err != nil {
			_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: company.ID, PointOfSaleID: pointOfSale.ID, Operation: string(op), Status: "FAILED", Error: err.Error()})
			return nil, err
		}
		if perr := uc.PersistSincronizacionAt(company.ID, pointOfSale.ID, op, &result); perr != nil {
			return nil, perr
		}
		out.Operations = append(out.Operations, toSincronizacionOpResult(op, &result))
		return out, nil
	}
	for _, op := range ports.FiscalSyncOperations {
		result, err := svc.Synchronize(ctx, req, op)
		if err != nil {
			_ = uc.saveSyncState(domain.CatalogSyncState{CompanyID: company.ID, PointOfSaleID: pointOfSale.ID, Operation: string(op), Status: "FAILED", Error: err.Error()})
			out.Errors = append(out.Errors, SincronizacionOpError{Operation: string(op), Error: err.Error()})
			continue
		}
		if perr := uc.PersistSincronizacionAt(company.ID, pointOfSale.ID, op, &result); perr != nil {
			out.Errors = append(out.Errors, SincronizacionOpError{Operation: string(op), Error: "persistencia: " + perr.Error()})
		}
		out.Operations = append(out.Operations, toSincronizacionOpResult(op, &result))
	}

	return out, nil
}
