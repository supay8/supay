package postgres

import (
	"sort"
	"strconv"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type PostgresTipoPuntoVentaRepository struct{ db *gorm.DB }

func NewPostgresTipoPuntoVentaRepository(db *gorm.DB) domain.TipoPuntoVentaRepository {
	return &PostgresTipoPuntoVentaRepository{db: db}
}

func (r *PostgresTipoPuntoVentaRepository) Replace(companyID string, tipos []domain.TipoPuntoVenta, syncedAt time.Time) error {
	items := make([]versionedCatalogItem, 0, len(tipos))
	for _, tipo := range tipos {
		items = append(items, versionedCatalogItem{Code: strconv.Itoa(tipo.CodigoClasificador), Description: tipo.Descripcion})
	}
	return replaceVersionedCatalog(r.db, companyID, catalogTipoPuntoVenta, syncedAt, items)
}

func (r *PostgresTipoPuntoVentaRepository) List(companyID string) ([]*domain.TipoPuntoVenta, error) {
	items, version, err := latestCatalogItems(r.db, companyID, catalogTipoPuntoVenta)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.TipoPuntoVenta, 0, len(items))
	for _, item := range items {
		code, err := strconv.Atoi(item.Codigo)
		if err != nil {
			return nil, err
		}
		row := &domain.TipoPuntoVenta{CompanyID: companyID, CodigoClasificador: code, Descripcion: item.Descripcion}
		if version != nil {
			row.SyncedAt, row.CreatedAt = version.SyncedAt, version.CreatedAt
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CodigoClasificador < result[j].CodigoClasificador
	})
	return result, nil
}

func (r *PostgresTipoPuntoVentaRepository) FindByClasificador(companyID string, codigoClasificador int) (*domain.TipoPuntoVenta, error) {
	items, err := r.List(companyID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.CodigoClasificador == codigoClasificador {
			return item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
