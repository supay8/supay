package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
)

// VerifyStatus consulta al SIAT el estado real de un documento emitido
// (verificacionEstadoFactura) y reconcilia el estado local de la factura con el
// CodigoEstado devuelto (según el catálogo mensajesServicios). Un error de
// transporte no modifica el estado local.
func (uc *InvoiceUsecase) VerifyStatus(ctx context.Context, id string) (*domain.Invoice, error) {
	tenantID, ok := fiscal.CompanyIDFromContext(ctx)
	if !ok {
		return nil, domain.ErrMissingCompanyID
	}
	inv, err := uc.invoiceRepo.GetByID(tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, domain.NewConflictError("la factura no ha sido emitida (no tiene cuf asignado)")
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		return nil, err
	}
	result, err := svc.VerifyStatus(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("error de verificación: %w", err)
	}

	if estado, ok := siatEstadoToDomain(result.CodigoEstado); ok && inv.Status != estado {
		event := invoiceTransitionEvent(inv, inv.Status, estado, domain.TransitionSIATReconciliation, map[string]any{"source": "VerifyStatus", "codigo_estado": result.CodigoEstado})
		claimed, err := uc.invoiceRepo.TransitionStatus(tenantID, inv.ID, inv.Status, estado, domain.TransitionSIATReconciliation, nil, event)
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, domain.NewConflictError("la factura cambió de estado durante la reconciliación SIAT")
		}
		inv.Status = estado
	}

	return inv, nil
}

// Annul anula ante el SIAT una factura emitida (anulacionFactura) con el motivo
// del catálogo sincronizado motivoAnulacion. Al ser aceptada (905 ANULACION
// CONFIRMADA) se persiste el estado, el motivo y la fecha de anulación.
func (uc *InvoiceUsecase) Annul(ctx context.Context, id string, codigoMotivo int) (*domain.Invoice, error) {
	tenantID, ok := fiscal.CompanyIDFromContext(ctx)
	if !ok {
		return nil, domain.ErrMissingCompanyID
	}
	inv, err := uc.invoiceRepo.GetByID(tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}

	if inv.Status != domain.InvoiceAccepted {
		return nil, domain.NewConflictError("solo se pueden anular facturas en estado ACCEPTED")
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, domain.NewConflictError("la factura no tiene cuf asignado")
	}
	if err := uc.validateMotivoAnulacion(inv.CompanyId, codigoMotivo); err != nil {
		return nil, err
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}

	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		return nil, err
	}
	result, err := svc.Annul(ctx, *req, codigoMotivo)
	if err != nil {
		return nil, fmt.Errorf("error de anulación: %w", err)
	}

	if !result.Transaccion {
		return nil, &EmissionRejectedError{
			CodigoEstado:    result.CodigoEstado,
			CodigoRecepcion: result.CodigoRecepcion,
			Mensajes:        result.Mensajes,
		}
	}

	now := time.Now()
	fields := map[string]any{
		"motivo_anulacion": codigoMotivo,
		"fecha_anulacion":  now,
	}
	if result.CodigoRecepcion != "" {
		fields["siat_reception_code"] = result.CodigoRecepcion
	}
	// Transición condicional ACCEPTED->CANCELLED: si otra anulación concurrente
	// ya la aplicó, aquí llega false en lugar de pisar el estado.
	event := invoiceTransitionEvent(inv, domain.InvoiceAccepted, domain.InvoiceCancelled, domain.TransitionCancellation, map[string]any{"source": "Annul", "codigo_motivo": codigoMotivo})
	claimed, err := uc.invoiceRepo.TransitionStatus(tenantID, id, domain.InvoiceAccepted, domain.InvoiceCancelled, domain.TransitionCancellation, fields, event)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, domain.NewConflictError("la factura ya no está en estado ACCEPTED (posible anulación concurrente)")
	}

	inv.Status = domain.InvoiceCancelled
	inv.MotivoAnulacion = &codigoMotivo
	inv.FechaAnulacion = &now
	if code, ok := fields["siat_reception_code"]; ok {
		codeStr := code.(string)
		inv.SiatReceptionCode = &codeStr
	}

	return inv, nil
}

