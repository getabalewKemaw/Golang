package telemetry

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPRequestsTotal counts all completed HTTP requests partitioned by method, path, and status code.
var HTTPRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "task_manager",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of completed HTTP requests.",
	},
	[]string{"method", "path", "status"},
)

// HTTPRequestDuration tracks latency distribution across standard sub-second and multi-second buckets.
var HTTPRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "task_manager",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Histogram of HTTP request durations in seconds.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	},
	[]string{"method", "path"},
)

func init() {
	// Register HTTP metrics with Prometheus default registry
	prometheus.MustRegister(HTTPRequestsTotal)
	prometheus.MustRegister(HTTPRequestDuration)
}

// RegisterDatabaseMetrics binds live PostgreSQL connection pool statistics to Prometheus Gauges.
// Prometheus invokes these functions dynamically whenever /metrics is scraped.
func RegisterDatabaseMetrics(databasePool *sql.DB) {
	if databasePool == nil {
		return
	}

	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "task_manager",
			Subsystem: "database",
			Name:      "open_connections",
			Help:      "Total number of open connections currently established with PostgreSQL.",
		},
		func() float64 {
			return float64(databasePool.Stats().OpenConnections)
		},
	))

	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "task_manager",
			Subsystem: "database",
			Name:      "in_use_connections",
			Help:      "Current number of database connections actively executing queries.",
		},
		func() float64 {
			return float64(databasePool.Stats().InUse)
		},
	))

	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "task_manager",
			Subsystem: "database",
			Name:      "idle_connections",
			Help:      "Current number of idle database connections sitting ready in the pool.",
		},
		func() float64 {
			return float64(databasePool.Stats().Idle)
		},
	))
}

// MetricsHandler returns the official standard Prometheus HTTP handler for scraping.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// NormalizePath replaces dynamic path IDs (like /tasks/42) with /tasks/:id to prevent high cardinality.
// In Prometheus, every unique label value creates a new time-series. Normalizing paths keeps memory small.
func NormalizePath(urlPath string) string {
	segments := strings.Split(urlPath, "/")
	for index, segment := range segments {
		// If segment is numeric ID, replace with :id
		if len(segment) > 0 && isNumeric(segment) {
			segments[index] = ":id"
		}
	}
	return strings.Join(segments, "/")
}

func isNumeric(text string) bool {
	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
