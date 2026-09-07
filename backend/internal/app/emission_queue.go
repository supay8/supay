package app

import (
	"context"
	"time"

	"github.com/brandsrx/supay/internal/emissionqueue"
)

// StartEmissionQueue starts River and the outbox dispatcher and returns a
// graceful shutdown function. Disabled deployments keep the legacy synchronous
// usecase path for local tooling.
func StartEmissionQueue(parent context.Context, queue *emissionqueue.Service, enabled bool, stopTimeout time.Duration) (func(), error) {
	if !enabled || queue == nil {
		return func() {}, nil
	}
	if err := queue.Start(parent); err != nil {
		return nil, err
	}
	return func() {
		if stopTimeout <= 0 {
			stopTimeout = 30 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		_ = queue.Stop(ctx)
	}, nil
}
