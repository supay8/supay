package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PostgresSiatActividadRepository persiste el catálogo de actividades
// económicas sincronizado del SIAT (tabla siat_actividades).
type PostgresSiatActividadRepository struct{ db *gorm.DB }

func NewPostgresSiatActividadRepository(db *gorm.DB) domain.SiatActividadRepository {
	return &PostgresSiatActividadRepository{db: db}
}

func (r *PostgresSiatActividadRepository) Replace(companyID string, items []domain.SiatActividad, syncedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.SiatActividad{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		rows := make([]models.SiatActividad, 0, len(items))
		for _, it := range items {
			rows = append(rows, models.SiatActividad{
				ID:            uuid.NewString(),
				CompanyId:     companyID,
				CodigoCaeb:    it.CodigoCaeb,
				Descripcion:   it.Descripcion,
				TipoActividad: it.TipoActividad,
				SyncedAt:      syncedAt,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (r *PostgresSiatActividadRepository) List(companyID string) ([]*domain.SiatActividad, error) {
	var rows []models.SiatActividad
	if err := r.db.Where("company_id = ?", companyID).
		Order("codigo_caeb ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.SiatActividad, 0, len(rows))
	for i := range rows {
		out = append(out, &domain.SiatActividad{
			CodigoCaeb:    rows[i].CodigoCaeb,
			Descripcion:   rows[i].Descripcion,
			TipoActividad: rows[i].TipoActividad,
		})
	}
	return out, nil
}

// PostgresSiatLeyendaRepository persiste las leyendas de factura sincronizadas
// del SIAT (tabla siat_leyendas_factura).
type PostgresSiatLeyendaRepository struct{ db *gorm.DB }

func NewPostgresSiatLeyendaRepository(db *gorm.DB) domain.SiatLeyendaRepository {
	return &PostgresSiatLeyendaRepository{db: db}
}

func (r *PostgresSiatLeyendaRepository) Replace(companyID string, leyendas []domain.SiatLeyenda, syncedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.SiatLeyendaFactura{}).Error; err != nil {
			return err
		}
		if len(leyendas) == 0 {
			return nil
		}
		rows := make([]models.SiatLeyendaFactura, 0, len(leyendas))
		for _, l := range leyendas {
			rows = append(rows, models.SiatLeyendaFactura{
				ID:                 uuid.NewString(),
				CompanyId:          companyID,
				CodigoActividad:    l.CodigoActividad,
				DescripcionLeyenda: l.DescripcionLeyenda,
				SyncedAt:           syncedAt,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (r *PostgresSiatLeyendaRepository) List(companyID string) ([]*domain.SiatLeyenda, error) {
	var rows []models.SiatLeyendaFactura
	if err := r.db.Where("company_id = ?", companyID).
		Order("codigo_actividad ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.SiatLeyenda, 0, len(rows))
	for i := range rows {
		out = append(out, &domain.SiatLeyenda{
			CodigoActividad:    rows[i].CodigoActividad,
			DescripcionLeyenda: rows[i].DescripcionLeyenda,
		})
	}
	return out, nil
}

func (r *PostgresSiatLeyendaRepository) ListByActividad(companyID, codigoActividad string) ([]*domain.SiatLeyenda, error) {
	var rows []models.SiatLeyendaFactura
	if err := r.db.Where("company_id = ? AND codigo_actividad = ?", companyID, codigoActividad).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.SiatLeyenda, 0, len(rows))
	for i := range rows {
		out = append(out, &domain.SiatLeyenda{
			CodigoActividad:    rows[i].CodigoActividad,
			DescripcionLeyenda: rows[i].DescripcionLeyenda,
		})
	}
	return out, nil
}

// PostgresSiatActividadDocSectorRepository persiste la relación actividad ↔
// documento-sector sincronizada del SIAT (tabla siat_actividades_doc_sector).
type PostgresSiatActividadDocSectorRepository struct{ db *gorm.DB }

func NewPostgresSiatActividadDocSectorRepository(db *gorm.DB) domain.SiatActividadDocSectorRepository {
	return &PostgresSiatActividadDocSectorRepository{db: db}
}

func (r *PostgresSiatActividadDocSectorRepository) Replace(companyID string, items []domain.SiatActividadDocSector, syncedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.SiatActividadDocSector{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		rows := make([]models.SiatActividadDocSector, 0, len(items))
		for _, it := range items {
			rows = append(rows, models.SiatActividadDocSector{
				ID:                    uuid.NewString(),
				CompanyId:             companyID,
				CodigoActividad:       it.CodigoActividad,
				CodigoDocumentoSector: it.CodigoDocumentoSector,
				TipoDocumentoSector:   it.TipoDocumentoSector,
				SyncedAt:              syncedAt,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (r *PostgresSiatActividadDocSectorRepository) List(companyID string) ([]*domain.SiatActividadDocSector, error) {
	var rows []models.SiatActividadDocSector
	if err := r.db.Where("company_id = ?", companyID).
		Order("codigo_actividad ASC, codigo_documento_sector ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return toDomainDocSectores(rows), nil
}

func (r *PostgresSiatActividadDocSectorRepository) ListByActividad(companyID, codigoActividad string) ([]*domain.SiatActividadDocSector, error) {
	var rows []models.SiatActividadDocSector
	if err := r.db.Where("company_id = ? AND codigo_actividad = ?", companyID, codigoActividad).
		Order("codigo_documento_sector ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return toDomainDocSectores(rows), nil
}

func toDomainDocSectores(rows []models.SiatActividadDocSector) []*domain.SiatActividadDocSector {
	out := make([]*domain.SiatActividadDocSector, 0, len(rows))
	for i := range rows {
		out = append(out, &domain.SiatActividadDocSector{
			CodigoActividad:       rows[i].CodigoActividad,
			CodigoDocumentoSector: rows[i].CodigoDocumentoSector,
			TipoDocumentoSector:   rows[i].TipoDocumentoSector,
		})
	}
	return out
}
