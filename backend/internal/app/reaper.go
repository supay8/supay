package app

import (
	"context"
	"log/slog"
	"time"
)

type staleEmissionRepository interface {
	ReleaseStaleSending(olderThan time.Duration) (int64, error)
}

// StartStaleEmissionReaper libera periódicamente las facturas atascadas en
// SENDING (p.ej. crash del proceso entre el claim y el update final),
// devolviéndolas a PENDING para que puedan reemitirse.
func StartStaleEmissionReaper(parent context.Context, repo staleEmissionRepository) func() {
	return startStaleEmissionReaper(parent, repo, 5*time.Minute, 10*time.Minute)
}

func startStaleEmissionReaper(parent context.Context, repo staleEmissionRepository, interval, staleAfter time.Duration) func() {
	if repo == nil {
		return func() {}
	}
	if parent == nil {
		parent = context.Background()
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if staleAfter <= 0 {
		staleAfter = 10 * time.Minute
	}
	ctx, cancel := context.WithCancel(parent)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				released, err := repo.ReleaseStaleSending(staleAfter)
				if err != nil {
					slog.Error("reaper: no se pudo liberar facturas SENDING atascadas", "error", err)
					continue
				}
				if released > 0 {
					slog.Warn("reaper: facturas SENDING atascadas liberadas a PENDING", "cantidad", released, "stale_after", staleAfter.String())
				}
			}
		}
	}()
	return cancel
}
