package observability

import (
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type Metrics struct {
	MessagesSent     *prometheus.CounterVec
	MessagesLatency  *prometheus.HistogramVec
	AgentsOnline     prometheus.Gauge
	TopicsActive     prometheus.Gauge
	ApprovalsPending prometheus.Gauge
	ErrorsCount      *prometheus.CounterVec
}

func NewMetrics(registry *prometheus.Registry) *Metrics {
	m := &Metrics{
		MessagesSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bobberchat_messages_sent_total",
				Help: "Total messages sent by BobberChat",
			},
			[]string{"agent_id", "tag"},
		),
		MessagesLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bobberchat_messages_latency_ms",
				Help:    "Message handling latency in milliseconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"agent_id"},
		),
		AgentsOnline: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "bobberchat_agents_online",
				Help: "Number of online agents",
			},
		),
		TopicsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "bobberchat_topics_active",
				Help: "Number of active topics",
			},
		),
		ApprovalsPending: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "bobberchat_approvals_pending",
				Help: "Number of pending approvals",
			},
		),
		ErrorsCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bobberchat_errors_count_total",
				Help: "Total number of errors",
			},
			[]string{"agent_id", "error_type"},
		),
	}

	if registry != nil {
		registry.MustRegister(
			m.MessagesSent,
			m.MessagesLatency,
			m.AgentsOnline,
			m.TopicsActive,
			m.ApprovalsPending,
			m.ErrorsCount,
		)
	}

	return m
}

func SetupLogger(level, format string) zerolog.Logger {
	parsedLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		parsedLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(parsedLevel)

	if strings.EqualFold(format, "console") || strings.EqualFold(format, "text") {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	}

	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
