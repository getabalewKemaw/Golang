package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder intercepts the HTTP status code and written byte count.
type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func (recorder *statusRecorder) Write(bytes []byte) (int, error) {
	writtenBytes, err := recorder.ResponseWriter.Write(bytes)
	recorder.bytesWritten += writtenBytes
	return writtenBytes, err
}

// LoggingMiddleware logs all incoming HTTP requests and responses using structured JSON (log/slog).
func LoggingMiddleware(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		startTime := time.Now()

		recorder := &statusRecorder{
			ResponseWriter: responseWriter,
			statusCode:     http.StatusOK,
		}

		nextHandler.ServeHTTP(recorder, request)

		duration := time.Since(startTime)
		requestID := GetRequestID(request.Context())

		slog.InfoContext(request.Context(), "HTTP request completed",
			slog.String("request_id", requestID),
			slog.String("method", request.Method),
			slog.String("path", request.URL.Path),
			slog.Int("status", recorder.statusCode),
			slog.Float64("duration_ms", float64(duration.Microseconds())/1000.0),
			slog.Int("bytes", recorder.bytesWritten),
			slog.String("remote_addr", request.RemoteAddr),
			slog.String("user_agent", request.UserAgent()),
		)
	})
}
