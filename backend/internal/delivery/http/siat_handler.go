package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
	goSiat "github.com/ron86i/go-siat/v2"
	"gorm.io/gorm"
)

// SiatHandler es el adaptador HTTP del SiatUsecase: decodifica requests,
// delega la lógica de negocio y codifica las respuestas.
type SiatHandler struct {
	siatUC     *usecase.SiatUsecase
	pdfService *pdf.Service
}

func NewSiatHandler(siatUC *usecase.SiatUsecase, pdfService *pdf.Service) *SiatHandler {
	return &SiatHandler{siatUC: siatUC, pdfService: pdfService}
}

type siatCuisResponse struct {
	Company     *domain.Company     `json:"company"`
	PointOfSale *domain.PointOfSale `json:"point_of_sale"`
	Response    *siat.RespuestaCuis `json:"response"`
}

func (h *SiatHandler) SolicitarCUIS(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.SolicitarCUIS(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, siatCuisResponse{
		Company:     res.Company,
		PointOfSale: res.PointOfSale,
		Response:    res.Response,
	})
}

func (h *SiatHandler) SolicitarCUFD(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.SolicitarCUFD(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) RegistrarEventoSignificativo(w http.ResponseWriter, r *http.Request) {
	var body usecase.EventoSignificativoInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}
	res, err := h.siatUC.RegistrarEventoSignificativo(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) EnviarPaquete(w http.ResponseWriter, r *http.Request) {
	var body usecase.PaqueteInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EnviarPaquete(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) ValidarPaquete(w http.ResponseWriter, r *http.Request) {
	var body usecase.PaqueteValidacionInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.ValidarPaquete(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) EnviarMasiva(w http.ResponseWriter, r *http.Request) {
	var body usecase.MasivaInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EnviarMasiva(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) ValidarMasiva(w http.ResponseWriter, r *http.Request) {
	var body usecase.PaqueteValidacionInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.ValidarMasiva(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) EnviarCompras(w http.ResponseWriter, r *http.Request) {
	var body usecase.ComprasInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EnviarCompras(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) FirmarFactura(w http.ResponseWriter, r *http.Request) {
	var body usecase.FirmaInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.FirmarFactura(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *SiatHandler) Sincronizar(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.Sincronizar(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), r.URL.Query().Get("operation"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"operations":    res.Operations,
		"errors":        res.Errors,
	})
}

func (h *SiatHandler) ListSinProducts(w http.ResponseWriter, r *http.Request) {
	limit := parseQueryInt(r.URL.Query().Get("limit"), 50)
	offset := parseQueryInt(r.URL.Query().Get("offset"), 0)
	items, total, err := h.siatUC.ListSinProducts(r.URL.Query().Get("companyId"), r.URL.Query().Get("query"), limit, offset)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset, "total": total})
}

func (h *SiatHandler) CatalogReadiness(w http.ResponseWriter, r *http.Request) {
	readiness, err := h.siatUC.CatalogReadiness(r.URL.Query().Get("companyId"), r.URL.Query().Get("pointOfSaleId"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, readiness)
}

// GetCatalog sirve los catálogos sincronizados desde la BD. Con
// GET /catalogs/{companyId}/{tipo} devuelve ese catálogo; con
// GET /catalogs/{companyId} o tipo=all los devuelve todos agrupados.
func (h *SiatHandler) GetCatalog(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListCatalog(chi.URLParam(r, "companyId"), chi.URLParam(r, "tipo"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *SiatHandler) EmitirDocumentoAjuste(w http.ResponseWriter, r *http.Request) {
	var body usecase.DocumentoAjusteInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EmitirDocumentoAjuste(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

// decodeBody decodifica el cuerpo JSON en out; un cuerpo vacío es válido
// (todos los inputs tienen defaults). Devuelve false si ya respondió.
func (h *SiatHandler) decodeBody(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.Body == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseQueryInt(raw string, fallback int) int {
	if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
		return value
	}
	return fallback
}

// DownloadPDF genera y descarga el PDF de la factura indicada.
func (h *SiatHandler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	if h.pdfService == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Servicio de PDF no inicializado")
		return
	}
	invoiceID := chi.URLParam(r, "invoiceId")
	if invoiceID == "" {
		writeJSONError(w, http.StatusBadRequest, "invoiceId es obligatorio")
		return
	}

	data, err := h.pdfService.GenerateInvoicePDF(invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "no encontrada") || errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONError(w, http.StatusNotFound, "factura no encontrada")
			return
		}
		slog.Error("no se pudo generar el PDF", "invoice_id", invoiceID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "error interno al generar el PDF")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"factura-%s.pdf\"", invoiceID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func errToStatus(err error) int {
	var br *domain.BadRequestError
	var nf *domain.NotFoundError
	var cf *domain.ConflictError
	var siatErr *goSiat.SiatError
	switch {
	case errors.Is(err, usecase.ErrSiatNoDisponible):
		return http.StatusServiceUnavailable
	case errors.As(err, &br):
		return http.StatusBadRequest
	case errors.As(err, &nf):
		return http.StatusNotFound
	case errors.As(err, &cf):
		return http.StatusConflict
	case errors.As(err, &siatErr):
		if goSiat.IsNetworkError(err) || goSiat.IsRetryable(err) {
			return http.StatusServiceUnavailable
		}
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
