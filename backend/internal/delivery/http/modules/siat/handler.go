package siat

import (
	"bytes"
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
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	if !h.decodeBody(w, r, &struct{}{}) {
		return
	}
	res, err := h.siatUC.SolicitarCUIS(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"))
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
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	if !h.decodeBody(w, r, &struct{}{}) {
		return
	}
	res, err := h.siatUC.SolicitarCUFD(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"))
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
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body usecase.EventoSignificativoInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.RegistrarEventoSignificativo(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"), body)
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
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body batchSubmissionRequest
	if !h.decodeBody(w, r, &body) {
		return
	}

	res, err := h.siatUC.EnviarPaquete(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"), usecase.PaqueteInput{FacturaIDs: body.InvoiceIDs})
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, newBatchResponse(res))
}

func (h *handler) validarPaquete(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body usecase.PaqueteValidacionInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	body.BatchID = chi.URLParam(r, "batchId")
	if body.BatchID == "" {
		deliveryHttp.RespondValidation(w, "batchId es obligatorio")
		return
	}
	res, err := h.siatUC.ValidarPaquete(r.Context(), companyID, "", body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, newBatchResponse(res))
}

func (h *handler) enviarMasiva(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body batchSubmissionRequest
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EnviarMasiva(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"), usecase.MasivaInput{FacturaIDs: body.InvoiceIDs})
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, newBatchResponse(res))
}

func (h *handler) validarMasiva(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body usecase.PaqueteValidacionInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	body.BatchID = chi.URLParam(r, "batchId")
	if body.BatchID == "" {
		deliveryHttp.RespondValidation(w, "batchId es obligatorio")
		return
	}
	res, err := h.siatUC.ValidarMasiva(r.Context(), companyID, "", body)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, newBatchResponse(res))
}

func (h *handler) enviarCompras(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body usecase.ComprasInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EnviarCompras(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"), body)
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
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body usecase.FirmaInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.FirmarFactura(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"), body)
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
	PointOfSaleID string `json:"point_of_sale_id"`
}

// setup orquesta el alta de un punto de venta en una llamada: CUIS (lazy),
// sincronización de catálogos, CUFD (lazy) y readiness. Idempotente.
func (h *handler) setup(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	var body setupRequest
	if !h.decodeBody(w, r, &body) {
		return
	}
	posID := chi.URLParam(r, "pointOfSaleId")
	if posID != "" && body.PointOfSaleID != "" && posID != body.PointOfSaleID {
		deliveryHttp.RespondValidation(w, "point_of_sale_id no coincide con la ruta")
		return
	}
	if posID == "" {
		posID = body.PointOfSaleID
	}
	if posID == "" {
		deliveryHttp.RespondValidation(w, "point_of_sale_id es obligatorio")
		return
	}
	res, err := h.siatUC.Setup(r.Context(), companyID, posID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) sincronizar(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	posID := chi.URLParam(r, "pointOfSaleId")
	if !h.decodeBody(w, r, &struct{}{}) {
		return
	}
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
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	limit := deliveryHttp.ParseQueryInt(r.URL.Query().Get("limit"), 50)
	offset := deliveryHttp.ParseQueryInt(r.URL.Query().Get("offset"), 0)
	items, total, err := h.siatUC.ListSinProducts(companyID, r.URL.Query().Get("query"), limit, offset)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset, "total": total})
}

func (h *handler) listActivitesDocumentSectors(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	limit := deliveryHttp.ParseQueryInt(r.URL.Query().Get("limit"), 50)
	offset := deliveryHttp.ParseQueryInt(r.URL.Query().Get("offset"), 0)
	items, total, err := h.siatUC.ListActivitesDocumentSectors(companyID, r.URL.Query().Get("query"), limit, offset)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset, "total": total})
}

func (h *handler) catalogReadiness(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	readiness, err := h.siatUC.CatalogReadiness(companyID, r.URL.Query().Get("point_of_sale_id"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, readiness)
}

// getCatalog sirve los catálogos sincronizados desde la BD. Con
// GET /siat/catalogs/{tipo} devuelve ese catálogo del tenant autenticado;
// GET /siat/catalogs o tipo=all los devuelve todos agrupados.
func (h *handler) getCatalog(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	res, err := h.siatUC.ListCatalog(companyID, chi.URLParam(r, "tipo"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) emitirDocumentoAjuste(w http.ResponseWriter, r *http.Request) {
	companyID, ok := authenticatedCompany(w, r)
	if !ok {
		return
	}
	slog.Warn("uso de endpoint deprecado documento-ajuste",
		"path", r.URL.Path,
		"company_id", companyID,
		"point_of_sale_id", chi.URLParam(r, "pointOfSaleId"),
	)
	var body usecase.DocumentoAjusteInput
	if !h.decodeBody(w, r, &body) {
		return
	}
	res, err := h.siatUC.EmitirDocumentoAjuste(r.Context(), companyID, chi.URLParam(r, "pointOfSaleId"), body)
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

// authenticatedCompany usa únicamente el tenant autenticado. Los identificadores
// heredados en ruta o query solo se aceptan cuando coinciden con ese tenant.
func authenticatedCompany(w http.ResponseWriter, r *http.Request) (string, bool) {
	companyID, ok := deliveryHttp.CompanyIDFromContext(r.Context())
	if !ok {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "no autorizado: falta company_id en contexto")
		return "", false
	}
	legacyIDs := append([]string{chi.URLParam(r, "companyId")}, r.URL.Query()["company_id"]...)
	for _, requestedID := range legacyIDs {
		if requestedID != "" && requestedID != companyID {
			deliveryHttp.WriteErrorBody(w, http.StatusForbidden, "FORBIDDEN", "la empresa solicitada no coincide con el tenant autenticado")
			return "", false
		}
	}
	return companyID, true
}

// decodeBody acepta un cuerpo ausente o un único objeto JSON del contrato.
func (h *handler) decodeBody(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.Body == nil {
		return true
	}
	decoder := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); errors.Is(err, io.EOF) {
		return true
	} else if err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		deliveryHttp.RespondValidation(w, "el payload debe ser un objeto JSON")
		return false
	}
	if err := decoder.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		deliveryHttp.RespondValidation(w, "el payload debe contener un único objeto JSON")
		return false
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(out); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido o campos no permitidos")
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
