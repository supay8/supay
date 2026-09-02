package http

import (
	"net/http"

	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// CatalogHandler expone endpoints GET de consulta de catálogos sincronizados.
// La sincronización sigue en SiatHandler; aquí solo se lee PostgreSQL.
type CatalogHandler struct {
	siatUC *usecase.SiatUsecase
}

func NewCatalogHandler(siatUC *usecase.SiatUsecase) *CatalogHandler {
	return &CatalogHandler{siatUC: siatUC}
}

func (h *CatalogHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "id")
	if companyID == "" {
		companyID = r.URL.Query().Get("company_id")
	}
	readiness, err := h.siatUC.CatalogReadiness(companyID, r.URL.Query().Get("point_of_sale_id"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, readiness)
}

func (h *CatalogHandler) ListActividadesEconomicas(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListActividadesEconomicas(chi.URLParam(r, "id"), r.URL.Query().Get("tipo_actividad"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *CatalogHandler) ListCompanyActividadesEconomicas(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListCompanyActividadesEconomicas(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *CatalogHandler) ListDocumentosSector(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListDocumentosSector(chi.URLParam(r, "id"), r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *CatalogHandler) ListLeyendasFactura(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListLeyendasFactura(chi.URLParam(r, "id"), r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *CatalogHandler) ListProductosSin(w http.ResponseWriter, r *http.Request) {
	codigoActividad, err := usecase.ParseCodigoActividadInt64(r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		respondError(w, err)
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("query")
	}
	limit := parseQueryInt(r.URL.Query().Get("limit"), 50)
	offset := parseQueryInt(r.URL.Query().Get("offset"), 0)
	res, err := h.siatUC.ListProductosSinQuery(chi.URLParam(r, "id"), q, codigoActividad, limit, offset)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *CatalogHandler) EmisionBootstrap(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.EmisionBootstrap(chi.URLParam(r, "id"), r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ListParametric sirve catálogos homogéneos por slug de dominio
// (p.ej. tipos-moneda, metodos-pago).
func (h *CatalogHandler) ListParametric(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListParametricCatalog(chi.URLParam(r, "id"), chi.URLParam(r, "catalogSlug"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ListPerfilesDocumentoSector expone metadata multi-sector de Supay (no SIAT).
func (h *CatalogHandler) ListPerfilesDocumentoSector(w http.ResponseWriter, r *http.Request) {
	perfiles := siat.PerfilesSector()
	items := make([]map[string]any, 0, len(perfiles))
	for _, p := range perfiles {
		campos := make([]map[string]any, 0, len(p.Campos))
		for _, c := range p.Campos {
			campos = append(campos, map[string]any{
				"clave":     c.JSON,
				"tipo":      c.Tipo,
				"requerido": c.Requerido,
				"etiqueta":  c.Etiqueta,
				"ejemplo":   c.Ejemplo,
			})
		}
		items = append(items, map[string]any{
			"codigo_documento_sector": p.Codigo,
			"nombre":                  p.Nombre,
			"tipo_factura_documento":  p.TipoDocumentoResuelto(0),
			"layout":                  p.Layout,
			"soportado":               p.Soportado,
			"campos_extra":            campos,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *CatalogHandler) GetPerfilDocumentoSector(w http.ResponseWriter, r *http.Request) {
	codigo := parseQueryInt(chi.URLParam(r, "codigo"), 0)
	if codigo <= 0 {
		respondValidation(w, "codigo de documento-sector inválido")
		return
	}
	for _, p := range siat.PerfilesSector() {
		if p.Codigo != codigo {
			continue
		}
		campos := make([]map[string]any, 0, len(p.Campos))
		for _, c := range p.Campos {
			campos = append(campos, map[string]any{
				"clave":     c.JSON,
				"tipo":      c.Tipo,
				"requerido": c.Requerido,
				"etiqueta":  c.Etiqueta,
				"ejemplo":   c.Ejemplo,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"codigo_documento_sector": p.Codigo,
			"nombre":                  p.Nombre,
			"tipo_factura_documento":  p.TipoDocumentoResuelto(0),
			"layout":                  p.Layout,
			"soportado":               p.Soportado,
			"campos_extra":            campos,
		})
		return
	}
	respondNotFound(w, "perfil de documento-sector no encontrado")
}
