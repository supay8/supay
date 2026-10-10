package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

// EmissionRejectedError indica que el SIAT respondió y rechazó la factura
// (Transaccion=false). La factura queda persistida como REJECTED.
type EmissionRejectedError struct {
	CodigoEstado    int
	CodigoRecepcion string
	Mensajes        []ports.FiscalMessage
}

// EmissionInProgressError maps concurrent emission attempts to HTTP 409.
type EmissionInProgressError struct{}

func (e *EmissionInProgressError) Error() string { return "la factura ya está en proceso de emisión" }

func (e *EmissionRejectedError) Error() string {
	var parts []string
	for _, m := range e.Mensajes {
		parts = append(parts, fmt.Sprintf("[%d] %s", m.Codigo, m.Descripcion))
	}
	if len(parts) == 0 {
		return "la factura fue rechazada por el SIAT"
	}
	return "la factura fue rechazada por el SIAT: " + strings.Join(parts, "; ")
}

func (e *EmissionInProgressError) Unwrap() error { return domain.NewConflictError(e.Error()) }

// Emit runs the complete emission inside the request. A normal POS sale must
// receive the SIAT result (or its offline contingency document) immediately;
// it must not wait for an internal retry queue.
func (uc *InvoiceUsecase) Emit(ctx context.Context, id string) (*domain.Invoice, error) {
	tenantID, ok := fiscal.CompanyIDFromContext(ctx)
	if !ok {
		return nil, domain.ErrMissingCompanyID
	}
	return uc.ProcessEmission(ctx, tenantID, id)
}

// ProcessEmission emite al SIAT una factura en estado PENDING usando el SDK go-siat.
// El flujo cubre los prerrequisitos (CUIS/CUFD vigentes, sucursal/PV, cliente e
// ítems mapeados a catálogos SIN), la construcción con builders del SDK, la
// firma digital automática (modalidad electrónica) y la persistencia del
// resultado (CUF, XML, hash, código de recepción y estado).
func (uc *InvoiceUsecase) ProcessEmission(ctx context.Context, tenantID, id string) (*domain.Invoice, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, domain.ErrMissingCompanyID
	}
	inv, err := uc.invoiceRepo.GetByID(tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}
	if inv.CodigoDocumentoSector == 30 {
		return nil, domain.NewBadRequestError("el sector 30 requiere emisión masiva; use /v1/siat/masiva/{companyId}/{pointOfSaleId}")
	}
	if inv.Status != domain.InvoicePending {
		if inv.Status == domain.InvoiceSending {
			return nil, &EmissionInProgressError{}
		}
		return nil, domain.NewConflictError("solo se pueden emitir facturas en estado PENDING")
	}

	// La factura permanece PENDING mientras obtiene credenciales para no ocupar
	// SENDING con un documento que todavia no tiene CUIS/CUFD utilizable.
	if err := uc.validateEmissionPayload(inv); err != nil {
		return nil, err
	}
	if inv.Archivo == "" {
		if err := uc.prepareCredentialsForEmission(ctx, inv); err != nil {
			return nil, err
		}
	}
	// Claim atómico PENDING->SENDING: evita emisiones duplicadas concurrentes.
	claimed, err := uc.invoiceRepo.ClaimForEmission(tenantID, id)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, &EmissionInProgressError{}
	}

	// Los errores que no son de conectividad revierten a PENDING. Un timeout o
	// una caida de red se resuelve mediante la contingencia oficial mas abajo.
	rollback := func() {
		event := invoiceTransitionEvent(inv, domain.InvoiceSending, domain.InvoicePending, domain.TransitionTransportFailure, map[string]any{"source": "Emit"})
		if _, err := uc.invoiceRepo.TransitionStatus(tenantID, inv.ID, domain.InvoiceSending, domain.InvoicePending, domain.TransitionTransportFailure, nil, event); err != nil {
			slog.Error("emisión: no se pudo revertir la factura a PENDING",
				"invoice_id", inv.ID, "error", err)
		}
		inv.Status = domain.InvoicePending
	}

	req, err := uc.buildSolicitudFactura(ctx, inv)
	if err != nil {
		rollback()
		return nil, err
	}
	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		rollback()
		return nil, err
	}
	result, err := uc.dispatchInvoiceEmission(ctx, inv, svc, *req)
	if err != nil {
		if isSIATConnectivityError(err) {
			offline, contingencyErr := uc.processOfflineContingency(ctx, inv, svc, *req)
			if contingencyErr == nil {
				return offline, nil
			}
			slog.Error("emisión: no se pudo activar contingencia offline",
				"invoice_id", inv.ID, "siat_error", err, "contingency_error", contingencyErr)
		}
		rollback()
		return nil, fmt.Errorf("error de emisión: %w", err)
	}
	applyEmissionArtifacts(inv, result)
	if result.CodigoRecepcion != "" {
		inv.SiatReceptionCode = &result.CodigoRecepcion
	}
	if msgs, err := marshalMensajes(result.Mensajes); err == nil {
		inv.SiatMensajes = &msgs
	}
	if err := uc.persistSignedXML(ctx, inv); err != nil {
		slog.Error("signed XML object persistence failed; invoice remains SENDING for retry", "invoice_id", inv.ID, "error", err)
		return nil, fmt.Errorf("signed XML persistence: %w", err)
	}
	status, transitionReason := emissionOutcome(result)
	inv.Status = status

	if err := uc.persistResultadoConReintentos(inv, result, transitionReason); err != nil {
		return nil, err
	}
	if !result.Transaccion {
		logSIATRejection(inv.CodigoDocumentoSector, result.CodigoEstado, result.Mensajes)
		return nil, &EmissionRejectedError{
			CodigoEstado:    result.CodigoEstado,
			CodigoRecepcion: result.CodigoRecepcion,
			Mensajes:        result.Mensajes,
		}
	}

	// El PDF forma parte del resultado sincrono que consume el POS.
	if uc.pdfService != nil && result.Transaccion {
		if strict, ok := uc.pdfService.(ports.StrictPdfGenerator); ok {
			if err := strict.GenerateAndPersistStrict(ctx, inv.ID); err != nil {
				slog.Error("PDF persistence failed after SIAT acceptance; reconcile", "invoice_id", inv.ID, "error", err)
				return nil, err
			}
		} else {
			uc.pdfService.GenerateAndPersist(ctx, inv.ID)
		}
	}
	if uc.emailDispatcher != nil {
		if err := uc.emailDispatcher.DispatchOnce(ctx); err != nil {
			slog.Warn("email de factura quedó pendiente de republicación", "invoice_id", inv.ID, "error", err)
		}
	}

	return inv, nil
}

var _ ports.InvoiceEmitter = (*InvoiceUsecase)(nil)
var _ ports.InvoiceEmissionProcessor = (*InvoiceUsecase)(nil)
