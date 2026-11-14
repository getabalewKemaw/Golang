package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task-manager/internal/middleware"
)

func TestRequestIDMiddleware_GeneratesNewID(testingT *testing.T) {
	var capturedContextID string

	nextHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		capturedContextID = middleware.GetRequestID(request.Context())
		responseWriter.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.RequestIDMiddleware(nextHandler)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(recorder, request)

	responseHeaderID := recorder.Header().Get(middleware.RequestIDHeader)
	if responseHeaderID == "" {
		testingT.Fatal("expected X-Request-ID in response headers, got empty string")
	}

	if capturedContextID != responseHeaderID {
		testingT.Errorf("expected context request ID '%s' to match header '%s'", capturedContextID, responseHeaderID)
	}

	if len(responseHeaderID) != 32 {
		testingT.Errorf("expected 32-character hex request ID, got length %d (%s)", len(responseHeaderID), responseHeaderID)
	}
}

func TestRequestIDMiddleware_PreservesExistingID(testingT *testing.T) {
	customID := "client-custom-trace-id-12345"

	var capturedContextID string
	nextHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		capturedContextID = middleware.GetRequestID(request.Context())
		responseWriter.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.RequestIDMiddleware(nextHandler)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set(middleware.RequestIDHeader, customID)
	recorder := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(recorder, request)

	responseHeaderID := recorder.Header().Get(middleware.RequestIDHeader)
	if responseHeaderID != customID {
		testingT.Errorf("expected preserved header ID '%s', got '%s'", customID, responseHeaderID)
	}

	if capturedContextID != customID {
		testingT.Errorf("expected preserved context ID '%s', got '%s'", customID, capturedContextID)
	}
}

func TestLoggingMiddleware_ExecutesHandlerAndRecordsStatus(testingT *testing.T) {
	nextHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusCreated)
		_, _ = responseWriter.Write([]byte(`{"status":"created"}`))
	})

	wrappedHandler := middleware.LoggingMiddleware(nextHandler)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", nil)
	recorder := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		testingT.Errorf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}
	if recorder.Body.String() != `{"status":"created"}` {
		testingT.Errorf("expected response body `{\"status\":\"created\"}`, got %s", recorder.Body.String())
	}
}

func TestRecoveryMiddleware_RecoversFromPanic(testingT *testing.T) {
	panickingHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		panic("simulated fatal nil pointer dereference!")
	})

	wrappedHandler := middleware.RecoveryMiddleware(panickingHandler)

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		testingT.Errorf("expected status 500, got %d", recorder.Code)
	}

	expectedSubstring := `"error":"internal server error"`
	if !strings.Contains(recorder.Body.String(), expectedSubstring) {
		testingT.Errorf("expected response to contain %s, got: %s", expectedSubstring, recorder.Body.String())
	}
}

func TestRateLimiterMiddleware_AllowsUpToBurstAndRejectsExcess(testingT *testing.T) {
	// Rate limiter configured with burst capacity of 3 tokens
	limiter := middleware.NewIPRateLimiter(1.0, 3.0)
	rateLimitingMiddleware := middleware.RateLimiterMiddleware(limiter)

	successHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	})

	wrappedHandler := rateLimitingMiddleware(successHandler)

	clientAddr := "192.168.1.100:54321"

	// 1. First 3 requests (within burst) should all succeed
	for requestIndex := 1; requestIndex <= 3; requestIndex++ {
		request := httptest.NewRequest(http.MethodGet, "/tasks", nil)
		request.RemoteAddr = clientAddr
		recorder := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			testingT.Fatalf("request %d: expected status 200, got %d", requestIndex, recorder.Code)
		}
	}

	// 2. 4th request (exceeds burst capacity) should be rejected immediately (429 Too Many Requests)
	excessRequest := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	excessRequest.RemoteAddr = clientAddr
	excessRecorder := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(excessRecorder, excessRequest)

	if excessRecorder.Code != http.StatusTooManyRequests {
		testingT.Fatalf("expected status 429 Too Many Requests, got %d", excessRecorder.Code)
	}

	if retryAfter := excessRecorder.Header().Get("Retry-After"); retryAfter == "" {
		testingT.Error("expected Retry-After header in 429 response")
	}
}
