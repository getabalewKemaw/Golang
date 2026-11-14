package middleware

import (
	"net/http"
	"strconv"
	"time"

	"task-manager/pkg/telemetry"
)

// MetricsMiddleware records request count and latency distributions into Prometheus metrics.
func MetricsMiddleware(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		// Ignore scraping requests to /metrics to prevent self-referential metric pollution
		if request.URL.Path == "/metrics" {
			nextHandler.ServeHTTP(responseWriter, request)
			return
		}

		startTime := time.Now()

		recorder := &statusRecorder{
			ResponseWriter: responseWriter,
			statusCode:     http.StatusOK,
		}

		nextHandler.ServeHTTP(recorder, request)

		durationSeconds := time.Since(startTime).Seconds()
		normalizedPath := telemetry.NormalizePath(request.URL.Path)
		statusString := strconv.Itoa(recorder.statusCode)

		// Increment cumulative request counter: http_requests_total{method="GET", path="/api/v1/tasks", status="200"}
		telemetry.HTTPRequestsTotal.WithLabelValues(request.Method, normalizedPath, statusString).Inc()

		// Record latency in duration histogram: http_request_duration_seconds{method="GET", path="/api/v1/tasks"}
		telemetry.HTTPRequestDuration.WithLabelValues(request.Method, normalizedPath).Observe(durationSeconds)
	})
}

