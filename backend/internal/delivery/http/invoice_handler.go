package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type InvoiceHandler struct {
	uc *usecase.InvoiceUsecase
}

func NewInvoiceHandler(uc *usecase.InvoiceUsecase) *InvoiceHandler {
	return &InvoiceHandler{uc: uc}
}

func (h *InvoiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
		return
	}
	inv, err := h.uc.Create(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) ListByPointOfSale(w http.ResponseWriter, r *http.Request) {
	pointOfSaleID := r.URL.Query().Get("pointOfSaleId")
	if pointOfSaleID == "" {
		writeJSONError(w, http.StatusBadRequest, "pointOfSaleId es obligatorio")
		return
	}
	list, err := h.uc.ListByPointOfSale(pointOfSaleID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *InvoiceHandler) Emit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.Emit(r.Context(), id)
	if err != nil {
		var rejected *usecase.EmissionRejectedError
		if errors.As(err, &rejected) {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) SiatStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.VerifyStatus(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) Annul(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	var req struct {
		CodigoMotivo int `json:"codigo_motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
		return
	}
	inv, err := h.uc.Annul(r.Context(), id, req.CodigoMotivo)
	if err != nil {
		var rejected *usecase.EmissionRejectedError
		if errors.As(err, &rejected) {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) RevertAnnul(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.RevertAnnul(r.Context(), id)
	if err != nil {
		var rejected *usecase.EmissionRejectedError
		if errors.As(err, &rejected) {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

type sectorCampoDTO struct {
	JSON      string `json:"json"`
	Metodo    string `json:"-"`
	Requerido bool   `json:"requerido"`
	Tipo      string `json:"tipo"`
}

type sectorDTO struct {
	Codigo             int              `json:"codigo"`
	Nombre             string           `json:"nombre"`
	TipoDocumento      int              `json:"tipo_documento"`
	Operacion          string           `json:"operacion"`
	Fachada            string           `json:"fachada"`
	Layout             string           `json:"layout,omitempty"`
	Modalidades        []int            `json:"modalidades,omitempty"`
	TieneBuilder       bool             `json:"tiene_builder"`
	RequiereArchivo    bool             `json:"requiere_archivo"`
	ConDetalle         bool             `json:"con_detalle"`
	DetalleUnico       bool             `json:"detalle_unico"`
	MontoSujetoIvaCero bool             `json:"monto_sujeto_iva_cero"`
	Ajuste             bool             `json:"es_ajuste"`
	Campos             []sectorCampoDTO `json:"campos_datos_sector"`
}

// Sectores expone el catálogo de documentos-sector soportados con la
// declaración de sus campos datos_sector, para que los clientes construyan
// formularios dinámicos y validen en el frontend.
func (h *InvoiceHandler) Sectores(w http.ResponseWriter, r *http.Request) {
	perfiles := siat.PerfilesSector()
	salida := make([]sectorDTO, 0, len(perfiles))
	for _, p := range perfiles {
		campos := make([]sectorCampoDTO, 0, len(p.Campos))
		for _, c := range p.Campos {
			campos = append(campos, sectorCampoDTO{
				JSON:      c.JSON,
				Requerido: c.Requerido,
				Tipo:      c.Tipo,
			})
		}
		salida = append(salida, sectorDTO{
			Codigo:             p.Codigo,
			Nombre:             p.Nombre,
			TipoDocumento:      p.TipoDocumentoResuelto(0),
			Operacion:          p.Operacion.String(),
			Fachada:            p.Facade.String(),
			Layout:             p.Layout,
			Modalidades:        append([]int(nil), p.Modalidades...),
			TieneBuilder:       p.HasBuilder(),
			RequiereArchivo:    !p.HasBuilder(),
			ConDetalle:         p.ConDetalle,
			DetalleUnico:       p.DetalleUnico,
			MontoSujetoIvaCero: p.MontoSujetoIvaCero,
			Ajuste:             p.EsAjuste(),
			Campos:             campos,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(salida)
}
