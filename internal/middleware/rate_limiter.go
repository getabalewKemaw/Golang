package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type clientRecord struct {
	tokens   float64
	lastSeen time.Time
}

// IPRateLimiter implements a thread-safe in-memory Token Bucket rate limiter per IP address.
type IPRateLimiter struct {
	rate          float64 // Tokens replenished per second
	burst         float64 // Maximum token bucket capacity
	clients       map[string]*clientRecord
	mutex         sync.Mutex
	cleanupPeriod time.Duration
}

// NewIPRateLimiter initializes an IP-based token bucket limiter.
func NewIPRateLimiter(rate float64, burst float64) *IPRateLimiter {
	limiter := &IPRateLimiter{
		rate:          rate,
		burst:         burst,
		clients:       make(map[string]*clientRecord),
		cleanupPeriod: 3 * time.Minute,
	}

	return limiter
}

// extractClientIP extracts the host IP address, stripping any trailing port.
func extractClientIP(remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		return strings.TrimSpace(remoteAddress)
	}
	return host
}

// Allow checks if the given IP address has an available token to proceed.
func (limiter *IPRateLimiter) Allow(clientIP string) bool {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	currentTime := time.Now()

	record, exists := limiter.clients[clientIP]
	if !exists {
		limiter.clients[clientIP] = &clientRecord{
			tokens:   limiter.burst - 1.0,
			lastSeen: currentTime,
		}
		return true
	}

	// Calculate tokens accumulated since last visit
	elapsedSeconds := currentTime.Sub(record.lastSeen).Seconds()
	record.lastSeen = currentTime
	record.tokens += elapsedSeconds * limiter.rate

	if record.tokens > limiter.burst {
		record.tokens = limiter.burst
	}

	// If at least one full token is available, consume it
	if record.tokens >= 1.0 {
		record.tokens -= 1.0
		return true
	}

	return false
}

// CleanupStaleClients removes client records that have been idle for longer than maxAge.
func (limiter *IPRateLimiter) CleanupStaleClients(maxAge time.Duration) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	currentTime := time.Now()
	for ipAddress, record := range limiter.clients {
		if currentTime.Sub(record.lastSeen) > maxAge {
			delete(limiter.clients, ipAddress)
		}
	}
}

// RateLimiterMiddleware enforces rate limits per incoming client IP address.
func RateLimiterMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(nextHandler http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			clientIP := extractClientIP(request.RemoteAddr)

			if !limiter.Allow(clientIP) {
				requestID := GetRequestID(request.Context())

				responseWriter.Header().Set("Content-Type", "application/json")
				responseWriter.Header().Set("Retry-After", "1")
				responseWriter.WriteHeader(http.StatusTooManyRequests)

				responsePayload := map[string]any{
					"error": "too many requests; please slow down and retry",
				}
				if requestID != "" {
					responsePayload["request_id"] = requestID
				}

				_ = json.NewEncoder(responseWriter).Encode(responsePayload)
				return
			}

			nextHandler.ServeHTTP(responseWriter, request)
		})
	}
}

