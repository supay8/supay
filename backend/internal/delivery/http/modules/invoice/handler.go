package invoice

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

const maxIdempotencyKeyLen = 100

// invoiceService es el contrato de operaciones de facturas expuesto por el
// usecase. El handler depende de la interfaz para facilitar tests con mocks.
type invoiceService interface {
	Create(ctx context.Context, req usecase.CreateInvoiceRequest) (*domain.Invoice, error)
	GetByID(id string) (*domain.Invoice, error)
	ListInvoices(filter domain.InvoiceListFilter) ([]*domain.Invoice, int64, error)
	Emit(ctx context.Context, id string) (*domain.Invoice, error)
	VerifyStatus(ctx context.Context, id string) (*domain.Invoice, error)
	Annul(ctx context.Context, id string, codigoMotivo int) (*domain.Invoice, error)
	RevertAnnul(ctx context.Context, id string) (*domain.Invoice, error)
	SectoresHabilitados(companyID string) (map[int]bool, error)
}

type handler struct {
	uc invoiceService
}

func newHandler(uc invoiceService) *handler {
	return &handler{uc: uc}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido: "+err.Error())
		return
	}
	req.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(req.IdempotencyKey) > maxIdempotencyKeyLen {
		deliveryHttp.RespondValidation(w, "Idempotency-Key no puede exceder 100 caracteres")
		return
	}

	inv, err := h.uc.Create(r.Context(), req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}

	status := http.StatusCreated
	if req.IdempotencyKey != "" {
		status = http.StatusOK
	}

	if req.Emit && inv.Status == domain.InvoicePending {
		inv, err = h.uc.Emit(r.Context(), inv.ID)
		if err != nil {
			deliveryHttp.RespondErrorWithInvoiceID(w, err, inv.ID)
			return
		}
	}

	includes := parseIncludes(r.URL.Query().Get("include"))
	deliveryHttp.WriteJSON(w, status, toInvoiceDTO(inv, includes))
}

func (h *handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	inv, err := h.uc.GetByID(id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	includes := parseIncludes(r.URL.Query().Get("include"))
	deliveryHttp.WriteJSON(w, http.StatusOK, toInvoiceDTO(inv, includes))
}

// list expone GET /invoices: listado paginado por punto de venta con filtros
// opcionales de estado y rango de emisión. Nunca devuelve xml/archivo; para
// el XML usá GET /invoices/{id}.
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	pointOfSaleID := query.Get("point_of_sale_id")
	if pointOfSaleID == "" {
		deliveryHttp.RespondValidation(w, "point_of_sale_id es obligatorio")
		return
	}

	filter := domain.InvoiceListFilter{PointOfSaleID: pointOfSaleID}

	if status := query.Get("status"); status != "" {
		st := domain.InvoiceStatus(strings.TrimSpace(status))
		filter.Status = &st
	}
	if from := query.Get("from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			deliveryHttp.RespondValidation(w, "from inválido: usar formato RFC3339 (ej: 2026-08-24T00:00:00-04:00)")
			return
		}
		filter.From = &t
	}
	if to := query.Get("to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			deliveryHttp.RespondValidation(w, "to inválido: usar formato RFC3339 (ej: 2026-08-24T23:59:59-04:00)")
			return
		}
		filter.To = &t
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			deliveryHttp.RespondValidation(w, "limit debe ser un entero mayor a cero")
			return
		}
		filter.Limit = limit
	}
	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			deliveryHttp.RespondValidation(w, "offset debe ser un entero mayor o igual a cero")
			return
		}
		filter.Offset = offset
	}

	list, total, err := h.uc.ListInvoices(filter)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}

	dtos := make([]invoiceDTO, 0, len(list))
	for _, inv := range list {
		dtos = append(dtos, toInvoiceDTO(inv, nil))
	}
	deliveryHttp.RespondList(w, dtos, int(total), filter.Limit, filter.Offset)
}

