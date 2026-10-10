package usecase

import (
	"log/slog"

	"github.com/brandsrx/supay/internal/domain"
)

// persistSentPackage guarda el registro de auditoría del envío al SIAT. Un
// fallo no revierte el envío pero sí se loguea: perder la trazabilidad fiscal
// (codigoRecepcion ↔ facturas) sin rastro es inaceptable.
func (uc *SiatUsecase) persistSentPackage(company *domain.Company, pos *domain.PointOfSale, codigoRecepcion string, tipo domain.SentPackageType, docSector, codigoEmision, cantidad int, contingencyEventId ...*string) {
	if uc.sentPackageRepo == nil || codigoRecepcion == "" {
		return
	}
	pkg := &domain.SentPackage{
		CompanyId:             company.ID,
		PointOfSaleId:         pos.ID,
		CodigoRecepcion:       codigoRecepcion,
		Type:                  tipo,
		CodigoDocumentoSector: docSector,
		CodigoEmision:         codigoEmision,
		CantidadFacturas:      cantidad,
		Status:                domain.PackageStatusPending,
	}
	if len(contingencyEventId) > 0 && contingencyEventId[0] != nil {
		pkg.ContingencyEventId = contingencyEventId[0]
	}
	if err := uc.sentPackageRepo.Create(pkg); err != nil {
		slog.Error("no se pudo persistir el registro de paquete enviado",
			"codigo_recepcion", codigoRecepcion, "pos_id", pos.ID, "tipo", tipo, "error", err)
	}
}
