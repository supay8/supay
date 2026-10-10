package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"log/slog"
	"strings"
	"time"
)

func fiscalBatchRequest(pkg *domain.SentPackage, company *domain.Company, pos *domain.PointOfSale) ports.FiscalBulk {
	return ports.FiscalBulk{
		CodigoAmbiente: company.Ambiente.CodigoAmbiente(), Nit: company.Nit,
		Modalidad: pkg.Modalidad, CodigoSucursal: pos.CodigoSucursal, CodigoPuntoVenta: resolveCodigoPuntoVenta(pos),
		Cuis: pkg.Cuis, Cufd: pkg.Cufd, CodigoDocumentoSector: pkg.CodigoDocumentoSector,
		CodigoTipoFactura: pkg.CodigoTipoFactura, CodigoEmision: pkg.CodigoEmision, Layout: pkg.Layout,
	}
}

func packageRequest(bulk ports.FiscalBulk, eventCode *int64) ports.FiscalPackage {
	pkg := ports.FiscalPackage{
		CodigoAmbiente: bulk.CodigoAmbiente, Nit: bulk.Nit, Modalidad: bulk.Modalidad,
		CodigoSucursal: bulk.CodigoSucursal, CodigoPuntoVenta: bulk.CodigoPuntoVenta,
		Cuis: bulk.Cuis, Cufd: bulk.Cufd, CodigoControl: bulk.CodigoControl,
		CodigoDocumentoSector: bulk.CodigoDocumentoSector, CodigoTipoFactura: bulk.CodigoTipoFactura,
		CodigoEmision: bulk.CodigoEmision, Layout: bulk.Layout, Facturas: bulk.Facturas,
	}
	if eventCode != nil {
		pkg.CodigoEvento = *eventCode
	}
	return pkg
}

func (uc *SiatUsecase) ValidarMasiva(ctx context.Context, companyID, posID string, body PaqueteValidacionInput) (*PaqueteResultado, error) {
	return uc.validateInvoiceBatch(ctx, companyID, posID, body.BatchID, domain.PackageTypeMasiva)
}

func (uc *SiatUsecase) ValidarPaquete(ctx context.Context, companyID, posID string, body PaqueteValidacionInput) (*PaqueteResultado, error) {
	return uc.validateInvoiceBatch(ctx, companyID, posID, body.BatchID, domain.PackageTypePaquete)
}

func (uc *SiatUsecase) validateInvoiceBatch(ctx context.Context, companyID, posID, batchID string, kind domain.SentPackageType) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	repo, err := uc.batchRepository()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(batchID) == "" {
		return nil, domain.NewBadRequestError("El identificador del lote es obligatorio")
	}
	pkg, err := uc.sentPackageRepo.GetByID(batchID)
	if err != nil {
		return nil, err
	}
	if pkg == nil || pkg.CompanyId != companyID || pkg.Type != kind || (posID != "" && pkg.PointOfSaleId != posID) {
		return nil, domain.NewNotFoundError("Lote no encontrado")
	}
	if pkg.CodigoRecepcion == "" || pkg.CodigoDocumentoSector <= 0 || pkg.CodigoTipoFactura <= 0 || pkg.Modalidad <= 0 || pkg.Cufd == "" || pkg.Cuis == "" {
		return nil, domain.NewConflictError("El lote no tiene recepción o metadatos completos para validarlo")
	}
	company, pos, err := uc.LoadCompanyAndPointOfSale(companyID, pkg.PointOfSaleId)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveService(ctx, companyID)
	if err != nil {
		return nil, err
	}
	req := fiscalBatchRequest(pkg, company, pos)
	var result ports.FiscalPackageResult
	if kind == domain.PackageTypeMasiva {
		result, err = svc.ValidateBulk(ctx, req, pkg.CodigoRecepcion)
	} else {
		result, err = svc.ValidatePackage(ctx, packageRequest(req, pkg.CodigoEvento), pkg.CodigoRecepcion)
	}
	if err != nil {
		return nil, err
	}
	var invoiceStatus *domain.InvoiceStatus
	// Transaccion solo confirma la consulta. 908 confirma la validación fiscal;
	// una recepción observada puede contener facturas con resultados distintos.
	if result.Transaccion && result.CodigoEstado == 908 {
		pkg.Status = domain.PackageStatusAccepted
		accepted := domain.InvoiceAccepted
		invoiceStatus = &accepted
	} else if result.CodigoEstado == 902 && pkg.Status != domain.PackageStatusAccepted {
		logSIATRejection(pkg.CodigoDocumentoSector, result.CodigoEstado, result.Mensajes)
		pkg.Status = domain.PackageStatusRejected
		rejected := domain.InvoiceRejected
		invoiceStatus = &rejected
	}
	now := time.Now()
	pkg.ValidatedAt = &now
	messages, _ := json.Marshal(result.Mensajes)
	text := string(messages)
	pkg.Mensajes = &text
	if err := repo.UpdateBatch(pkg, invoiceStatus); err != nil {
		return nil, fmt.Errorf("guardar validación del lote %s: %w", pkg.ID, err)
	}
	if invoiceStatus != nil && (*invoiceStatus == domain.InvoiceAccepted || *invoiceStatus == domain.InvoiceObserved) && uc.emailDispatcher != nil {
		if err := uc.emailDispatcher.DispatchOnce(ctx); err != nil {
			slog.Warn("emails del lote quedaron pendientes de republicación", "batch_id", pkg.ID, "error", err)
		}
	}
	return &PaqueteResultado{Company: company, PointOfSale: pos, Response: &result,
		Batches: []BatchResultado{{BatchID: pkg.ID, InvoiceIDs: pkg.InvoiceIDs, Status: pkg.Status, Response: &result}}}, nil
}
