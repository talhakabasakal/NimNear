package rpc

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rpcRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimnear_nimiq_rpc_requests_total",
		Help: "Nimiq JSON-RPC calls by method and outcome.",
	}, []string{"method", "outcome"})
	rpcDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nimnear_nimiq_rpc_request_duration_seconds",
		Help:    "Nimiq JSON-RPC latency by method.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method"})
)

func observeRPC(method, outcome string, seconds float64) {
	if method == "" {
		method = "unknown"
	}
	if outcome == "" {
		outcome = "error"
	}
	rpcRequests.WithLabelValues(method, outcome).Inc()
	rpcDuration.WithLabelValues(method).Observe(seconds)
}
