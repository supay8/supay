package domain

import "fmt"

type InvoiceTransitionReason string

const (
	TransitionEmissionStart      InvoiceTransitionReason = "EMISSION_START"
	TransitionSIATAccepted       InvoiceTransitionReason = "SIAT_ACCEPTED"
	TransitionSIATObserved       InvoiceTransitionReason = "SIAT_OBSERVED"
	TransitionSIATRejected       InvoiceTransitionReason = "SIAT_REJECTED"
	TransitionTransportFailure   InvoiceTransitionReason = "TRANSPORT_FAILURE"
	TransitionContingency        InvoiceTransitionReason = "CONTINGENCY"
	TransitionStaleRecovery      InvoiceTransitionReason = "STALE_RECOVERY"
	TransitionCancellation       InvoiceTransitionReason = "CANCELLATION"
	TransitionCancellationRevert InvoiceTransitionReason = "CANCELLATION_REVERT"
	TransitionSIATReconciliation InvoiceTransitionReason = "SIAT_RECONCILIATION"
)

var (
	ErrInvalidInvoiceStatus           = fmt.Errorf("estado de factura inválido")
	ErrInvalidInvoiceTransition       = fmt.Errorf("transición de estado de factura no permitida")
	ErrInvalidTransitionReason        = fmt.Errorf("motivo de transición de factura inválido")
	ErrStatusUpdateRequiresTransition = fmt.Errorf("el cambio de estado requiere una transición explícita")
)

// InvoiceStateMachine centraliza las transiciones permitidas por el dominio.
type InvoiceStateMachine struct{}

func (InvoiceStateMachine) Transition(from, to InvoiceStatus, reason InvoiceTransitionReason) error {
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("%w: %q -> %q", ErrInvalidInvoiceStatus, from, to)
	}
	if reason == "" {
		return fmt.Errorf("%w: %q -> %q", ErrInvalidTransitionReason, from, to)
	}
	if !transitionAllowed(from, to, reason) {
		return fmt.Errorf("%w: %q -> %q (%s)", ErrInvalidInvoiceTransition, from, to, reason)
	}
	return nil
}

func transitionAllowed(from, to InvoiceStatus, reason InvoiceTransitionReason) bool {
	switch {
	case from == InvoicePending && to == InvoiceSending:
		return reason == TransitionEmissionStart
	case from == InvoiceSending && to == InvoicePending:
		return reason == TransitionTransportFailure || reason == TransitionStaleRecovery
	case from == InvoiceSending && to == InvoiceOffline:
		return reason == TransitionContingency
	case from == InvoiceSending && to == InvoiceAccepted:
		return reason == TransitionSIATAccepted || reason == TransitionSIATReconciliation
	case from == InvoiceSending && to == InvoiceObserved:
		return reason == TransitionSIATObserved || reason == TransitionSIATReconciliation
	case from == InvoiceSending && to == InvoiceRejected:
		return reason == TransitionSIATRejected || reason == TransitionSIATReconciliation
	case from == InvoiceObserved && to == InvoiceAccepted:
		return reason == TransitionSIATAccepted || reason == TransitionSIATReconciliation
	case from == InvoiceObserved && to == InvoiceRejected:
		return reason == TransitionSIATRejected || reason == TransitionSIATReconciliation
	case from == InvoiceAccepted && to == InvoiceCancelled:
		return reason == TransitionCancellation || reason == TransitionSIATReconciliation
	case from == InvoiceCancelled && to == InvoiceAccepted:
		return reason == TransitionCancellationRevert || reason == TransitionSIATReconciliation
	default:
		return false
	}
}
