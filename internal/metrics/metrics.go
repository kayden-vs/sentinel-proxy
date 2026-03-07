package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	BytesStreamed       *prometheus.CounterVec
	RequestsTotal       *prometheus.CounterVec
	StreamKillsTotal    *prometheus.CounterVec
	AnomaliesDetected   *prometheus.CounterVec
	ActiveStreams       prometheus.Gauge
	StreamDuration      *prometheus.HistogramVec
	BytesPerRequest     *prometheus.HistogramVec
	ThresholdDecisions  *prometheus.CounterVec
	RedisErrors         prometheus.Counter
	RedisLatency        *prometheus.HistogramVec
	ViolationsByGrade   *prometheus.CounterVec
	IdentityResolutions *prometheus.CounterVec
}

var (
	instance *Metrics
	once     sync.Once
)

func Get() *Metrics {
	once.Do(func() {
		instance = newMetrics()
	})
	return instance
}

func newMetrics() *Metrics {
	m := &Metrics{
		BytesStreamed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "bytes_streamed_total",
				Help:      "Total bytes streamed through the proxy.",
			},
			[]string{"user_id", "endpoint"},
		),
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "requests_total",
				Help:      "Total number of proxy requests.",
			},
			[]string{"endpoint", "status", "identity_method"},
		),
		StreamKillsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "stream_kills_total",
				Help:      "Total number of stream terminations.",
			},
			[]string{"reason", "endpoint"},
		),
		AnomaliesDetected: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "anomalies_detected_total",
				Help:      "Total number of rate anomalies detected.",
			},
			[]string{"user_id", "endpoint"},
		),
		ActiveStreams: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "active_streams",
				Help:      "Number of currently active streaming connections.",
			},
		),
		StreamDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "stream_duration_seconds",
				Help:      "Duration of streaming connections.",
				Buckets:   prometheus.ExponentialBuckets(0.1, 2, 12),
			},
			[]string{"endpoint", "outcome"},
		),
		BytesPerRequest: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "bytes_per_request",
				Help:      "Distribution of bytes per request.",
				Buckets:   prometheus.ExponentialBuckets(1024, 4, 10),
			},
			[]string{"endpoint"},
		),
		ThresholdDecisions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "threshold",
				Name:      "decisions_total",
				Help:      "Total threshold evaluation decisions.",
			},
			[]string{"outcome"},
		),
		RedisErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "redis",
				Name:      "errors_total",
				Help:      "Total Redis operation errors (fail-open triggered).",
			},
		),
		RedisLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "sentinel",
				Subsystem: "redis",
				Name:      "operation_duration_seconds",
				Help:      "Redis operation latency.",
				Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"operation"},
		),
		ViolationsByGrade: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "proxy",
				Name:      "violations_by_grade_total",
				Help:      "Violations by enforcement grade.",
			},
			[]string{"grade"},
		),
		IdentityResolutions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sentinel",
				Subsystem: "identity",
				Name:      "resolutions_total",
				Help:      "Identity resolutions by method.",
			},
			[]string{"method"},
		),
	}

	prometheus.MustRegister(
		m.BytesStreamed,
		m.RequestsTotal,
		m.StreamKillsTotal,
		m.AnomaliesDetected,
		m.ActiveStreams,
		m.StreamDuration,
		m.BytesPerRequest,
		m.ThresholdDecisions,
		m.RedisErrors,
		m.RedisLatency,
		m.ViolationsByGrade,
		m.IdentityResolutions,
	)

	return m
}

func Handler() http.Handler {
	return promhttp.Handler()
}
