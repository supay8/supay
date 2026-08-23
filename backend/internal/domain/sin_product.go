package domain

import "time"

type SinProduct struct {
	ID                string    `json:"id"`
	CompanyID         string    `json:"company_id"`
	CodigoProductoSin int64     `json:"codigo_producto_sin"`
	CodigoActividad   int64     `json:"codigo_actividad"`
	Descripcion       string    `json:"descripcion"`
	Active            bool      `json:"active"`
	SyncedAt          time.Time `json:"synced_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type SinProductRepository interface {
	Replace(companyID string, products []SinProduct, syncedAt time.Time) error
	List(companyID, query string, limit, offset int) ([]*SinProduct, int64, error)
	ListAll(companyID string) ([]*SinProduct, error)
	GetByCode(companyID string, code int64) (*SinProduct, error)
}

type CatalogSyncState struct {
	CompanyID     string     `json:"company_id"`
	PointOfSaleID string     `json:"point_of_sale_id"`
	Operation     string     `json:"operation"`
	Status        string     `json:"status"`
	RowsSaved     int        `json:"rows_saved"`
	SyncedAt      *time.Time `json:"synced_at,omitempty"`
	Error         string     `json:"error,omitempty"`
}

type CatalogReadiness struct {
	Ready   bool               `json:"ready"`
	Missing []string           `json:"missing,omitempty"`
	States  []CatalogSyncState `json:"states"`
}

type CatalogSyncStateRepository interface {
	Upsert(state CatalogSyncState) error
	List(companyID, pointOfSaleID string) ([]*CatalogSyncState, error)
}
