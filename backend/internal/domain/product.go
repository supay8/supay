package domain

import "time"

type Product struct {
	ID        string           `json:"id"`
	CompanyID string           `json:"company_id"`
	SKU       string           `json:"sku"`
	Name      string           `json:"name"`
	Active    bool             `json:"active"`
	Mappings  []ProductMapping `json:"mappings,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type ProductMapping struct {
	ID                    string    `json:"id"`
	ProductID             string    `json:"product_id"`
	SinProductID          *string   `json:"sin_product_id,omitempty"`
	CodigoProductoSin     int64     `json:"codigo_producto_sin"`
	CodigoActividad       string    `json:"codigo_actividad"`
	CodigoDocumentoSector int       `json:"codigo_documento_sector"`
	UnidadMedida          int       `json:"unidad_medida"`
	IsDefault             bool      `json:"is_default"`
	Active                bool      `json:"active"`
	SyncedAt              time.Time `json:"synced_at"`
}

type ProductRepository interface {
	Create(*Product) error
	GetByID(companyID, id string) (*Product, error)
	GetBySKU(companyID, sku string) (*Product, error)
	List(companyID string) ([]*Product, error)
	UpsertMapping(companyID string, mapping ProductMapping) error
}
