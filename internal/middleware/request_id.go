package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

type requestIDKey struct{}

// RequestIDHeader is the canonical HTTP header used for correlation tracing.
const RequestIDHeader = "X-Request-ID"

// generateRequestID produces a cryptographically random 32-character hexadecimal string.
func generateRequestID() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(randomBytes)
}

// RequestIDMiddleware extracts an existing X-Request-ID from the client, or generates a new one.
// It injects the ID into the request context and the outgoing HTTP response headers.
func RequestIDMiddleware(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Attach to context and response headers
		requestContext := context.WithValue(request.Context(), requestIDKey{}, requestID)
		responseWriter.Header().Set(RequestIDHeader, requestID)

		nextHandler.ServeHTTP(responseWriter, request.WithContext(requestContext))
	})
}

// GetRequestID retrieves the correlation request ID from context, or returns an empty string.
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey{}).(string); ok {
		return requestID
	}
	return ""
}

