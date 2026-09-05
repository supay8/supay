package app

import (
	"context"
	"log/slog"
	"time"

	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/usecase"
)

type maintenanceRunner interface {
	RunCredentialRenewal(ctx context.Context) error
	RunCertificateCheck(ctx context.Context) error
}

// StartMaintenanceScheduler inicia el scheduler liviano de fase 8. Ejecuta
// ambos chequeos al arrancar y luego usa tickers independientes. Devuelve una
// función de cancelación para el graceful shutdown.
func StartMaintenanceScheduler(parent context.Context, runner maintenanceRunner, cfg appconfig.MaintenanceConfig) func() {
	if !cfg.Enabled || runner == nil {
		return func() {}
	}
	if cfg.CredentialInterval <= 0 {
		cfg.CredentialInterval = time.Hour
	}
	if cfg.CertificateInterval <= 0 {
		cfg.CertificateInterval = 24 * time.Hour
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 5 * time.Minute
	}
	ctx, cancel := context.WithCancel(parent)
	go func() {
		credentialTicker := time.NewTicker(cfg.CredentialInterval)
		certificateTicker := time.NewTicker(cfg.CertificateInterval)
		defer credentialTicker.Stop()
		defer certificateTicker.Stop()

		runMaintenanceJob(ctx, cfg.JobTimeout, "credential_renewal", runner.RunCredentialRenewal)
		runMaintenanceJob(ctx, cfg.JobTimeout, "certificate_check", runner.RunCertificateCheck)

		for {
			select {
			case <-ctx.Done():
				return
			case <-credentialTicker.C:
				runMaintenanceJob(ctx, cfg.JobTimeout, "credential_renewal", runner.RunCredentialRenewal)
			case <-certificateTicker.C:
				runMaintenanceJob(ctx, cfg.JobTimeout, "certificate_check", runner.RunCertificateCheck)
			}
		}
	}()
	return cancel
}

func runMaintenanceJob(parent context.Context, timeout time.Duration, name string, job func(context.Context) error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	started := time.Now()
	if err := job(ctx); err != nil {
		slog.Error("scheduler: job de mantenimiento finalizó con errores",
			"job", name,
			"duration", time.Since(started).String(),
			"error", err,
		)
		return
	}
	slog.Info("scheduler: job de mantenimiento completado",
		"job", name,
		"duration", time.Since(started).String(),
	)
}

var _ maintenanceRunner = (*usecase.MaintenanceService)(nil)
