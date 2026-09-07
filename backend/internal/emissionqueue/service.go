package emissionqueue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/observability"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivertype"
	"gorm.io/gorm"
)

type Config struct {
	DispatchInterval  time.Duration
	DispatchBatchSize int
	OutboxLockTimeout time.Duration
	MaxWorkers        int
	MaxAttempts       int
	JobTimeout        time.Duration
	SoftStopTimeout   time.Duration
	RetryBase         time.Duration
	RetryMax          time.Duration
	RatePerSecond     float64
	RateBurst         int
	CircuitThreshold  int
	CircuitCooldown   time.Duration
}

type Service struct {
	db         *sql.DB
	client     *river.Client[*sql.Tx]
	dispatcher *Dispatcher
	cancel     context.CancelFunc
	mu         sync.Mutex
	started    bool
}

func NewService(db *gorm.DB, outbox domain.OutboxRepository, processor InvoiceEmissionProcessor, cfg Config) (*Service, error) {
	if db == nil {
		return nil, errors.New("cola de emisión: base de datos no configurada")
	}
	if outbox == nil {
		return nil, errors.New("cola de emisión: repositorio outbox no configurado")
	}
	if processor == nil {
		return nil, errors.New("cola de emisión: procesador no configurado")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("cola de emisión: obtener database/sql: %w", err)
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 10
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 8
	}

	limiter := NewTenantRateLimiter(cfg.RatePerSecond, cfg.RateBurst)
	breaker := NewCircuitBreaker(cfg.CircuitThreshold, cfg.CircuitCooldown)
	metrics := observability.DefaultMetrics()
	workers := river.NewWorkers()
	worker := NewInvoiceEmissionWorker(processor, limiter, breaker, cfg.JobTimeout).WithMetrics(metrics)
	if err := river.AddWorkerSafely(workers, worker); err != nil {
		return nil, fmt.Errorf("registrar worker de emisión: %w", err)
	}

	client, err := river.NewClient(riverdatabasesql.New(sqlDB), &river.Config{
		Queues: map[string]river.QueueConfig{
			InvoiceEmissionQueue: {MaxWorkers: cfg.MaxWorkers},
		},
		Workers:         workers,
		MaxAttempts:     cfg.MaxAttempts,
		JobTimeout:      cfg.JobTimeout,
		SoftStopTimeout: cfg.SoftStopTimeout,
		RetryPolicy:     NewExponentialRetryPolicy(cfg.RetryBase, cfg.RetryMax),
		Logger:          slog.Default(),
	})
	if err != nil {
		return nil, fmt.Errorf("crear cliente River: %w", err)
	}
	publisher := &RiverPublisher{client: client, maxAttempts: cfg.MaxAttempts}
	return &Service{
		db:         sqlDB,
		client:     client,
		dispatcher: NewDispatcher(outbox, publisher, cfg.DispatchInterval, cfg.DispatchBatchSize, cfg.OutboxLockTimeout).WithMetrics(metrics),
	}, nil
}

func (s *Service) Start(parent context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if err := s.validateSchema(parent); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	if err := s.client.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("iniciar River: %w", err)
	}
	s.cancel = cancel
	s.started = true
	go s.dispatcher.Run(ctx)
	slog.Info("cola de emisión iniciada", "queue", InvoiceEmissionQueue)
	return nil
}

func (s *Service) validateSchema(ctx context.Context) error {
	for _, table := range []string{"outbox", "river_job"} {
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			return fmt.Errorf("validar esquema de cola: %w", err)
		}
		if !exists {
			return fmt.Errorf("tabla %s no existe: ejecute las migraciones (AUTO_MIGRATE=true o el comando de migración del despliegue)", table)
		}
	}
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	s.started = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return s.client.Stop(ctx)
}

type riverInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type RiverPublisher struct {
	client      riverInserter
	maxAttempts int
}

func (p *RiverPublisher) Publish(ctx context.Context, event domain.OutboxEvent) error {
	if event.EventType != domain.OutboxEventInvoiceEmit || event.AggregateType != domain.OutboxAggregateInvoice {
		return fmt.Errorf("evento outbox no soportado: %s/%s", event.AggregateType, event.EventType)
	}
	_, err := p.client.Insert(ctx, InvoiceEmissionArgs{
		InvoiceID: event.AggregateID,
		TenantID:  event.TenantID,
		OutboxID:  event.ID,
	}, &river.InsertOpts{
		Queue:       InvoiceEmissionQueue,
		MaxAttempts: p.maxAttempts,
		Tags:        []string{"tenant-" + event.TenantID},
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByQueue: true,
		},
	})
	return err
}

type ExponentialRetryPolicy struct {
	base time.Duration
	max  time.Duration
	now  func() time.Time
}

func NewExponentialRetryPolicy(base, maximum time.Duration) *ExponentialRetryPolicy {
	if base <= 0 {
		base = 2 * time.Second
	}
	if maximum <= 0 {
		maximum = 5 * time.Minute
	}
	if maximum < base {
		maximum = base
	}
	return &ExponentialRetryPolicy{base: base, max: maximum, now: timeNow()}
}

func (p *ExponentialRetryPolicy) NextRetry(job *rivertype.JobRow) time.Time {
	attempt := job.Attempt
	if attempt < 1 {
		attempt = 1
	}
	shift := min(attempt-1, 30)
	delay := p.base * time.Duration(1<<shift)
	if delay > p.max || delay < 0 {
		delay = p.max
	}
	return p.now().Add(delay)
}

var _ river.ClientRetryPolicy = (*ExponentialRetryPolicy)(nil)
