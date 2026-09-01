package app

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestsTotal counts requests by method, path and status.
var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration observes request latency by method and path.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// DBQueriesTotal counts database operations by table and status.
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total DB queries",
		},
		[]string{"operation", "table", "status"},
	)

	// DBQueryDuration observes database operation latency.
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "DB query duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "table"},
	)

	// AppInfo is a constant 1 gauge with build labels.
	AppInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "app_info",
			Help: "App info (always 1)",
		},
		[]string{"version", "commit", "go_version"},
	)
)

// RegisterAppInfo sets app_info=1 with build labels.
func RegisterAppInfo(version, commit string) {
	AppInfo.WithLabelValues(version, commit, runtime.Version()).Set(1)
}
