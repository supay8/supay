package postgres

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type PostgresSiatActividadRepository struct{ db *gorm.DB }

func NewPostgresSiatActividadRepository(db *gorm.DB) domain.SiatActividadRepository {
	return &PostgresSiatActividadRepository{db: db}
}

func (r *PostgresSiatActividadRepository) Replace(companyID string, items []domain.SiatActividad, syncedAt time.Time) error {
	rows := make([]versionedCatalogItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, versionedCatalogItem{
			Code: item.CodigoCaeb, Description: item.Descripcion,
			Metadata: map[string]any{"tipo_actividad": item.TipoActividad},
		})
	}
	return replaceVersionedCatalog(r.db, companyID, catalogActividades, syncedAt, rows)
}

func (r *PostgresSiatActividadRepository) List(companyID string) ([]*domain.SiatActividad, error) {
	items, _, err := latestCatalogItems(r.db, companyID, catalogActividades)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SiatActividad, 0, len(items))
	for _, item := range items {
		metadata, err := catalogMetadata(item)
		if err != nil {
			return nil, err
		}
		out = append(out, &domain.SiatActividad{
			CodigoCaeb: item.Codigo, Descripcion: item.Descripcion,
			TipoActividad: metadataString(metadata, "tipo_actividad"),
		})
	}
	return out, nil
}

type PostgresSiatLeyendaRepository struct{ db *gorm.DB }

func NewPostgresSiatLeyendaRepository(db *gorm.DB) domain.SiatLeyendaRepository {
	return &PostgresSiatLeyendaRepository{db: db}
}

func (r *PostgresSiatLeyendaRepository) Replace(companyID string, leyendas []domain.SiatLeyenda, syncedAt time.Time) error {
	items := make([]versionedCatalogItem, 0, len(leyendas))
	for _, legend := range leyendas {
		hash := sha256.Sum256([]byte(legend.DescripcionLeyenda))
		items = append(items, versionedCatalogItem{
			Code:        fmt.Sprintf("%s:%x", legend.CodigoActividad, hash),
			Description: legend.DescripcionLeyenda,
			Metadata:    map[string]any{"codigo_actividad": legend.CodigoActividad},
		})
	}
	return replaceVersionedCatalog(r.db, companyID, catalogLeyendasFactura, syncedAt, items)
}

func (r *PostgresSiatLeyendaRepository) List(companyID string) ([]*domain.SiatLeyenda, error) {
	return r.list(companyID, "")
}

func (r *PostgresSiatLeyendaRepository) ListByActividad(companyID, codigoActividad string) ([]*domain.SiatLeyenda, error) {
	return r.list(companyID, codigoActividad)
}

func (r *PostgresSiatLeyendaRepository) list(companyID, activity string) ([]*domain.SiatLeyenda, error) {
	items, _, err := latestCatalogItems(r.db, companyID, catalogLeyendasFactura)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SiatLeyenda, 0, len(items))
	for _, item := range items {
		metadata, err := catalogMetadata(item)
		if err != nil {
			return nil, err
		}
		code := metadataString(metadata, "codigo_actividad")
		if activity != "" && activity != code {
			continue
		}
		out = append(out, &domain.SiatLeyenda{CodigoActividad: code, DescripcionLeyenda: item.Descripcion})
	}
	return out, nil
}

type PostgresSiatActividadDocSectorRepository struct{ db *gorm.DB }

func NewPostgresSiatActividadDocSectorRepository(db *gorm.DB) domain.SiatActividadDocSectorRepository {
	return &PostgresSiatActividadDocSectorRepository{db: db}
}

func (r *PostgresSiatActividadDocSectorRepository) Replace(companyID string, items []domain.SiatActividadDocSector, syncedAt time.Time) error {
	rows := make([]versionedCatalogItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, versionedCatalogItem{
			Code:        fmt.Sprintf("%s:%d", item.CodigoActividad, item.CodigoDocumentoSector),
			Description: item.TipoDocumentoSector,
			Metadata: map[string]any{
				"codigo_actividad":        item.CodigoActividad,
				"codigo_documento_sector": item.CodigoDocumentoSector,
				"tipo_documento_sector":   item.TipoDocumentoSector,
			},
		})
	}
	return replaceVersionedCatalog(r.db, companyID, catalogActividadesDocSector, syncedAt, rows)
}

func (r *PostgresSiatActividadDocSectorRepository) List(companyID string) ([]*domain.SiatActividadDocSector, error) {
	return r.list(companyID, "")
}

func (r *PostgresSiatActividadDocSectorRepository) ListByActividad(companyID, codigoActividad string) ([]*domain.SiatActividadDocSector, error) {
	return r.list(companyID, codigoActividad)
}

func (r *PostgresSiatActividadDocSectorRepository) list(companyID, activity string) ([]*domain.SiatActividadDocSector, error) {
	items, _, err := latestCatalogItems(r.db, companyID, catalogActividadesDocSector)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SiatActividadDocSector, 0, len(items))
	for _, item := range items {
		metadata, err := catalogMetadata(item)
		if err != nil {
			return nil, err
		}
		code := metadataString(metadata, "codigo_actividad")
		if activity != "" && activity != code {
			continue
		}
		out = append(out, &domain.SiatActividadDocSector{
			CodigoActividad:       code,
			CodigoDocumentoSector: metadataInt(metadata, "codigo_documento_sector"),
			TipoDocumentoSector:   metadataString(metadata, "tipo_documento_sector"),
		})
	}
	return out, nil
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return value
}

func metadataInt(metadata map[string]any, key string) int {
	switch value := metadata[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}
