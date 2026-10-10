package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"log/slog"
	"strings"
)

// Los documentos fiscales se construyen exclusivamente a partir de facturas
// persistidas. El consumidor solo elige el POS y, opcionalmente, sus facturas.
func (uc *SiatUsecase) EnviarMasiva(ctx context.Context, companyID, posID string, body MasivaInput) (*PaqueteResultado, error) {
	return uc.sendInvoiceBatches(ctx, companyID, posID, body.FacturaIDs, domain.PackageTypeMasiva)
}

func (uc *SiatUsecase) EnviarPaquete(ctx context.Context, companyID, posID string, body PaqueteInput) (*PaqueteResultado, error) {
	return uc.sendInvoiceBatches(ctx, companyID, posID, body.FacturaIDs, domain.PackageTypePaquete)
}

type invoiceBatch struct {
	pkg       *domain.SentPackage
	documents []ports.FiscalDocument
	bulk      ports.FiscalBulk
	pack      ports.FiscalPackage
}

func (uc *SiatUsecase) batchRepository() (ports.FiscalBatchRepository, error) {
	repo, ok := uc.sentPackageRepo.(ports.FiscalBatchRepository)
	if !ok {
		return nil, domain.NewConflictError("La persistencia de lotes no está configurada")
	}
	return repo, nil
}

func (uc *SiatUsecase) sendInvoiceBatches(ctx context.Context, companyID, posID string, ids []string, kind domain.SentPackageType) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if uc.invoiceRepo == nil {
		return nil, domain.NewConflictError("El repositorio de facturas no está configurado")
	}
	repo, err := uc.batchRepository()
	if err != nil {
		return nil, err
	}
	company, pos, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	status := domain.InvoicePending
	var event *domain.ContingencyEvent
	if kind == domain.PackageTypePaquete {
		status = domain.InvoiceOffline
		event, err = uc.batchEvent(pos.ID)
		if err != nil {
			return nil, err
		}
	}
	invoices, err := uc.selectBatchInvoices(repo, companyID, posID, ids, status, event)
	if err != nil {
		return nil, err
	}
	out := &PaqueteResultado{Company: company, PointOfSale: pos, Batches: []BatchResultado{}, Response: &ports.FiscalPackageResult{Transaccion: true}}
	if len(invoices) == 0 {
		return out, nil
	}
	if uc.credentials == nil {
		return nil, domain.NewConflictError("El servicio de credenciales no está configurado")
	}
	if err := uc.credentials.EnsureCuis(ctx, company, pos); err != nil {
		return nil, err
	}
	current, err := uc.credentials.EnsureCufd(ctx, company, pos)
	if err != nil {
		return nil, err
	}
	batches, err := uc.prepareInvoiceBatches(ctx, company, pos, current, invoices, event, kind)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveService(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if err := prepareBatchPayloads(ctx, svc, batches, company, pos, current); err != nil {
		return nil, err
	}
	for _, batch := range batches {
		pkg := batch.pkg
		item := BatchResultado{InvoiceIDs: pkg.InvoiceIDs, Status: "NOT_SENT"}
		if err := ctx.Err(); err != nil {
			item.Error = err.Error()
			out.Batches = append(out.Batches, item)
			out.Response.Transaccion = false
			continue
		}
		createdFiles, err := uc.persistBatchDocuments(ctx, batch)
		if err != nil {
			item.Error = err.Error()
			out.Batches = append(out.Batches, item)
			out.Response.Transaccion = false
			continue
		}
		if err := repo.ReserveBatch(pkg, pkg.InvoiceIDs, status); err != nil {
			uc.cleanupBatchDocuments(createdFiles)
			item.Error = err.Error()
			out.Batches = append(out.Batches, item)
			out.Response.Transaccion = false
			continue
		}
		item.BatchID = pkg.ID
		var result ports.FiscalPackageResult
		if kind == domain.PackageTypeMasiva {
			result, err = svc.SendBulk(ctx, batch.bulk)
		} else {
			result, err = svc.SendPackage(ctx, batch.pack)
		}
		var invoiceStatus *domain.InvoiceStatus
		if err != nil {
			// Una respuesta perdida no demuestra que SIAT no recibió el lote.
			// Conservar la reserva evita una segunda emisión accidental.
			pkg.Status, item.Error = domain.PackageStatusUnknown, err.Error()
			pkg.Mensajes = &item.Error
			out.Response.Transaccion = false
		} else {
			item.Response = &result
			pkg.CodigoRecepcion = result.CodigoRecepcion
			messages, _ := json.Marshal(result.Mensajes)
			text := string(messages)
			pkg.Mensajes = &text
			if !result.Transaccion || result.CodigoEstado == 902 {
				logSIATRejection(pkg.CodigoDocumentoSector, result.CodigoEstado, result.Mensajes)
			}
			if result.Transaccion && result.CodigoRecepcion != "" {
				pkg.Status = domain.PackageStatusPending
				sent := domain.InvoiceSent
				if result.CodigoEstado == 908 {
					pkg.Status, sent = domain.PackageStatusAccepted, domain.InvoiceAccepted
				}
				invoiceStatus = &sent
				out.Response.CantidadFacturas += pkg.CantidadFacturas
			} else if result.CodigoEstado == 902 {
				pkg.Status = domain.PackageStatusRejected
				rejected := domain.InvoiceRejected
				invoiceStatus = &rejected
				out.Response.Transaccion = false
			} else {
				pkg.Status = domain.PackageStatusUnknown
				out.Response.Transaccion = false
			}
		}
		if persistErr := repo.UpdateBatch(pkg, invoiceStatus); persistErr != nil {
			item.Error = strings.TrimSpace(fmt.Sprintf("%s No se pudo guardar el resultado del lote %s: %v", item.Error, pkg.ID, persistErr))
			// El estado de SIAT puede conocerse, pero la BD conserva la reserva.
			pkg.Status = domain.PackageStatusUnknown
			out.Response.Transaccion = false
		} else if invoiceStatus != nil && (*invoiceStatus == domain.InvoiceAccepted || *invoiceStatus == domain.InvoiceObserved) && uc.emailDispatcher != nil {
			if dispatchErr := uc.emailDispatcher.DispatchOnce(ctx); dispatchErr != nil {
				slog.Warn("emails del lote quedaron pendientes de republicación", "batch_id", pkg.ID, "error", dispatchErr)
			}
		}
		item.Status = pkg.Status
		out.Batches = append(out.Batches, item)
	}
	return out, nil
}
