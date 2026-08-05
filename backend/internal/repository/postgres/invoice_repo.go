package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

type PostgresInvoiceRepository struct {
	db *gorm.DB
}

func NewPostgresInvoiceRepository(db *gorm.DB) *PostgresInvoiceRepository {
	return &PostgresInvoiceRepository{db: db}
}

func (r *PostgresInvoiceRepository) Create(inv *models.Invoice) error {
	return r.db.Create(inv).Error
}

func (r *PostgresInvoiceRepository) GetByID(id string) (*models.Invoice, error) {
	var inv models.Invoice
	if err := r.db.Preload("Items").Preload("PointOfSale").Preload("Company").Preload("Customer").Preload("CufdRecord").First(&inv, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *PostgresInvoiceRepository) Update(inv *models.Invoice) error {
	return r.db.Save(inv).Error
}

func (r *PostgresInvoiceRepository) FindActiveCufdForPointOfSale(pointOfSaleId string, at time.Time) (*models.Cufd, error) {
	var cufd models.Cufd
	if err := r.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ?", pointOfSaleId, at, at).Order("valid_from desc").First(&cufd).Error; err != nil {
		return nil, err
	}
	return &cufd, nil
}
