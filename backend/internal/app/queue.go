package app

import (
	"github.com/brandsrx/supay/internal/emissionqueue"
)

func configureQueue(c *App) error {
	if !c.cfg.Queue.Enabled || !c.cfg.RunMode.RunsWorker() {
		return nil
	}
	queue, err := emissionqueue.NewService(c.db, c.outboxRepo, c.invoiceUsecase, emissionqueue.Config{
		DispatchInterval:  c.cfg.Queue.DispatchInterval,
		DispatchBatchSize: c.cfg.Queue.DispatchBatchSize,
		OutboxLockTimeout: c.cfg.Queue.OutboxLockTimeout,
		MaxWorkers:        c.cfg.Queue.MaxWorkers,
		MaxAttempts:       c.cfg.Queue.MaxAttempts,
		JobTimeout:        c.cfg.Queue.JobTimeout,
		SoftStopTimeout:   c.cfg.Queue.SoftStopTimeout,
		RetryBase:         c.cfg.Queue.RetryBase,
		RetryMax:          c.cfg.Queue.RetryMax,
		RatePerSecond:     c.cfg.Queue.TenantRatePerSecond,
		RateBurst:         c.cfg.Queue.TenantRateBurst,
		CircuitThreshold:  c.cfg.Queue.CircuitThreshold,
		CircuitCooldown:   c.cfg.Queue.CircuitCooldown,
	})
	if err != nil {
		return err
	}
	c.emissionQueue = queue
	return nil
}
