package siat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type siatService interface {
	SolicitarCUIS(ctx context.Context, companyID, posID string) (*usecase.CuisResultado, error)
	SolicitarCUFD(ctx context.Context, companyID, posID string) (*usecase.CufdResultado, error)
	RegistrarEventoSignificativo(ctx context.Context, companyID, posID string, body usecase.EventoSignificativoInput) (*usecase.EventoSignificativoResultado, error)
	EnviarPaquete(ctx context.Context, companyID, posID string, body usecase.PaqueteInput) (*usecase.PaqueteResultado, error)
	ValidarPaquete(ctx context.Context, companyID, posID string, body usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error)
	EnviarMasiva(ctx context.Context, companyID, posID string, body usecase.MasivaInput) (*usecase.PaqueteResultado, error)
	ValidarMasiva(ctx context.Context, companyID, posID string, body usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error)
	EnviarCompras(ctx context.Context, companyID, posID string, body usecase.ComprasInput) (*usecase.ComprasResultado, error)
	FirmarFactura(ctx context.Context, companyID, posID string, body usecase.FirmaInput) (*usecase.FirmaResultado, error)
	Setup(ctx context.Context, companyID, posID string) (*usecase.SetupResultado, error)
	Sincronizar(ctx context.Context, companyID, posID, opRaw string) (*usecase.SincronizacionResultado, error)
	BuildSincronizacionResumen(companyID, posID string, res *usecase.SincronizacionResultado) *usecase.SincronizacionResumen
	ListSinProducts(companyID, query string, limit, offset int) ([]*domain.SinProduct, int64, error)
	ListActivitesDocumentSectors(companyID, query string, limit, offset int) ([]*domain.SiatActividadDocSector, int64, error)
	CatalogReadiness(companyID, pointOfSaleID string) (*domain.CatalogReadiness, error)
	ListCatalog(companyID, tipo string) (any, error)
	EmitirDocumentoAjuste(ctx context.Context, companyID, posID string, body usecase.DocumentoAjusteInput) (*usecase.DocumentoAjusteResultado, error)
}

type pdfGenerator interface {
	GenerateInvoicePDF(invoiceID string) ([]byte, error)
}

type handler struct {
	siatUC     siatService
	pdfService pdfGenerator
}

func newHandler(siatUC siatService, pdfService pdfGenerator) *handler {
	return &handler{siatUC: siatUC, pdfService: pdfService}
}

type siatCuisResponse struct {
	Success       bool   `json:"success"`
	Cuis          string `json:"cuis"`
	FechaVigencia string `json:"fecha_vigencia"`
}

func (h *handler) solicitarCUIS(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.SolicitarCUIS(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, siatCuisResponse{
		Success:       res.Success,
		Cuis:          res.Data.Codigo,
		FechaVigencia: res.Data.FechaVigencia.String(),
	})
}

func (h *handler) solicitarCUFD(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.SolicitarCUFD(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"success": res.Response.Transaccion,
		"data": map[string]any{
			"cufd":           res.Response.Codigo,
			"fecha_vigencia": res.Response.FechaVigencia.Format("2006-01-02 15:04:05"),
			"codigo_control": res.Response.CodigoControl,
		},
	})
}

func (h *handler) registrarEventoSignificativo(w http.ResponseWriter, r *http.Request) {
	var body usecase.EventoSignificativoInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			deliveryHttp.RespondValidation(w, "payload JSON inválido")
			return
		}
	}
	res, err := h.siatUC.RegistrarEventoSignificativo(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"success":          true,
		"codigo_recepcion": res.Response.CodigoRecepcion,
	})
}

