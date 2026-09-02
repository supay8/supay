package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresCatalogSyncStateRepository struct{ db *gorm.DB }

func NewPostgresCatalogSyncStateRepository(db *gorm.DB) domain.CatalogSyncStateRepository {
	return &PostgresCatalogSyncStateRepository{db: db}
}

// Upsert inserta o actualiza el estado de sincronización de una operación en
// una sola sentencia (ON CONFLICT sobre la unicidad company+pos+operación).
func (r *PostgresCatalogSyncStateRepository) Upsert(state domain.CatalogSyncState) error {
	row := models.CatalogSyncState{ID: uuid.NewString(), CompanyId: state.CompanyID, PointOfSaleId: state.PointOfSaleID, Operation: state.Operation, Status: state.Status, RowsSaved: state.RowsSaved, SyncedAt: state.SyncedAt, Error: state.Error}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "company_id"},
			{Name: "point_of_sale_id"},
			{Name: "operation"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"status", "rows_saved", "synced_at", "error", "updated_at"}),
	}).Create(&row).Error
}

func (r *PostgresCatalogSyncStateRepository) List(companyID, pointOfSaleID string) ([]*domain.CatalogSyncState, error) {
	var rows []models.CatalogSyncState
	if err := r.db.Where("company_id = ? AND point_of_sale_id = ?", companyID, pointOfSaleID).Order("operation ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.CatalogSyncState, 0, len(rows))
	for i := range rows {
		out = append(out, &domain.CatalogSyncState{CompanyID: rows[i].CompanyId, PointOfSaleID: rows[i].PointOfSaleId, Operation: rows[i].Operation, Status: rows[i].Status, RowsSaved: rows[i].RowsSaved, SyncedAt: rows[i].SyncedAt, Error: rows[i].Error})
	}
	return out, nil
}
