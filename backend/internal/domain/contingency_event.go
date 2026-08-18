package domain

import "time"

// ContingencyEvent registra localmente un evento significativo (contingencia)
// reportado al SIAT. SiatEventCode guarda el codigoRecepcionEventoSignificativo
// que devuelve registroEventoSignificativo y que el envío de paquetes debe usar
// como codigoEvento en recepcionPaqueteFactura.
type ContingencyEvent struct {
	ID            string     `json:"id"`
	PointOfSaleID string     `json:"point_of_sale_id"`
	Reason        string     `json:"reason"`
	Description   *string    `json:"description,omitempty"`
	StartDate     time.Time  `json:"start_date"`
	EndDate       *time.Time `json:"end_date,omitempty"`
	SiatEventCode *string    `json:"siat_event_code,omitempty"`
	IsSynced      bool       `json:"is_synced"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ContingencyEventRepository interface {
	Create(e *ContingencyEvent) error
	GetLatestByPointOfSale(pointOfSaleID string) (*ContingencyEvent, error)
}
