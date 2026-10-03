package observability

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns an isolated Prometheus registry. Labels are deliberately bounded:
// tenant, invoice, customer and product identifiers are never metric labels.
type Metrics struct {
	registry         *prometheus.Registry
	httpRequests     *prometheus.CounterVec
	httpDuration     *prometheus.HistogramVec
	httpInFlight     prometheus.Gauge
	emissions        *prometheus.CounterVec
	emissionDuration *prometheus.HistogramVec
	outboxEvents     *prometheus.CounterVec
	outboxPending    prometheus.GaugeFunc
	providerMu       sync.RWMutex
	outboxPendingFn  func() (float64, error)
}

func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "supay", Subsystem: "http", Name: "requests_total",
			Help: "Total de solicitudes HTTP por método, ruta normalizada y estado.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "supay", Subsystem: "http", Name: "request_duration_seconds",
			Help:    "Duración de solicitudes HTTP por método y ruta normalizada.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "supay", Subsystem: "http", Name: "requests_in_flight",
			Help: "Solicitudes HTTP atendidas actualmente.",
		}),
		emissions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "supay", Subsystem: "invoice", Name: "emissions_total",
			Help: "Trabajos de emisión de facturas clasificados por resultado.",
		}, []string{"result"}),
		emissionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "supay", Subsystem: "invoice", Name: "emission_duration_seconds",
			Help:    "Duración de los intentos de emisión de facturas.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		}, []string{"result"}),
		outboxEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "supay", Subsystem: "outbox", Name: "events_total",
			Help: "Eventos outbox procesados por resultado.",
		}, []string{"result"}),
	}
	m.outboxPending = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "supay", Subsystem: "outbox", Name: "pending",
		Help: "Cantidad actual de eventos outbox en estado PENDING.",
	}, func() float64 {
		m.providerMu.RLock()
		provider := m.outboxPendingFn
		m.providerMu.RUnlock()
		if provider == nil {
			return 0
		}
		value, err := provider()
		if err != nil {
			return math.NaN()
		}
		return value
	})
	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.httpRequests,
		m.httpDuration,
		m.httpInFlight,
		m.emissions,
		m.emissionDuration,
		m.outboxEvents,
		m.outboxPending,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "supay", Name: "build_info", Help: "Información de la versión pública de la API.",
			ConstLabels: prometheus.Labels{"api_version": "v1"},
		}, func() float64 { return 1 }),
	)
	return m
}

// SetOutboxPendingProvider connects the metrics registry to the durable outbox.
// The provider is evaluated at scrape time so restarts cannot hide old rows.
func (m *Metrics) SetOutboxPendingProvider(provider func() (float64, error)) {
	if m == nil {
		return
	}
	m.providerMu.Lock()
	m.outboxPendingFn = provider
	m.providerMu.Unlock()
}

var (
	defaultMetrics     *Metrics
	defaultMetricsOnce sync.Once
)

func DefaultMetrics() *Metrics {
	defaultMetricsOnce.Do(func() { defaultMetrics = NewMetrics() })
	return defaultMetrics
}

func (m *Metrics) Handler() http.Handler {
	if m == nil {
		m = DefaultMetrics()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (m *Metrics) BeginHTTPRequest() func(method, route string, status int, duration time.Duration) {
	if m == nil {
		return func(string, string, int, time.Duration) {}
	}
	m.httpInFlight.Inc()
	return func(method, route string, status int, duration time.Duration) {
		m.httpInFlight.Dec()
		m.httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
		m.httpDuration.WithLabelValues(method, route).Observe(duration.Seconds())
	}
}

func (m *Metrics) ObserveEmission(result string, duration time.Duration) {
	if m == nil {
		return
	}
	result = boundedResult(result)
	m.emissions.WithLabelValues(result).Inc()
	m.emissionDuration.WithLabelValues(result).Observe(duration.Seconds())
}

func (m *Metrics) ObserveOutbox(result string) {
	if m == nil {
		return
	}
	m.outboxEvents.WithLabelValues(boundedResult(result)).Inc()
}

func boundedResult(result string) string {
	switch result {
	case "success", "retry", "rejected", "discarded", "rate_limited", "circuit_open", "in_progress", "published", "publish_failed", "mark_failed":
		return result
	default:
		return "unknown"
	}
}
