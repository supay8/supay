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
	cuisService     *siat.CuisService
	cufdService     *siat.CufdService
	emissionService *siat.EmissionService
	pdfService      *pdf.Service
	modalidad       int
}

func NewSiatHandler(
	companyRepo domain.CompanyRepository,
	pointOfSaleRepo domain.PointOfSaleRepository,
	cufdRepo domain.CufdRepository,
	cuisService *siat.CuisService,
	cufdService *siat.CufdService,
	emissionService *siat.EmissionService,
	pdfService *pdf.Service,
	modalidad int,
) *SiatHandler {
	return &SiatHandler{
		companyRepo:     companyRepo,
		pointOfSaleRepo: pointOfSaleRepo,
		cufdRepo:        cufdRepo,
		cuisService:     cuisService,
		cufdService:     cufdService,
		emissionService: emissionService,
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
	if h.cuisService == nil {
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

	resp, err := h.cuisService.SolicitarCUIS(r.Context(), req)
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
	if h.cufdService == nil {
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

	resp, err := h.cufdService.SolicitarCUFD(r.Context(), req)
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
		CodigoQR:      resp.CodigoQR,
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

func (h *SiatHandler) EmitInvoice(w http.ResponseWriter, r *http.Request) {
	if h.emissionService == nil {
		http.Error(w, "Servicio de emisión no inicializado", http.StatusServiceUnavailable)
		return
	}

	invoiceID := chi.URLParam(r, "invoiceId")
	if invoiceID == "" {
		http.Error(w, "invoiceId es obligatorio", http.StatusBadRequest)
		return
	}

	inv, err := h.emissionService.Emit(r.Context(), invoiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"invoice_id": inv.ID, "status": inv.Status, "xml_hash": inv.XmlHash})
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
