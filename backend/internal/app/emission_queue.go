package app

import (
	"context"
	"sync"
	"time"
)

const maxEmissionQueueStopTimeout = 10 * time.Second

type emissionQueueLifecycle interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// StartEmissionQueue starts River and the outbox dispatcher and returns a
// graceful shutdown function. Disabled deployments keep the legacy synchronous
// usecase path for local tooling.
func StartEmissionQueue(parent context.Context, queue emissionQueueLifecycle, enabled bool, stopTimeout time.Duration) (func() error, error) {
	if !enabled || queue == nil {
		return func() error { return nil }, nil
	}
	if parent == nil {
		parent = context.Background()
	}
	// SIGTERM is coordinated by the caller through the returned stop function.
	// Detaching cancellation prevents River from interpreting the signal as an
	// immediate hard stop before it receives its graceful Stop call.
	if err := queue.Start(context.WithoutCancel(parent)); err != nil {
		return nil, err
	}
	if stopTimeout <= 0 || stopTimeout > maxEmissionQueueStopTimeout {
		stopTimeout = maxEmissionQueueStopTimeout
	}
	var once sync.Once
	var stopErr error
	return func() error {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
			defer cancel()
			stopErr = queue.Stop(ctx)
		})
		return stopErr
	}, nil
}
