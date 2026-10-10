package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

func isSIATConnectivityError(err error) bool {
	var preparation *emissionPreparationError
	if err == nil || errors.Is(err, context.Canceled) || errors.As(err, &preparation) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		return true
	}
	// Algunos clientes SOAP pierden el tipo concreto al envolver el error de
	// transporte. Se limita el fallback a mensajes inequívocos de conectividad.
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection refused", "connection reset", "network is unreachable",
		"no such host", "i/o timeout", "tls handshake timeout",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (uc *InvoiceUsecase) processOfflineContingency(
	ctx context.Context,
	inv *domain.Invoice,
	svc ports.FiscalSingle,
	req ports.FiscalDocument,
) (*domain.Invoice, error) {
	preparer, ok := svc.(ports.OfflineFiscalService)
	if !ok {
		return nil, errors.New("el adaptador SIAT no soporta generación offline")
	}
	if uc.contingencyRepo == nil {
		return nil, errors.New("repositorio de contingencias no configurado")
	}

	if err := uc.discardOnlinePreparation(ctx, inv); err != nil {
		return nil, fmt.Errorf("retirar preparación online: %w", err)
	}

	// El contexto HTTP puede haber vencido precisamente por el timeout del
	// SIAT. La construccion, firma y persistencia offline son operaciones locales.
	offlineCtx := context.WithoutCancel(ctx)
	result, err := preparer.PrepareOffline(offlineCtx, req)
	if err != nil {
		return nil, fmt.Errorf("generar factura offline: %w", err)
	}

	event, err := uc.openConnectivityContingency(inv)
	if err != nil {
		return nil, fmt.Errorf("registrar contingencia local: %w", err)
	}

	if strings.TrimSpace(result.Cuf) != "" {
		inv.Cuf = &result.Cuf
	}
	if result.Xml != "" {
		inv.Xml = &result.Xml
	}
	if result.XmlHash != "" {
		inv.XmlHash = &result.XmlHash
	}
	inv.Archivo = result.Archivo
	inv.HashArchivo = result.XmlHash
	inv.ContingencyEventId = &event.ID
	inv.EmissionType = "OFFLINE"
	inv.Status = domain.InvoiceOffline
	if err := uc.persistSignedXML(offlineCtx, inv); err != nil {
		slog.Error("offline signed XML object persistence failed; invoice remains SENDING for retry", "invoice_id", inv.ID, "error", err)
		return nil, fmt.Errorf("persistir XML offline firmado: %w", err)
	}

	fields := map[string]any{
		"cuf": inv.Cuf, "xml_hash": inv.XmlHash,
		"archivo": inv.Archivo, "hash_archivo": inv.HashArchivo,
		"cufd_id": inv.CufdId, "contingency_event_id": event.ID,
		"emission_type": inv.EmissionType,
	}
	transitionEvent := invoiceTransitionEvent(inv, domain.InvoiceSending, domain.InvoiceOffline, domain.TransitionContingency, map[string]any{
		"source": "Emit", "contingency_event_id": event.ID, "trigger": "SIAT_CONNECTIVITY_FAILURE",
	})
	claimed, err := uc.invoiceRepo.TransitionStatus(
		inv.CompanyId, inv.ID, domain.InvoiceSending, domain.InvoiceOffline,
		domain.TransitionContingency, fields, transitionEvent,
	)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, domain.NewConflictError("la factura cambió de estado mientras se activaba la contingencia")
	}
	if uc.pdfService != nil {
		if strict, ok := uc.pdfService.(ports.StrictPdfGenerator); ok {
			if err := strict.GenerateAndPersistStrict(offlineCtx, inv.ID); err != nil {
				return nil, err
			}
		} else {
			uc.pdfService.GenerateAndPersist(offlineCtx, inv.ID)
		}
	}
	return inv, nil
}

func (uc *InvoiceUsecase) openConnectivityContingency(inv *domain.Invoice) (*domain.ContingencyEvent, error) {
	latest, err := uc.contingencyRepo.GetLatestByPointOfSale(inv.PointOfSaleId)
	if err == nil && latest != nil && !latest.IsSynced && latest.EndDate == nil {
		return latest, nil
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	description := "Caída de red detectada durante la emisión; pendiente de registro ante el SIAT"
	start := inv.IssueDate
	if start.IsZero() {
		start = time.Now().In(fiscal.LaPaz)
	}
	event := &domain.ContingencyEvent{
		PointOfSaleID: inv.PointOfSaleId,
		Reason:        "FALLA_CONEXION_INTERNET",
		Description:   &description,
		StartDate:     start,
		IsSynced:      false,
	}
	if err := uc.contingencyRepo.Create(event); err != nil {
		return nil, err
	}
	return event, nil
}
