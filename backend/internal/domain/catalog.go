package domain

// CatalogItem es un elemento de un catálogo sincronizado del SIAT.
// Tipo identifica el catálogo de origen (p.ej. "unidadMedida", "metodoPago",
// "actividades", "tipoDocumentoIdentidad", ...).
type CatalogItem struct {
	Codigo      int    `json:"codigo"`
	Descripcion string `json:"descripcion"`
	Tipo        string `json:"tipo"`
}

// SiatActividad es una actividad económica del catálogo CAEB sincronizada del
// SIAT (sincronizarActividades). CodigoCaeb es texto porque el SIAT lo envía
// como cadena.
type SiatActividad struct {
	CodigoCaeb    string `json:"codigo_caeb"`
	Descripcion   string `json:"descripcion"`
	TipoActividad string `json:"tipo_actividad"`
}

// SiatLeyenda es una leyenda oficial de factura asociada a una actividad
// económica (sincronizarListaLeyendasFactura).
type SiatLeyenda struct {
	CodigoActividad    string `json:"codigo_actividad"`
	DescripcionLeyenda string `json:"descripcion_leyenda"`
}

// SiatActividadDocSector es la relación entre una actividad económica y un
// documento-sector del SIAT (sincronizarListaActividadesDocumentoSector). Es la
// fuente para resolver el documento-sector con el que se emite una factura.
type SiatActividadDocSector struct {
	CodigoActividad       string `json:"codigo_actividad"`
	CodigoDocumentoSector int    `json:"codigo_documento_sector"`
	TipoDocumentoSector   string `json:"tipo_documento_sector"`
}
