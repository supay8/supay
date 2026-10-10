package usecase

import (
	"errors"
	"strconv"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

func (uc *SiatUsecase) ListSinProducts(companyID, query string, limit, offset int) ([]*domain.SinProduct, int64, error) {
	res, err := uc.ListProductosSinQuery(companyID, query, 0, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return res.Items, res.Total, nil
}

// ListActivitesDocumentSectors mantiene el contrato legado (paginado en memoria).
func (uc *SiatUsecase) ListActivitesDocumentSectors(companyID, query string, limit, offset int) ([]*domain.SiatActividadDocSector, int64, error) {
	if uc.docSectorRepo == nil {
		return nil, 0, errors.New("repositorio actividadesDocumentoSector no configurado")
	}
	items, err := uc.docSectorRepo.List(companyID)
	if err != nil {
		return nil, 0, err
	}
	term := strings.TrimSpace(strings.ToLower(query))
	if term != "" {
		filtered := make([]*domain.SiatActividadDocSector, 0, len(items))
		for _, it := range items {
			haystack := strings.ToLower(it.CodigoActividad + " " + it.TipoDocumentoSector + " " + strconv.Itoa(it.CodigoDocumentoSector))
			if strings.Contains(haystack, term) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	total := int64(len(items))
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*domain.SiatActividadDocSector{}, total, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], total, nil
}

// CatalogoResultado agrupa los elementos de un catálogo sincronizado para el
// endpoint de lectura. Items es polimórfico: paramétricas devuelven
// {codigo, descripcion}, actividades {codigo_caeb, descripcion,
// tipo_actividad}, etc., fiel a la estructura que entrega el SIAT.
type CatalogoResultado struct {
	Tipo     string `json:"tipo"`
	Cantidad int    `json:"cantidad"`
	Items    []any  `json:"items"`
}

func toCatalogoResultado(tipo string, items []any) *CatalogoResultado {
	return &CatalogoResultado{Tipo: tipo, Cantidad: len(items), Items: items}
}

// ListCatalog devuelve el catálogo sincronizado de la empresa para el tipo
// indicado (p.ej. "actividades", "leyendasFactura", "tipoMoneda"). Con tipo
// vacío o "all" devuelve todos los catálogos almacenados agrupados por tipo.
func (uc *SiatUsecase) ListCatalog(companyID, tipo string) (any, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, domain.NewBadRequestError("companyId es obligatorio")
	}
	tipo = strings.TrimSpace(tipo)
	if tipo == "" || strings.EqualFold(tipo, "all") {
		return uc.ListAllCatalogs(companyID)
	}
	items, err := uc.listCatalogByTipo(companyID, tipo)
	if err != nil {
		return nil, err
	}
	return toCatalogoResultado(tipo, items), nil
}

// ListAllCatalogs consulta cada catálogo almacenado; un catálogo sin repos
// configurado o con error se omite (la respuesta incluye lo disponible).
func (uc *SiatUsecase) ListAllCatalogs(companyID string) (any, error) {
	out := make(map[string]*CatalogoResultado)
	for _, op := range ports.FiscalSyncOperations {
		switch op {
		case ports.OpFechaHora, ports.OpVerificarComunicacion:
			continue // operativos, no almacenan catálogo
		}
		items, err := uc.listCatalogByTipo(companyID, string(op))
		if err != nil {
			continue
		}
		out[string(op)] = toCatalogoResultado(string(op), items)
	}
	return out, nil
}

func (uc *SiatUsecase) listCatalogByTipo(companyID, tipo string) ([]any, error) {
	op := ports.FiscalSyncOperation(tipo)
	switch op {
	case ports.OpActividades:
		if uc.actividadRepo == nil {
			return nil, errors.New("catálogo de actividades no disponible")
		}
		items, err := uc.actividadRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case ports.OpLeyendasFactura:
		if uc.leyendaRepo == nil {
			return nil, errors.New("catálogo de leyendas no disponible")
		}
		items, err := uc.leyendaRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case ports.OpActividadesDocumentoSector:
		if uc.docSectorRepo == nil {
			return nil, errors.New("catálogo actividadesDocumentoSector no disponible")
		}
		items, err := uc.docSectorRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case ports.OpProductosServicios:
		if uc.sinProductRepo == nil {
			return nil, errors.New("catálogo de productos SIN no disponible")
		}
		items, err := uc.sinProductRepo.ListAll(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil

	case ports.OpTipoPuntoVenta:
		if uc.tipoPVRepo == nil {
			return nil, errors.New("catálogo de tipos de punto de venta no disponible")
		}
		items, err := uc.tipoPVRepo.List(companyID)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil
	}

	if _, ok := ports.ParseFiscalSyncOperation(tipo); !ok {
		return nil, domain.NewBadRequestError("Catálogo desconocido: " + tipo)
	}
	if op == ports.OpFechaHora || op == ports.OpVerificarComunicacion {
		return nil, domain.NewBadRequestError("La operación " + tipo + " no almacena catálogo")
	}
	parametricas, err := uc.catalogRepo.List(companyID, tipo)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(parametricas))
	for i, item := range parametricas {
		out[i] = item
	}
	return out, nil
}
