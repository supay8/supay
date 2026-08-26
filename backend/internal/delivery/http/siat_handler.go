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
			respondValidation(w, "payload JSON inválido")
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

type setupRequest struct {
	CompanyID     string `json:"company_id"`
	PointOfSaleID string `json:"point_of_sale_id"`
}

// Setup orquesta el alta de un punto de venta en una llamada: CUIS (lazy),
// sincronización de catálogos, CUFD (lazy) y readiness. Idempotente.
func (h *SiatHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var body setupRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			respondValidation(w, "payload JSON inválido")
			return
		}
	}
	if body.CompanyID == "" || body.PointOfSaleID == "" {
		respondValidation(w, "company_id y point_of_sale_id son obligatorios")
		return
	}
	res, err := h.siatUC.Setup(r.Context(), body.CompanyID, body.PointOfSaleID)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
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
	items, total, err := h.siatUC.ListSinProducts(r.URL.Query().Get("company_id"), r.URL.Query().Get("query"), limit, offset)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset, "total": total})
}

func (h *SiatHandler) CatalogReadiness(w http.ResponseWriter, r *http.Request) {
	readiness, err := h.siatUC.CatalogReadiness(r.URL.Query().Get("company_id"), r.URL.Query().Get("point_of_sale_id"))
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
	slog.Warn("uso de endpoint deprecado documento-ajuste",
		"path", r.URL.Path,
		"company_id", chi.URLParam(r, "companyId"),
		"point_of_sale_id", chi.URLParam(r, "pointOfSaleId"),
	)
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
		respondValidation(w, "payload JSON inválido")
		return false
	}
	return true
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
		writeErrorBody(w, http.StatusServiceUnavailable, errorBody{Code: codeInternal, Message: "servicio de PDF no inicializado"})
		return
	}
	invoiceID := chi.URLParam(r, "id")
	if invoiceID == "" {
		invoiceID = chi.URLParam(r, "invoiceId")
	}
	if invoiceID == "" {
		respondValidation(w, "id es obligatorio")
		return
	}

	data, err := h.pdfService.GenerateInvoicePDF(invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "no encontrada") || errors.Is(err, gorm.ErrRecordNotFound) {
			respondNotFound(w, "factura no encontrada")
			return
		}
		slog.Error("no se pudo generar el PDF", "invoice_id", invoiceID, "error", err)
		writeErrorBody(w, http.StatusInternalServerError, errorBody{Code: codeInternal, Message: "error interno al generar el PDF"})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"factura-%s.pdf\"", invoiceID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
