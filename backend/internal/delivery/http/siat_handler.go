package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/go-chi/chi/v5"
)

type SiatHandler struct {
	companyRepo     domain.CompanyRepository
	pointOfSaleRepo domain.PointOfSaleRepository
	cufdRepo        domain.CufdRepository
	tipoPVRepo      domain.TipoPuntoVentaRepository
	catalogRepo     domain.CatalogRepository
	siatService     *siat.Service
	pdfService      *pdf.Service
	modalidad       int
}

func NewSiatHandler(
	companyRepo domain.CompanyRepository,
	pointOfSaleRepo domain.PointOfSaleRepository,
	cufdRepo domain.CufdRepository,
	tipoPVRepo domain.TipoPuntoVentaRepository,
	catalogRepo domain.CatalogRepository,
	siatService *siat.Service,
	pdfService *pdf.Service,
	modalidad int,
) *SiatHandler {
	return &SiatHandler{
		companyRepo:     companyRepo,
		pointOfSaleRepo: pointOfSaleRepo,
		cufdRepo:        cufdRepo,
		tipoPVRepo:      tipoPVRepo,
		catalogRepo:     catalogRepo,
		siatService:     siatService,
		pdfService:      pdfService,
		modalidad:       modalidad,
	}
}

type siatCuisResponse struct {
	Company     *domain.Company     `json:"company"`
	PointOfSale *domain.PointOfSale `json:"point_of_sale"`
	Response    *siat.RespuestaCuis `json:"response"`
}

type siatCufdResponse struct {
	Company     *domain.Company     `json:"company"`
	PointOfSale *domain.PointOfSale `json:"point_of_sale"`
	Response    *siat.RespuestaCufd `json:"response"`
}

func (h *SiatHandler) SolicitarCUIS(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}
	modalidad := h.modalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	req := siat.SolicitudCuis{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: codigoPuntoVenta,
	}
	if pointOfSale.Cuis != nil && *pointOfSale.Cuis != "" {
		req.Cuis = pointOfSale.Cuis
	}

	resp, err := h.siatService.SolicitarCUIS(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	now := time.Now().UTC()
	updatedCuis := resp.Codigo
	pointOfSale.Cuis = &updatedCuis
	pointOfSale.CuisCreatedAt = &now
	if err := h.pointOfSaleRepo.Update(pointOfSale); err != nil {
		http.Error(w, "No se pudo persistir el CUIS en el punto de venta", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatCuisResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    resp,
	})
}

func (h *SiatHandler) SolicitarCUFD(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		http.Error(w, "El punto de venta no tiene CUIS activo", http.StatusConflict)
		return
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}
	modalidad := h.modalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	req := siat.SolicitudCufd{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		Cuis:             *pointOfSale.Cuis,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: codigoPuntoVenta,
	}

	resp, err := h.siatService.SolicitarCUFD(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	now := time.Now().UTC()
	cufd := &domain.Cufd{
		PointOfSaleID: pointOfSale.ID,
		Cufd:          resp.Codigo,
		ControlCode:   resp.CodigoControl,
		Direccion:     resp.Direccion,
		ValidFrom:     now,
		ValidTo:       resp.FechaVigencia.Time,
		Active:        true,
	}
	if err := h.cufdRepo.Create(cufd); err != nil {
		http.Error(w, "No se pudo persistir el CUFD", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatCufdResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    resp,
	})
}

type siatSincronizacionOpResult struct {
	Operation   string `json:"operation"`
	Transaccion bool   `json:"transaccion"`
	Codigos     int    `json:"codigos"`
	FechaHora   string `json:"fechaHora,omitempty"`
}

type siatSincronizacionOpError struct {
	Operation string `json:"operation"`
	Error     string `json:"error"`
}

type siatSincronizacionResponse struct {
	Company     *domain.Company               `json:"company"`
	PointOfSale *domain.PointOfSale           `json:"point_of_sale"`
	Operations  []siatSincronizacionOpResult  `json:"operations,omitempty"`
	Errors      []siatSincronizacionOpError   `json:"errors,omitempty"`
}

