package domain

import "time"

// CatalogItem es un elemento de un catálogo sincronizado del SIAT.
// Tipo identifica el catálogo de origen (p.ej. "unidadMedida", "metodoPago",
// "actividades", "tipoDocumentoIdentidad", ...).
type CatalogItem struct {
	Codigo      int    `json:"codigo"`
	Descripcion string `json:"descripcion"`
	Tipo        string `json:"tipo"`
}

// CatalogRepository define el contrato para persistir catálogos sincronizados.
type CatalogRepository interface {
	// Replace reemplaza el catálogo de la empresa para el tipo indicado con los
	// valores sincronizados del SIAT (delete + insert en una transacción).
	Replace(companyID, tipo string, items []CatalogItem, syncedAt time.Time) error
	// List devuelve el catálogo vigente de la empresa para el tipo indicado.
	List(companyID, tipo string) ([]*CatalogItem, error)
	// ListAll devuelve todos los catálogos de la empresa agrupados por tipo.
	ListAll(companyID string) (map[string][]*CatalogItem, error)
}

// SiatActividad es una actividad económica del catálogo CAEB sincronizada del
// SIAT (sincronizarActividades). CodigoCaeb es texto porque el SIAT lo envía
// como cadena.
type SiatActividad struct {
	CodigoCaeb    string `json:"codigo_caeb"`
	Descripcion   string `json:"descripcion"`
	TipoActividad string `json:"tipo_actividad"`
}

type SiatActividadRepository interface {
	// Replace reemplaza el catálogo de actividades de la empresa con los valores
	// sincronizados del SIAT.
	Replace(companyID string, items []SiatActividad, syncedAt time.Time) error
	// List devuelve el catálogo vigente ordenado por código CAEB.
	List(companyID string) ([]*SiatActividad, error)
}

// SiatLeyenda es una leyenda oficial de factura asociada a una actividad
// económica (sincronizarListaLeyendasFactura).
type SiatLeyenda struct {
	CodigoActividad    string `json:"codigo_actividad"`
	DescripcionLeyenda string `json:"descripcion_leyenda"`
}

type SiatLeyendaRepository interface {
	// Replace reemplaza el catálogo de leyendas de la empresa con los valores
	// sincronizados del SIAT.
	Replace(companyID string, leyendas []SiatLeyenda, syncedAt time.Time) error
	// List devuelve todas las leyendas vigentes de la empresa.
	List(companyID string) ([]*SiatLeyenda, error)
	// ListByActividad devuelve las leyendas asociadas a una actividad económica.
	ListByActividad(companyID, codigoActividad string) ([]*SiatLeyenda, error)
}

// SiatActividadDocSector es la relación entre una actividad económica y un
// documento-sector del SIAT (sincronizarListaActividadesDocumentoSector). Es la
// fuente para resolver el documento-sector con el que se emite una factura.
type SiatActividadDocSector struct {
	CodigoActividad       string `json:"codigo_actividad"`
	CodigoDocumentoSector int    `json:"codigo_documento_sector"`
	TipoDocumentoSector   string `json:"tipo_documento_sector"`
}

type SiatActividadDocSectorRepository interface {
	// Replace reemplaza la relación actividad-sector de la empresa con los
	// valores sincronizados del SIAT.
	Replace(companyID string, items []SiatActividadDocSector, syncedAt time.Time) error
	// List devuelve todas las relaciones vigentes de la empresa.
	List(companyID string) ([]*SiatActividadDocSector, error)
	// ListByActividad devuelve los sectores habilitados para una actividad.
	ListByActividad(companyID, codigoActividad string) ([]*SiatActividadDocSector, error)
}
