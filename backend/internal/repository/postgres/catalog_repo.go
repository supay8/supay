package postgres

import (
	"sort"
	"strconv"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

type PostgresCatalogRepository struct{ db *gorm.DB }

func NewPostgresCatalogRepository(db *gorm.DB) domain.CatalogRepository {
	return &PostgresCatalogRepository{db: db}
}

func (r *PostgresCatalogRepository) Replace(companyID, tipo string, items []domain.CatalogItem, syncedAt time.Time) error {
	rows := make([]versionedCatalogItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, versionedCatalogItem{Code: strconv.Itoa(item.Codigo), Description: item.Descripcion})
	}
	return replaceVersionedCatalog(r.db, companyID, tipo, syncedAt, rows)
}

func (r *PostgresCatalogRepository) List(companyID, tipo string) ([]*domain.CatalogItem, error) {
	rows, _, err := latestCatalogItems(r.db, companyID, tipo)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CatalogItem, 0, len(rows))
	for _, row := range rows {
		code, err := strconv.Atoi(row.Codigo)
		if err != nil {
			return nil, err
		}
		result = append(result, &domain.CatalogItem{Codigo: code, Descripcion: row.Descripcion, Tipo: tipo})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Codigo < result[j].Codigo })
	return result, nil
}

func (r *PostgresCatalogRepository) ListAll(companyID string) (map[string][]*domain.CatalogItem, error) {
	type catalogRow struct {
		models.CatalogItem
		Tipo string
	}
	var rows []catalogRow
	err := r.db.Raw(`
		SELECT i.*, v.tipo
		FROM catalog_items i
		JOIN catalog_versions v ON v.id = i.version_id
		JOIN (
			SELECT tenant_id, tipo, MAX(version) AS version
			FROM catalog_versions
			WHERE tenant_id = ?
			  AND tipo NOT IN ('tipoPuntoVenta', 'actividades', 'leyendasFactura', 'actividadesDocumentoSector')
			GROUP BY tenant_id, tipo
		) latest ON latest.tenant_id = v.tenant_id
			AND latest.tipo = v.tipo AND latest.version = v.version
		ORDER BY v.tipo, i.codigo`, companyID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string][]*domain.CatalogItem)
	for _, row := range rows {
		code, err := strconv.Atoi(row.Codigo)
		if err != nil {
			continue
		}
		result[row.Tipo] = append(result[row.Tipo], &domain.CatalogItem{Codigo: code, Descripcion: row.Descripcion, Tipo: row.Tipo})
	}
	for catalogType := range result {
		sort.Slice(result[catalogType], func(i, j int) bool {
			return result[catalogType][i].Codigo < result[catalogType][j].Codigo
		})
	}
	return result, nil
}
