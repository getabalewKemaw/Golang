package middleware_test

import (
	"net/http"
	"net/http/httptest"
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

