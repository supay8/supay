package app

import (
	"log/slog"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

// StartStaleEmissionReaper libera periódicamente las facturas atascadas en
// SENDING (p.ej. crash del proceso entre el claim y el update final),
// devolviéndolas a PENDING para que puedan reemitirse.
func StartStaleEmissionReaper(repo domain.InvoiceRepository) {
	go func() {
		interval := 5 * time.Minute
		staleAfter := 10 * time.Minute
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			released, err := repo.ReleaseStaleSending(staleAfter)
			if err != nil {
				slog.Error("reaper: no se pudo liberar facturas SENDING atascadas", "error", err)
				continue
			}
			if released > 0 {
				slog.Warn("reaper: facturas SENDING atascadas liberadas a PENDING", "cantidad", released, "stale_after", staleAfter.String())
			}
		}
	}()
}