func (h *handler) emit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	inv, err := h.uc.Emit(r.Context(), id)
	if err != nil {
		deliveryHttp.RespondErrorWithInvoiceID(w, err, id)
		return
	}
	includes := parseIncludes(r.URL.Query().Get("include"))
	deliveryHttp.WriteJSON(w, http.StatusOK, toInvoiceDTO(inv, includes))
}

func (h *handler) siatStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	inv, err := h.uc.VerifyStatus(r.Context(), id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	includes := parseIncludes(r.URL.Query().Get("include"))
	deliveryHttp.WriteJSON(w, http.StatusOK, toInvoiceDTO(inv, includes))
}

func (h *handler) annul(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	var req struct {
		CodigoMotivo int `json:"codigo_motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido: "+err.Error())
		return
	}
	inv, err := h.uc.Annul(r.Context(), id, req.CodigoMotivo)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	includes := parseIncludes(r.URL.Query().Get("include"))
	deliveryHttp.WriteJSON(w, http.StatusOK, toInvoiceDTO(inv, includes))
}

func (h *handler) revertAnnul(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	inv, err := h.uc.RevertAnnul(r.Context(), id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	includes := parseIncludes(r.URL.Query().Get("include"))
	deliveryHttp.WriteJSON(w, http.StatusOK, toInvoiceDTO(inv, includes))
}

// downloadXML expone GET /invoices/{id}/xml: devuelve el XML firmado generado
// por el SIAT. Solo disponible para facturas ya emitidas.
func (h *handler) downloadXML(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	inv, err := h.uc.GetByID(id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	if inv.Xml == nil || strings.TrimSpace(*inv.Xml) == "" {
		deliveryHttp.RespondError(w, domain.NewConflictError("la factura aún no tiene XML disponible"))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", "attachment; filename=\"factura-"+id+".xml\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(*inv.Xml))
}

// sectores expone el catálogo de documentos-sector soportados con la
// declaración de sus campos datos_sector, para que los clientes construyan
// formularios dinámicos y validen en el frontend. Con ?company_id= anota cada
// sector con habilitado según el catálogo de la empresa.
func (h *handler) sectores(w http.ResponseWriter, r *http.Request) {
	var habilitados map[int]bool
	if companyID := r.URL.Query().Get("company_id"); companyID != "" {
		if h.uc == nil {
			deliveryHttp.RespondError(w, domain.NewConflictError("servicio de sectores no disponible"))
			return
		}
		var err error
		habilitados, err = h.uc.SectoresHabilitados(companyID)
		if err != nil {
			deliveryHttp.RespondError(w, err)
			return
		}
	}

	perfiles := siat.PerfilesSector()
	salida := make([]sectorDTO, 0, len(perfiles))
	for _, p := range perfiles {
		campos := make([]sectorCampoDTO, 0, len(p.Campos))
		for _, c := range p.Campos {
			campos = append(campos, aSectorCampoDTO(c))
		}
		camposDetalle := make([]sectorCampoDTO, 0, len(p.CamposDetalle))
		for _, c := range p.CamposDetalle {
			camposDetalle = append(camposDetalle, aSectorCampoDTO(c))
		}

		dto := sectorDTO{
			Codigo:             p.Codigo,
			Nombre:             p.Nombre,
			TipoDocumento:      p.TipoDocumentoResuelto(0),
			Operacion:          p.Operacion.String(),
			Fachada:            p.Facade.String(),
			Layout:             p.Layout,
			Modalidades:        append([]int(nil), p.Modalidades...),
			Soportado:          p.Soportado,
			TieneBuilder:       p.HasBuilder(),
			RequiereArchivo:    !p.HasBuilder(),
			ConDetalle:         p.ConDetalle,
			DetalleUnico:       p.DetalleUnico,
			MontoSujetoIvaCero: p.MontoSujetoIvaCero,
			Ajuste:             p.EsAjuste(),
			Campos:             campos,
			CamposDetalle:      camposDetalle,
		}
		if habilitados != nil {
			habilitado := habilitados[p.Codigo]
			dto.Habilitado = &habilitado
		}
		salida = append(salida, dto)
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, salida)
}
