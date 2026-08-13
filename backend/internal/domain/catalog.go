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
