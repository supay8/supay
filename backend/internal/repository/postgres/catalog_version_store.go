package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	catalogTipoPuntoVenta       = "tipoPuntoVenta"
	catalogActividades          = "actividades"
	catalogLeyendasFactura      = "leyendasFactura"
	catalogActividadesDocSector = "actividadesDocumentoSector"
)

type versionedCatalogItem struct {
	Code        string
	Description string
	Metadata    map[string]any
}

// replaceVersionedCatalog appends an immutable catalog version. The advisory
// lock serializes version allocation for a tenant/type pair.
func replaceVersionedCatalog(db *gorm.DB, tenantID, catalogType string, syncedAt time.Time, items []versionedCatalogItem) error {
	return db.Transaction(func(tx *gorm.DB) error {
		lockKey := tenantID + "|catalog|" + catalogType
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", lockKey).Error; err != nil {
			return err
		}

		var nextVersion int
		if err := tx.Model(&models.CatalogVersion{}).
			Where("tenant_id = ? AND tipo = ?", tenantID, catalogType).
			Select("COALESCE(MAX(version), 0) + 1").Scan(&nextVersion).Error; err != nil {
			return err
		}
		version := models.CatalogVersion{
			ID: uuid.NewString(), TenantID: tenantID, Tipo: catalogType,
			Version: nextVersion, SyncedAt: syncedAt, Source: "SIAT",
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}

		rows := make([]models.CatalogItem, 0, len(items))
		seen := make(map[string]struct{}, len(items))
		for _, item := range items {
			if _, exists := seen[item.Code]; exists {
				continue
			}
			seen[item.Code] = struct{}{}
			if item.Metadata == nil {
				item.Metadata = map[string]any{}
			}
			metadata, err := json.Marshal(item.Metadata)
			if err != nil {
				return fmt.Errorf("serializar metadata de catálogo %s: %w", catalogType, err)
			}
			rows = append(rows, models.CatalogItem{
				ID: uuid.NewString(), VersionID: version.ID, Codigo: item.Code,
				Descripcion: item.Description, Metadata: datatypes.JSON(metadata),
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func latestCatalogItems(db *gorm.DB, tenantID, catalogType string) ([]models.CatalogItem, *models.CatalogVersion, error) {
	var version models.CatalogVersion
	if err := db.Where("tenant_id = ? AND tipo = ?", tenantID, catalogType).
		Order("version DESC").First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []models.CatalogItem{}, nil, nil
		}
		return nil, nil, err
	}
	var items []models.CatalogItem
	if err := db.Where("version_id = ?", version.ID).Order("codigo ASC").Find(&items).Error; err != nil {
		return nil, nil, err
	}
	return items, &version, nil
}

func catalogMetadata(item models.CatalogItem) (map[string]any, error) {
	metadata := make(map[string]any)
	if len(item.Metadata) == 0 {
		return metadata, nil
	}
	if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("metadata inválida para catalog_item %s: %w", item.ID, err)
	}
	return metadata, nil
}