func (h *handler) enviarPaquete(w http.ResponseWriter, r *http.Request) {
	var body usecase.PaqueteInput
	if !h.decodeBody(w, r, &body) {
		return
	}

	res, err := h.siatUC.EnviarPaquete(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *handler) validarPaquete(w http.ResponseWriter, r *http.Request) {
	var body usecase.PaqueteValidacionInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.ValidarPaquete(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *handler) enviarMasiva(w http.ResponseWriter, r *http.Request) {
	var body usecase.MasivaInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EnviarMasiva(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *handler) validarMasiva(w http.ResponseWriter, r *http.Request) {
	var body usecase.PaqueteValidacionInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.ValidarMasiva(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *handler) enviarCompras(w http.ResponseWriter, r *http.Request) {
	var body usecase.ComprasInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EnviarCompras(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

func (h *handler) firmarFactura(w http.ResponseWriter, r *http.Request) {
	var body usecase.FirmaInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.FirmarFactura(r.Context(), chi.URLParam(r, "companyId"), chi.URLParam(r, "pointOfSaleId"), body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

type setupRequest struct {
	CompanyID     string `json:"company_id"`
	PointOfSaleID string `json:"point_of_sale_id"`
}

// setup orquesta el alta de un punto de venta en una llamada: CUIS (lazy),
// sincronización de catálogos, CUFD (lazy) y readiness. Idempotente.
func (h *handler) setup(w http.ResponseWriter, r *http.Request) {
	var body setupRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			deliveryHttp.RespondValidation(w, "payload JSON inválido")
			return
		}
	}
	if body.CompanyID == "" || body.PointOfSaleID == "" {
		deliveryHttp.RespondValidation(w, "company_id y point_of_sale_id son obligatorios")
		return
	}
	res, err := h.siatUC.Setup(r.Context(), body.CompanyID, body.PointOfSaleID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) sincronizar(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "companyId")
	posID := chi.URLParam(r, "pointOfSaleId")
	res, err := h.siatUC.Sincronizar(r.Context(), companyID, posID, r.URL.Query().Get("operation"))
	if err != nil && res == nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	if err != nil {
		slog.Error("error en sincronización de catálogos", "error", err)
	}
	resumen := h.siatUC.BuildSincronizacionResumen(companyID, posID, res)
	status := http.StatusOK
	if !resumen.Success && err != nil {
		status = http.StatusForbidden
	}
	deliveryHttp.WriteJSON(w, status, resumen)
}

func (h *handler) listSinProducts(w http.ResponseWriter, r *http.Request) {
	limit := deliveryHttp.ParseQueryInt(r.URL.Query().Get("limit"), 50)
	offset := deliveryHttp.ParseQueryInt(r.URL.Query().Get("offset"), 0)
	items, total, err := h.siatUC.ListSinProducts(r.URL.Query().Get("company_id"), r.URL.Query().Get("query"), limit, offset)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset, "total": total})
}

func (h *handler) listActivitesDocumentSectors(w http.ResponseWriter, r *http.Request) {
	limit := deliveryHttp.ParseQueryInt(r.URL.Query().Get("limit"), 50)
	offset := deliveryHttp.ParseQueryInt(r.URL.Query().Get("offset"), 0)
	items, total, err := h.siatUC.ListActivitesDocumentSectors(r.URL.Query().Get("company_id"), r.URL.Query().Get("query"), limit, offset)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset, "total": total})
}

func (h *handler) catalogReadiness(w http.ResponseWriter, r *http.Request) {
	readiness, err := h.siatUC.CatalogReadiness(r.URL.Query().Get("company_id"), r.URL.Query().Get("point_of_sale_id"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, readiness)
}

// getCatalog sirve los catálogos sincronizados desde la BD. Con
// GET /catalogs/{companyId}/{tipo} devuelve ese catálogo; con
// GET /catalogs/{companyId} o tipo=all los devuelve todos agrupados.
func (h *handler) getCatalog(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListCatalog(chi.URLParam(r, "companyId"), chi.URLParam(r, "tipo"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) emitirDocumentoAjuste(w http.ResponseWriter, r *http.Request) {
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
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
		"company":       res.Company,
		"point_of_sale": res.PointOfSale,
		"response":      res.Response,
	})
}

// decodeBody decodifica el cuerpo JSON en out; un cuerpo vacío es válido
// (todos los inputs tienen defaults). Devuelve false si ya respondió.
func (h *handler) decodeBody(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.Body == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return false
	}
	return true
}

// downloadPDF genera y descarga el PDF de la factura indicada.
func (h *handler) downloadPDF(w http.ResponseWriter, r *http.Request) {
	if h.pdfService == nil {
		deliveryHttp.WriteErrorBody(w, http.StatusServiceUnavailable, deliveryHttp.CodeInternal, "servicio de PDF no inicializado")
		return
	}
	invoiceID := chi.URLParam(r, "id")
	if invoiceID == "" {
		invoiceID = chi.URLParam(r, "invoiceId")
	}
	if invoiceID == "" {
		deliveryHttp.RespondValidation(w, "id es obligatorio")
		return
	}

	data, err := h.pdfService.GenerateInvoicePDF(invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "no encontrada") || errors.Is(err, gorm.ErrRecordNotFound) {
			deliveryHttp.RespondNotFound(w, "factura no encontrada")
			return
		}
		slog.Error("no se pudo generar el PDF", "invoice_id", invoiceID, "error", err)
		deliveryHttp.WriteErrorBody(w, http.StatusInternalServerError, deliveryHttp.CodeInternal, "error interno al generar el PDF")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"factura-%s.pdf\"", invoiceID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
