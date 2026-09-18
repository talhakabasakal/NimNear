package websocket

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	wsConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "nimnear_ws_connections",
		Help: "Active Nimnear WebSocket connections on this instance.",
	})
	domainEventsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimnear_domain_events_published_total",
		Help: "Domain events published after authoritative state transitions.",
	}, []string{"type"})
	domainEventsDelivered = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimnear_domain_events_delivered_total",
		Help: "Domain events delivered to local WebSocket rooms.",
	}, []string{"type"})
	domainEventsPublishFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimnear_domain_events_publish_failures_total",
		Help: "Failed domain event publishes. Payment state is not rolled back.",
	}, []string{"type"})
	wsFanoutPublishFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nimnear_ws_fanout_publish_failures_total",
		Help: "Redis/WebSocket fan-out publish failures.",
	})
)

func incWSConnections(delta float64) {
	wsConnections.Add(delta)
}

func observePublished(eventType string) {
	domainEventsPublished.WithLabelValues(eventType).Inc()
}

func observeDelivered(eventType string) {
	domainEventsDelivered.WithLabelValues(eventType).Inc()
}

func observePublishFailure(eventType string) {
	domainEventsPublishFailures.WithLabelValues(eventType).Inc()
}

func observeFanoutFailure() {
	wsFanoutPublishFailures.Inc()
}
