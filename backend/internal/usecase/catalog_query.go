package usecase

import (
	"strconv"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
)

// Slugs de dominio Supay → operación/tipo SIAT (solo paramétricos homogéneos).
var catalogSlugToTipo = map[string]string{
	"tipos-moneda":               string(siat.OpTipoMoneda),
	"metodos-pago":               string(siat.OpTipoMetodoPago),
	"unidades-medida":            string(siat.OpUnidadMedida),
	"tipos-documento-identidad":  string(siat.OpTipoDocumentoIdentidad),
	"motivos-anulacion":          string(siat.OpMotivoAnulacion),
	"tipos-emision":              string(siat.OpTipoEmision),
	"tipos-factura":              string(siat.OpTiposFactura),
	"tipos-documento-sector":     string(siat.OpTipoDocumentoSector),
	"tipos-punto-venta":          string(siat.OpTipoPuntoVenta),
	"tipos-habitacion":           string(siat.OpTipoHabitacion),
	"paises-origen":              string(siat.OpPaisOrigen),
	"eventos-significativos":     string(siat.OpEventosSignificativos),
	"mensajes-servicios":         string(siat.OpMensajesServicios),
}

// CatalogItemsResult es la respuesta uniforme de catálogos paramétricos.
type CatalogItemsResult struct {
	Catalog string               `json:"catalog"`
	Items   []CatalogCodigoDesc  `json:"items"`
	Total   int                  `json:"total"`
}

// CatalogCodigoDesc es el shape público de un ítem paramétrico.
type CatalogCodigoDesc struct {
	Codigo      int    `json:"codigo"`
	Descripcion string `json:"descripcion"`
}

// ActividadesEconomicasResult lista actividades CAEB sincronizadas.
type ActividadesEconomicasResult struct {
	Items []*domain.SiatActividad `json:"items"`
	Total int                     `json:"total"`
}

// CompanyActividadesResult distingue la actividad principal de la empresa.
type CompanyActividadesResult struct {
	ActividadPrincipal *domain.SiatActividad   `json:"actividad_principal,omitempty"`
	Actividades        []*domain.SiatActividad `json:"actividades"`
}

// DocumentoSectorItem enriquece la relación act↔sector con el nombre de Supay.
type DocumentoSectorItem struct {
	CodigoActividad       string `json:"codigo_actividad"`
	CodigoDocumentoSector int    `json:"codigo_documento_sector"`
	TipoDocumentoSector   string `json:"tipo_documento_sector"`
	Nombre                string `json:"nombre,omitempty"`
}

// DocumentosSectorResult lista relaciones actividad ↔ documento-sector.
type DocumentosSectorResult struct {
	Items []DocumentoSectorItem `json:"items"`
	Total int                   `json:"total"`
}

// LeyendasFacturaResult lista leyendas oficiales.
type LeyendasFacturaResult struct {
	Items []*domain.SiatLeyenda `json:"items"`
	Total int                   `json:"total"`
}