// RevertAnnul revierte una anulación aceptada por el SIAT
// (reversionAnulacionFactura; 907 REVERSION DE ANULACION CONFIRMADA),
// devolviendo la factura a ACCEPTED y limpiando el motivo y la fecha de
// anulación persistidos.
func (uc *InvoiceUsecase) RevertAnnul(ctx context.Context, id string) (*domain.Invoice, error) {
	tenantID, ok := fiscal.CompanyIDFromContext(ctx)
	if !ok {
		return nil, domain.ErrMissingCompanyID
	}
	inv, err := uc.invoiceRepo.GetByID(tenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}
	if inv.Status != domain.InvoiceCancelled {
		return nil, domain.NewConflictError("solo se pueden revertir anulaciones de facturas en estado CANCELLED")
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, domain.NewConflictError("la factura no tiene cuf asignado")
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		return nil, err
	}
	result, err := svc.RevertAnnul(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("error de reversión de anulación: %w", err)
	}

	if !result.Transaccion {
		return nil, &EmissionRejectedError{
			CodigoEstado:    result.CodigoEstado,
			CodigoRecepcion: result.CodigoRecepcion,
			Mensajes:        result.Mensajes,
		}
	}

	fields := map[string]any{
		"motivo_anulacion": nil,
		"fecha_anulacion":  nil,
	}
	if result.CodigoRecepcion != "" {
		fields["siat_reception_code"] = result.CodigoRecepcion
	}
	// Transición condicional CANCELLED->ACCEPTED (simétrica a Annul).
	event := invoiceTransitionEvent(inv, domain.InvoiceCancelled, domain.InvoiceAccepted, domain.TransitionCancellationRevert, map[string]any{"source": "RevertAnnul"})
	claimed, err := uc.invoiceRepo.TransitionStatus(tenantID, id, domain.InvoiceCancelled, domain.InvoiceAccepted, domain.TransitionCancellationRevert, fields, event)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, domain.NewConflictError("la factura ya no está en estado CANCELLED (posible reversión concurrente)")
	}

	inv.Status = domain.InvoiceAccepted
	inv.MotivoAnulacion = nil
	inv.FechaAnulacion = nil
	if code, ok := fields["siat_reception_code"]; ok {
		codeStr := code.(string)
		inv.SiatReceptionCode = &codeStr
	}

	return inv, nil
}

// validateMotivoAnulacion valida que el motivo exista en el catálogo
// sincronizado motivoAnulacion del SIAT.
func (uc *InvoiceUsecase) validateMotivoAnulacion(companyID string, codigoMotivo int) error {
	if uc.catalogRepo == nil {
		return nil
	}
	items, err := uc.catalogRepo.List(companyID, "motivoAnulacion")
	if err != nil {
		return fmt.Errorf("no se pudo consultar el catálogo de motivos de anulación: %w", err)
	}
	for _, item := range items {
		if item.Codigo == codigoMotivo {
			return nil
		}
	}
	return domain.NewBadRequestError(fmt.Sprintf("motivo de anulación %d no es válido; consulte el catálogo motivoAnulacion", codigoMotivo))
}

// siatEstadoToDomain mapea el CodigoEstado de una respuesta de verificación del
// SIAT al estado de dominio local, según el catálogo mensajesServicios:
// 902 RECEPCION RECHAZADA, 904 RECEPCION OBSERVADA, 905 ANULACION CONFIRMADA,
// 907 REVERSION DE ANULACION CONFIRMADA, 908 RECEPCION VALIDADA.
func siatEstadoToDomain(codigoEstado int) (domain.InvoiceStatus, bool) {
	switch codigoEstado {
	case 908, 907:
		return domain.InvoiceAccepted, true
	case 904:
		return domain.InvoiceObserved, true
	case 902, 906, 909:
		return domain.InvoiceRejected, true
	case 905:
		return domain.InvoiceCancelled, true
	default:
		return "", false
	}
}
