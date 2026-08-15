package domain

import "time"

type Cufd struct {
	ID            string    `json:"id"`
	PointOfSaleID string    `json:"point_of_sale_id"`
	Cufd          string    `json:"cufd"`
	ControlCode   string    `json:"control_code"`
	Direccion     string    `json:"direccion"`
	CodigoQR      *string   `json:"codigo_qr,omitempty"`
	ValidFrom     time.Time `json:"valid_from"`
	ValidTo       time.Time `json:"valid_to"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type CufdRepository interface {
	Create(c *Cufd) error
	GetActiveByPos(pointOfSaleID string) (*Cufd, error)
	GetByPosAndWindow(pointOfSaleID string, from, to time.Time) (*Cufd, error)
	DeactivateExpired() error
}
