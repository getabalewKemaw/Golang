package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// RecoveryMiddleware captures any panic thrown by downstream handlers,
// logs the stack trace with correlation request_id, and writes a safe HTTP 500 response.
func RecoveryMiddleware(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		defer func() {
			if panicValue := recover(); panicValue != nil {
				requestID := GetRequestID(request.Context())
				stackTrace := string(debug.Stack())

				slog.Error("panic recovered in HTTP handler",
					slog.String("request_id", requestID),
					slog.Any("panic", panicValue),
					slog.String("stack", stackTrace),
				)

				responseWriter.Header().Set("Content-Type", "application/json")
				responseWriter.WriteHeader(http.StatusInternalServerError)

				responsePayload := map[string]any{
					"error": "internal server error",
				}
				if requestID != "" {
					responsePayload["request_id"] = requestID
				}

				_ = json.NewEncoder(responseWriter).Encode(responsePayload)
			}
		}()

		nextHandler.ServeHTTP(responseWriter, request)
	})
}
