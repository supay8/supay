package postgres

import (
	"context"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/ports"
)

var _ ports.EmissionPreparationStore = (*PostgresInvoiceRepository)(nil)

func (r *PostgresInvoiceRepository) StorePreparedEmission(ctx context.Context, inv *domain.Invoice) error {
	result := r.db.WithContext(ctx).Model(&models.Invoice{}).
		Where("tenant_id = ? AND id = ? AND status = ?", inv.CompanyId, inv.ID, string(domain.InvoiceSending)).
		Updates(map[string]any{"cuf": inv.Cuf, "xml_hash": inv.XmlHash, "archivo": inv.Archivo, "hash_archivo": inv.HashArchivo, "cufd_id": inv.CufdId})
	if result.Error != nil {
		return repositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return repositoryError(domain.NewConflictError("la factura perdió la reserva antes del envío"))
	}
	return nil
}