// Sincronizar baja catálogos del SIAT para la empresa y punto de venta indicados.
// Sin parámetro ?operation= sincroniza todas las operaciones del SDK.
func (h *SiatHandler) Sincronizar(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		http.Error(w, "El punto de venta no tiene CUIS activo", http.StatusConflict)
		return
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}

	req := siat.SolicitudSincronizacion{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoPuntoVenta: codigoPuntoVenta,
		Cuis:             *pointOfSale.Cuis,
	}

	opRaw := r.URL.Query().Get("operation")
	if opRaw != "" {
		op, ok := siat.ParseSincronizacionOp(opRaw)
		if !ok {
			http.Error(w, "Operación de sincronización desconocida: "+opRaw, http.StatusBadRequest)
			return
		}
		result, err := h.siatService.Sincronizar(r.Context(), req, op)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if perr := h.persistSincronizacion(company.ID, op, result); perr != nil {
			http.Error(w, "No se pudo persistir el catálogo: "+perr.Error(), http.StatusInternalServerError)
			return
		}
		h.respondSincronizacion(w, company, pointOfSale, []siatSincronizacionOpResult{toSincronizacionOpResult(op, result)}, nil)
		return
	}

	var results []siatSincronizacionOpResult
	var errors []siatSincronizacionOpError
	for _, op := range siat.SincronizacionOperations {
		result, err := h.siatService.Sincronizar(r.Context(), req, op)
		if err != nil {
			errors = append(errors, siatSincronizacionOpError{Operation: string(op), Error: err.Error()})
			continue
		}
		if perr := h.persistSincronizacion(company.ID, op, result); perr != nil {
			errors = append(errors, siatSincronizacionOpError{Operation: string(op), Error: "persistencia: " + perr.Error()})
		}
		results = append(results, toSincronizacionOpResult(op, result))
	}

	h.respondSincronizacion(w, company, pointOfSale, results, errors)
}

func toSincronizacionOpResult(op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) siatSincronizacionOpResult {
	out := siatSincronizacionOpResult{
		Operation:   string(op),
		Transaccion: res.Transaccion,
		Codigos:     len(res.Codigos),
	}
	if !res.FechaHora.IsZero() {
		out.FechaHora = res.FechaHora.Format(time.RFC3339Nano)
	}
	return out
}

func (h *SiatHandler) respondSincronizacion(w http.ResponseWriter, company *domain.Company, pointOfSale *domain.PointOfSale, results []siatSincronizacionOpResult, errors []siatSincronizacionOpError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatSincronizacionResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Operations:  results,
		Errors:      errors,
	})
}

// persistSincronizacion guarda el catálogo sincronizado: tipos de punto de venta
// en tipo_punto_ventas y el resto en catalogs. FechaHora y VerificarComunicacion
// no son catálogos y no se persisten.
func (h *SiatHandler) persistSincronizacion(companyID string, op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) error {
	if res == nil || !res.Transaccion {
		return nil
	}
	now := time.Now().UTC()

	if op == siat.OpTipoPuntoVenta {
		tipos := make([]domain.TipoPuntoVenta, 0, len(res.Codigos))
		for _, c := range res.Codigos {
			tipos = append(tipos, domain.TipoPuntoVenta{
				CodigoClasificador: c.CodigoClasificador,
				Descripcion:        c.Descripcion,
			})
		}
		return h.tipoPVRepo.Replace(companyID, tipos, now)
	}

	switch op {
	case siat.OpFechaHora, siat.OpVerificarComunicacion:
		return nil
	}

	items := make([]domain.CatalogItem, 0, len(res.Codigos))
	for _, c := range res.Codigos {
		items = append(items, domain.CatalogItem{
			Codigo:      c.CodigoClasificador,
			Descripcion: c.Descripcion,
			Tipo:        string(op),
		})
	}
	return h.catalogRepo.Replace(companyID, string(op), items, now)
}

// DownloadPDF genera y descarga el PDF de la factura indicada.
func (h *SiatHandler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	if h.pdfService == nil {
		http.Error(w, "Servicio de PDF no inicializado", http.StatusServiceUnavailable)
		return
	}
	invoiceID := chi.URLParam(r, "invoiceId")
	if invoiceID == "" {
		http.Error(w, "invoiceId es obligatorio", http.StatusBadRequest)
		return
	}

	data, err := h.pdfService.GenerateInvoicePDF(invoiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"factura-%s.pdf\"", invoiceID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *SiatHandler) loadCompanyAndPointOfSale(r *http.Request) (*domain.Company, *domain.PointOfSale, error) {
	companyID := chi.URLParam(r, "companyId")
	pointOfSaleID := chi.URLParam(r, "pointOfSaleId")
	if companyID == "" || pointOfSaleID == "" {
		return nil, nil, &badRequestError{message: "companyId y pointOfSaleId son obligatorios"}
	}

	company, err := h.companyRepo.GetByID(companyID)
	if err != nil {
		return nil, nil, &notFoundError{message: "Empresa no encontrada"}
	}

	pointOfSale, err := h.pointOfSaleRepo.GetByID(pointOfSaleID)
	if err != nil {
		return nil, nil, &notFoundError{message: "Punto de venta no encontrado"}
	}

	if pointOfSale.CompanyId != company.ID {
		return nil, nil, &badRequestError{message: "El punto de venta no pertenece a la empresa indicada"}
	}

	return company, pointOfSale, nil
}

type badRequestError struct {
	message string
}

func (e *badRequestError) Error() string { return e.message }

type notFoundError struct {
	message string
}

func (e *notFoundError) Error() string { return e.message }

func errToStatus(err error) int {
	switch err.(type) {
	case *badRequestError:
		return http.StatusBadRequest
	case *notFoundError:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
