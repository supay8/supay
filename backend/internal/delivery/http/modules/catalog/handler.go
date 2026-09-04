package catalog

import (
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type catalogService interface {
	CatalogReadiness(companyID, pointOfSaleID string) (*domain.CatalogReadiness, error)
	ListActividadesEconomicas(companyID, tipoActividad string) (*usecase.ActividadesEconomicasResult, error)
	ListCompanyActividadesEconomicas(companyID string) (*usecase.CompanyActividadesResult, error)
	ListDocumentosSector(companyID, codigoActividad string) (*usecase.DocumentosSectorResult, error)
	ListLeyendasFactura(companyID, codigoActividad string) (*usecase.LeyendasFacturaResult, error)
	ListProductosSinQuery(companyID, query string, codigoActividad int64, limit, offset int) (*usecase.ProductosSinResult, error)
	EmisionBootstrap(companyID, codigoActividad string) (*usecase.EmisionBootstrapResult, error)
	ListParametricCatalog(companyID, catalogSlug string) (*usecase.CatalogItemsResult, error)
}

type handler struct {
	siatUC catalogService
}

func newHandler(siatUC catalogService) *handler {
	return &handler{siatUC: siatUC}
}

func (h *handler) readiness(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "id")
	if companyID == "" {
		companyID = r.URL.Query().Get("company_id")
	}
	readiness, err := h.siatUC.CatalogReadiness(companyID, r.URL.Query().Get("point_of_sale_id"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, readiness)
}

func (h *handler) listActividadesEconomicas(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListActividadesEconomicas(chi.URLParam(r, "id"), r.URL.Query().Get("tipo_actividad"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) listCompanyActividadesEconomicas(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListCompanyActividadesEconomicas(chi.URLParam(r, "id"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) listDocumentosSector(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListDocumentosSector(chi.URLParam(r, "id"), r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) listLeyendasFactura(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListLeyendasFactura(chi.URLParam(r, "id"), r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) listProductosSin(w http.ResponseWriter, r *http.Request) {
	codigoActividad, err := usecase.ParseCodigoActividadInt64(r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("query")
	}
	limit := deliveryHttp.ParseQueryInt(r.URL.Query().Get("limit"), 50)
	offset := deliveryHttp.ParseQueryInt(r.URL.Query().Get("offset"), 0)
	res, err := h.siatUC.ListProductosSinQuery(chi.URLParam(r, "id"), q, codigoActividad, limit, offset)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

func (h *handler) emisionBootstrap(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.EmisionBootstrap(chi.URLParam(r, "id"), r.URL.Query().Get("codigo_actividad"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

// listParametric sirve catálogos homogéneos por slug de dominio
// (p.ej. tipos-moneda, metodos-pago).
func (h *handler) listParametric(w http.ResponseWriter, r *http.Request) {
	res, err := h.siatUC.ListParametricCatalog(chi.URLParam(r, "id"), chi.URLParam(r, "catalogSlug"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, res)
}

// listPerfilesDocumentoSector expone metadata multi-sector de Supay (no SIAT).
func (h *handler) listPerfilesDocumentoSector(w http.ResponseWriter, r *http.Request) {
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
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *handler) getPerfilDocumentoSector(w http.ResponseWriter, r *http.Request) {
	codigo := deliveryHttp.ParseQueryInt(chi.URLParam(r, "codigo"), 0)
	if codigo <= 0 {
		deliveryHttp.RespondValidation(w, "codigo de documento-sector inválido")
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
		deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{
			"codigo_documento_sector": p.Codigo,
			"nombre":                  p.Nombre,
			"tipo_factura_documento":  p.TipoDocumentoResuelto(0),
			"layout":                  p.Layout,
			"soportado":               p.Soportado,
			"campos_extra":            campos,
		})
		return
	}
	deliveryHttp.RespondNotFound(w, "perfil de documento-sector no encontrado")
}
