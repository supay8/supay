package domain

import (
	"errors"
	"testing"
)

func TestInvoiceStateMachineAllowsExpectedTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from, to InvoiceStatus
		reason   InvoiceTransitionReason
	}{
		{"start emission", InvoicePending, InvoiceSending, TransitionEmissionStart},
		{"transport recovery", InvoiceSending, InvoicePending, TransitionTransportFailure},
		{"stale recovery", InvoiceSending, InvoicePending, TransitionStaleRecovery},
		{"accepted", InvoiceSending, InvoiceAccepted, TransitionSIATAccepted},
		{"observed", InvoiceSending, InvoiceObserved, TransitionSIATObserved},
		{"rejected", InvoiceSending, InvoiceRejected, TransitionSIATRejected},
		{"observed accepted", InvoiceObserved, InvoiceAccepted, TransitionSIATAccepted},
		{"observed rejected", InvoiceObserved, InvoiceRejected, TransitionSIATRejected},
		{"cancel", InvoiceAccepted, InvoiceCancelled, TransitionCancellation},
		{"revert cancel", InvoiceCancelled, InvoiceAccepted, TransitionCancellationRevert},
	}

	machine := InvoiceStateMachine{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := machine.Transition(test.from, test.to, test.reason); err != nil {
				t.Fatalf("expected transition to be allowed: %v", err)
			}
		})
	}
}

func TestInvoiceStateMachineRejectsInvalidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from, to InvoiceStatus
		reason   InvoiceTransitionReason
		want     error
	}{
		{"rejected is terminal", InvoiceRejected, InvoiceAccepted, TransitionSIATReconciliation, ErrInvalidInvoiceTransition},
		{"cancel requires explicit reason", InvoiceAccepted, InvoiceCancelled, TransitionSIATAccepted, ErrInvalidInvoiceTransition},
		{"missing reason", InvoicePending, InvoiceSending, "", ErrInvalidTransitionReason},
		{"unknown source", InvoiceStatus("UNKNOWN"), InvoiceSending, TransitionEmissionStart, ErrInvalidInvoiceStatus},
	}

	machine := InvoiceStateMachine{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := machine.Transition(test.from, test.to, test.reason)
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}
