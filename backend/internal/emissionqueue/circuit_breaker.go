package emissionqueue

import (
	"sync"
	"time"
)

type circuitState struct {
	failures    int
	openedUntil time.Time
	halfOpen    bool
}

// CircuitBreaker isolates SIAT failures by tenant. Once the cooldown expires,
// exactly one half-open probe is admitted.
type CircuitBreaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration
	now       func() time.Time
	states    map[string]circuitState
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{
		threshold: threshold, cooldown: cooldown, now: timeNow(), states: map[string]circuitState{},
	}
}

func (b *CircuitBreaker) Allow(tenantID string) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.states[tenantID]
	if state.openedUntil.IsZero() {
		return true, 0
	}
	now := b.now()
	if now.Before(state.openedUntil) {
		return false, state.openedUntil.Sub(now)
	}
	if state.halfOpen {
		return false, time.Second
	}
	state.halfOpen = true
	b.states[tenantID] = state
	return true, 0
}

func (b *CircuitBreaker) Success(tenantID string) {
	b.mu.Lock()
	delete(b.states, tenantID)
	b.mu.Unlock()
}

func (b *CircuitBreaker) Failure(tenantID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.states[tenantID]
	state.failures++
	if state.halfOpen || state.failures >= b.threshold {
		state.openedUntil = b.now().Add(b.cooldown)
		state.halfOpen = false
	}
	b.states[tenantID] = state
}

// AbortProbe releases a half-open probe that ended before reaching SIAT (for
// example, process shutdown or local validation). It neither closes the
// circuit nor counts the attempt as an SIAT failure.
func (b *CircuitBreaker) AbortProbe(tenantID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state, ok := b.states[tenantID]
	if !ok || !state.halfOpen {
		return
	}
	state.halfOpen = false
	b.states[tenantID] = state
}