// ProductosSinResult lista paginada de productos/servicios SIN.
type ProductosSinResult struct {
	Items  []*domain.SinProduct `json:"items"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
	Total  int64                `json:"total"`
}

// EmisionBootstrapResult agrupa catálogos pequeños para pantallas de emisión.
type EmisionBootstrapResult struct {
	Actividades               []*domain.SiatActividad `json:"actividades"`
	DocumentosSector          []DocumentoSectorItem   `json:"documentos_sector"`
	Leyendas                  []*domain.SiatLeyenda   `json:"leyendas"`
	TiposMoneda               []CatalogCodigoDesc     `json:"tipos_moneda"`
	MetodosPago               []CatalogCodigoDesc     `json:"metodos_pago"`
	UnidadesMedida            []CatalogCodigoDesc     `json:"unidades_medida"`
	TiposDocumentoIdentidad   []CatalogCodigoDesc     `json:"tipos_documento_identidad"`
	MotivosAnulacion          []CatalogCodigoDesc     `json:"motivos_anulacion"`
}

// SincronizacionResumen es la respuesta pública del POST de sincronización:
// solo estado del proceso, sin ítems de catálogo.
type SincronizacionResumen struct {
	Success         bool                        `json:"success"`
	CompanyID       string                      `json:"company_id"`
	PointOfSaleID   string                      `json:"point_of_sale_id"`
	Operations      []SincronizacionOpResult     `json:"operations"`
	Errors          []SincronizacionOpError      `json:"errors,omitempty"`
	Readiness       *domain.CatalogReadiness     `json:"readiness,omitempty"`
}

// ResolveCatalogTipo traduce un slug de dominio Supay al tipo SIAT almacenado.
// Acepta también el nombre camelCase legado (tipoMoneda) por compatibilidad.
func ResolveCatalogTipo(slugOrTipo string) (slug string, tipo string, ok bool) {
	raw := strings.TrimSpace(slugOrTipo)
	if raw == "" {
		return "", "", false
	}
	if tipo, found := catalogSlugToTipo[raw]; found {
		return raw, tipo, true
	}
	for slug, tipo := range catalogSlugToTipo {
		if tipo == raw {
			return slug, tipo, true
		}
	}
	return "", "", false
}

// ListActividadesEconomicas devuelve el catálogo CAEB de la empresa.
func (uc *SiatUsecase) ListActividadesEconomicas(companyID, tipoActividad string) (*ActividadesEconomicasResult, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	if uc.actividadRepo == nil {
		return nil, domain.NewBadRequestError("catálogo de actividades no disponible")
	}
	items, err := uc.actividadRepo.List(companyID)
	if err != nil {
		return nil, err
	}
	filtro := strings.TrimSpace(tipoActividad)
	if filtro != "" {
		filtered := make([]*domain.SiatActividad, 0, len(items))
		for _, it := range items {
			if strings.EqualFold(it.TipoActividad, filtro) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	return &ActividadesEconomicasResult{Items: items, Total: len(items)}, nil
}

// ListCompanyActividadesEconomicas expone la actividad principal de la empresa
// y el catálogo sincronizado completo.
func (uc *SiatUsecase) ListCompanyActividadesEconomicas(companyID string) (*CompanyActividadesResult, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	if uc.companyRepo == nil {
		return nil, domain.NewBadRequestError("repositorio de empresas no configurado")
	}
	company, err := uc.companyRepo.GetByID(companyID)
	if err != nil {
		return nil, err
	}
	acts, err := uc.ListActividadesEconomicas(companyID, "")
	if err != nil {
		return nil, err
	}
	out := &CompanyActividadesResult{Actividades: acts.Items}
	if company.CodigoActividad != nil {
		codigo := strings.TrimSpace(*company.CodigoActividad)
		for _, a := range acts.Items {
			if a.CodigoCaeb == codigo {
				out.ActividadPrincipal = a
				break
			}
		}
		if out.ActividadPrincipal == nil && codigo != "" {
			out.ActividadPrincipal = &domain.SiatActividad{CodigoCaeb: codigo}
		}
	}
	return out, nil
}

// ListDocumentosSector consulta la relación actividad ↔ documento-sector.
func (uc *SiatUsecase) ListDocumentosSector(companyID, codigoActividad string) (*DocumentosSectorResult, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	if uc.docSectorRepo == nil {
		return nil, domain.NewBadRequestError("catálogo documentos-sector no disponible")
	}
	var (
		items []*domain.SiatActividadDocSector
		err   error
	)
	actividad := strings.TrimSpace(codigoActividad)
	if actividad != "" {
		items, err = uc.docSectorRepo.ListByActividad(companyID, actividad)
	} else {
		items, err = uc.docSectorRepo.List(companyID)
	}
	if err != nil {
		return nil, err
	}
	out := make([]DocumentoSectorItem, 0, len(items))
	for _, it := range items {
		out = append(out, DocumentoSectorItem{
			CodigoActividad:       it.CodigoActividad,
			CodigoDocumentoSector: it.CodigoDocumentoSector,
			TipoDocumentoSector:   it.TipoDocumentoSector,
			Nombre:                nombreDocumentoSector(it.CodigoDocumentoSector),
		})
	}
	return &DocumentosSectorResult{Items: out, Total: len(out)}, nil
}

// ListLeyendasFactura lista leyendas; opcionalmente filtradas por actividad.
func (uc *SiatUsecase) ListLeyendasFactura(companyID, codigoActividad string) (*LeyendasFacturaResult, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	if uc.leyendaRepo == nil {
		return nil, domain.NewBadRequestError("catálogo de leyendas no disponible")
	}
	var (
		items []*domain.SiatLeyenda
		err   error
	)
	actividad := strings.TrimSpace(codigoActividad)
	if actividad != "" {
		items, err = uc.leyendaRepo.ListByActividad(companyID, actividad)
	} else {
		items, err = uc.leyendaRepo.List(companyID)
	}
	if err != nil {
		return nil, err
	}
	return &LeyendasFacturaResult{Items: items, Total: len(items)}, nil
}

// ListProductosSinQuery pagina productos/servicios SIN con búsqueda opcional.
func (uc *SiatUsecase) ListProductosSinQuery(companyID, query string, codigoActividad int64, limit, offset int) (*ProductosSinResult, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	if uc.sinProductRepo == nil {
		return nil, domain.NewBadRequestError("catálogo de productos SIN no disponible")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, total, err := uc.sinProductRepo.List(companyID, query, codigoActividad, limit, offset)
	if err != nil {
		return nil, err
	}
	return &ProductosSinResult{Items: items, Limit: limit, Offset: offset, Total: total}, nil
}

// ListParametricCatalog sirve catálogos homogéneos codigo/descripcion por slug.
func (uc *SiatUsecase) ListParametricCatalog(companyID, catalogSlug string) (*CatalogItemsResult, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	slug, tipo, ok := ResolveCatalogTipo(catalogSlug)
	if !ok {
		return nil, domain.NewBadRequestError("catálogo desconocido: " + catalogSlug)
	}

	if tipo == string(siat.OpTipoPuntoVenta) {
		if uc.tipoPVRepo == nil {
			return nil, domain.NewBadRequestError("catálogo de tipos de punto de venta no disponible")
		}
		tipos, err := uc.tipoPVRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		items := make([]CatalogCodigoDesc, 0, len(tipos))
		for _, t := range tipos {
			items = append(items, CatalogCodigoDesc{Codigo: t.CodigoClasificador, Descripcion: t.Descripcion})
		}
		return &CatalogItemsResult{Catalog: slug, Items: items, Total: len(items)}, nil
	}

	if uc.catalogRepo == nil {
		return nil, domain.NewBadRequestError("repositorio de catálogos no configurado")
	}
	rows, err := uc.catalogRepo.List(companyID, tipo)
	if err != nil {
		return nil, err
	}
	items := make([]CatalogCodigoDesc, 0, len(rows))
	for _, row := range rows {
		items = append(items, CatalogCodigoDesc{Codigo: row.Codigo, Descripcion: row.Descripcion})
	}
	return &CatalogItemsResult{Catalog: slug, Items: items, Total: len(items)}, nil
}

// EmisionBootstrap agrupa catálogos pequeños necesarios para emitir.
// No incluye productos SIN (usar ListProductosSinQuery).
func (uc *SiatUsecase) EmisionBootstrap(companyID, codigoActividad string) (*EmisionBootstrapResult, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("company_id es obligatorio")
	}
	out := &EmisionBootstrapResult{}

	if acts, err := uc.ListActividadesEconomicas(companyID, ""); err == nil {
		out.Actividades = acts.Items
	}
	actividad := strings.TrimSpace(codigoActividad)
	if actividad == "" && uc.companyRepo != nil {
		if company, err := uc.companyRepo.GetByID(companyID); err == nil && company.CodigoActividad != nil {
			actividad = strings.TrimSpace(*company.CodigoActividad)
		}
	}
	if docs, err := uc.ListDocumentosSector(companyID, actividad); err == nil {
		out.DocumentosSector = docs.Items
	}
	if ley, err := uc.ListLeyendasFactura(companyID, actividad); err == nil {
		out.Leyendas = ley.Items
	}

	out.TiposMoneda = mustParametricItems(uc, companyID, "tipos-moneda")
	out.MetodosPago = mustParametricItems(uc, companyID, "metodos-pago")
	out.UnidadesMedida = mustParametricItems(uc, companyID, "unidades-medida")
	out.TiposDocumentoIdentidad = mustParametricItems(uc, companyID, "tipos-documento-identidad")
	out.MotivosAnulacion = mustParametricItems(uc, companyID, "motivos-anulacion")
	return out, nil
}

// BuildSincronizacionResumen arma la respuesta pública del sync (sin ítems).
func (uc *SiatUsecase) BuildSincronizacionResumen(companyID, posID string, res *SincronizacionResultado) *SincronizacionResumen {
	out := &SincronizacionResumen{
		Success:       res != nil && len(res.Errors) == 0,
		CompanyID:     companyID,
		PointOfSaleID: posID,
	}
	if res != nil {
		out.Operations = res.Operations
		out.Errors = res.Errors
		if res.Company != nil {
			out.CompanyID = res.Company.ID
		}
		if res.PointOfSale != nil {
			out.PointOfSaleID = res.PointOfSale.ID
		}
	}
	if readiness, err := uc.CatalogReadiness(out.CompanyID, out.PointOfSaleID); err == nil {
		out.Readiness = readiness
	}
	return out
}

func mustParametricItems(uc *SiatUsecase, companyID, slug string) []CatalogCodigoDesc {
	res, err := uc.ListParametricCatalog(companyID, slug)
	if err != nil || res == nil {
		return []CatalogCodigoDesc{}
	}
	return res.Items
}

func nombreDocumentoSector(codigo int) string {
	for _, p := range siat.PerfilesSector() {
		if p.Codigo == codigo {
			return p.Nombre
		}
	}
	return ""
}

// ParseCodigoActividadInt64 convierte un query string a int64; vacío → 0.
func ParseCodigoActividadInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, domain.NewBadRequestError("codigo_actividad inválido")
	}
	return n, nil
}
