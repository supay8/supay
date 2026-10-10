package usecase

import (
	"fmt"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"sort"
	"strconv"
	"strings"
)

func (uc *SiatUsecase) batchEvent(posID string) (*domain.ContingencyEvent, error) {
	if uc.contingencyRepo == nil {
		return nil, domain.NewConflictError("No hay un evento de contingencia registrado")
	}
	event, err := uc.contingencyRepo.GetLatestByPointOfSale(posID)
	if err != nil {
		return nil, fmt.Errorf("obtener evento de contingencia: %w", err)
	}
	if event == nil || event.PointOfSaleID != posID || !event.IsSynced || event.EndDate == nil || event.SiatEventCode == nil {
		return nil, domain.NewConflictError("Registre y cierre el evento de contingencia antes de enviar sus facturas")
	}
	code, err := strconv.ParseInt(*event.SiatEventCode, 10, 64)
	if err != nil || code <= 0 || !event.EndDate.After(event.StartDate) {
		return nil, domain.NewConflictError("El evento de contingencia guardado no tiene una recepción o intervalo válido")
	}
	return event, nil
}

func (uc *SiatUsecase) selectBatchInvoices(repo ports.FiscalBatchRepository, companyID, posID string, ids []string, status domain.InvoiceStatus, event *domain.ContingencyEvent) ([]*domain.Invoice, error) {
	var invoices []*domain.Invoice
	var err error
	if ids == nil {
		var eventID *string
		if event != nil {
			eventID = &event.ID
		}
		invoices, err = repo.ListPendingBatchInvoices(companyID, posID, status, eventID)
	} else {
		if len(ids) == 0 {
			return nil, domain.NewBadRequestError("invoice_ids debe contener al menos una factura cuando se proporciona")
		}
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			if strings.TrimSpace(id) == "" || seen[id] {
				return nil, domain.NewBadRequestError("invoice_ids contiene identificadores vacíos o duplicados")
			}
			seen[id] = true
		}
		invoices, err = uc.invoiceRepo.GetByIDs(companyID, ids)
		if err == nil && len(invoices) != len(ids) {
			return nil, domain.NewNotFoundError("No se encontraron todas las facturas seleccionadas")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("obtener facturas del lote: %w", err)
	}
	for _, inv := range invoices {
		if inv == nil || inv.CompanyId != companyID || inv.PointOfSaleId != posID {
			return nil, domain.NewNotFoundError("La selección contiene facturas ajenas al punto de venta")
		}
		if inv.Status != status || (inv.SiatReceptionCode != nil && *inv.SiatReceptionCode != "") {
			return nil, domain.NewConflictError("La factura " + inv.ID + " ya fue enviada o no está pendiente para este tipo de lote")
		}
		if event != nil {
			if inv.ContingencyEventId == nil || *inv.ContingencyEventId != event.ID {
				return nil, domain.NewConflictError("La factura " + inv.ID + " no pertenece al evento de contingencia")
			}
			if inv.IssueDate.Before(event.StartDate) || inv.IssueDate.After(*event.EndDate) {
				return nil, domain.NewConflictError("La fecha de la factura " + inv.ID + " está fuera del evento de contingencia")
			}
		} else {
			if inv.ContingencyEventId != nil || inv.EmissionType == "OFFLINE" || inv.EmissionType == "MASIVA" || (inv.Cuf != nil && *inv.Cuf != "") {
				return nil, domain.NewConflictError("La factura " + inv.ID + " ya tiene una identidad fiscal o pertenece a una contingencia; no puede emitirse como masiva")
			}
		}
	}
	// Orden estable para dividir lotes y para asociar CUFs con las facturas.
	sort.Slice(invoices, func(i, j int) bool {
		if invoices[i].InvoiceNumber != invoices[j].InvoiceNumber {
			return invoices[i].InvoiceNumber < invoices[j].InvoiceNumber
		}
		return invoices[i].ID < invoices[j].ID
	})
	return invoices, nil
}
