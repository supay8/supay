package domain

import "fmt"

type InvoiceTransitionReason string

const (
	TransitionEmissionStart      InvoiceTransitionReason = "EMISSION_START"
	TransitionBatchReserved      InvoiceTransitionReason = "BATCH_RESERVED"
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

type invoiceTransition struct{ from, to InvoiceStatus }

// This table is private and read-only after initialization. Reasons are part
// of the state machine, so recovery cannot bypass cancellation/emission rules.
var allowedTransitions = map[invoiceTransition]map[InvoiceTransitionReason]struct{}{
	{InvoicePending, InvoiceSending}:    {TransitionEmissionStart: {}},
	{InvoiceSending, InvoicePending}:    {TransitionTransportFailure: {}, TransitionStaleRecovery: {}},
	{InvoiceSending, InvoiceOffline}:    {TransitionContingency: {}},
	{InvoiceSending, InvoiceSent}:       {TransitionBatchReserved: {}},
	{InvoiceOffline, InvoiceSent}:       {TransitionBatchReserved: {}},
	{InvoiceSending, InvoiceAccepted}:   {TransitionSIATAccepted: {}, TransitionSIATReconciliation: {}},
	{InvoiceSending, InvoiceObserved}:   {TransitionSIATObserved: {}, TransitionSIATReconciliation: {}},
	{InvoiceSending, InvoiceRejected}:   {TransitionSIATRejected: {}, TransitionSIATReconciliation: {}},
	{InvoiceObserved, InvoiceAccepted}:  {TransitionSIATAccepted: {}, TransitionSIATReconciliation: {}},
	{InvoiceObserved, InvoiceRejected}:  {TransitionSIATRejected: {}, TransitionSIATReconciliation: {}},
	{InvoiceSent, InvoiceAccepted}:      {TransitionSIATAccepted: {}, TransitionSIATReconciliation: {}},
	{InvoiceSent, InvoiceObserved}:      {TransitionSIATObserved: {}, TransitionSIATReconciliation: {}},
	{InvoiceSent, InvoiceRejected}:      {TransitionSIATRejected: {}, TransitionSIATReconciliation: {}},
	{InvoiceAccepted, InvoiceCancelled}: {TransitionCancellation: {}, TransitionSIATReconciliation: {}},
	{InvoiceCancelled, InvoiceAccepted}: {TransitionCancellationRevert: {}, TransitionSIATReconciliation: {}},
}

func CanTransition(from, to InvoiceStatus, reason InvoiceTransitionReason) bool {
	_, ok := allowedTransitions[invoiceTransition{from, to}][reason]
	return ok
}
func transitionAllowed(from, to InvoiceStatus, reason InvoiceTransitionReason) bool {
	return CanTransition(from, to, reason)
}
