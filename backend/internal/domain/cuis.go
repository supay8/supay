package domain

import "time"

type Cuis struct {
	ID            string    `json:"id"`
	PointOfSaleID string    `json:"point_of_sale_id"`
	Cuis          string    `json:"cuis"`
	ValidFrom     time.Time `json:"valid_from"`
	ValidTo       time.Time `json:"valid_to"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type CuisRepository interface {
	Create(c *Cuis) error
	GetActiveByPos(pointOfSaleID string) (*Cuis, error)
	DeactivateExpired() error
}
