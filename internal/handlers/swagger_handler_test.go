package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task-manager/internal/handlers"
)

func TestSwaggerHandler_ServeOpenAPISpec(t *testing.T) {
	handler := handlers.NewSwaggerHandler()

	request := httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil)
	recorder := httptest.NewRecorder()

	handler.ServeOpenAPISpec(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected application/json content type, got %s", contentType)
	}

	var parsedSpec map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &parsedSpec); err != nil {
		t.Fatalf("OpenAPI specification is not valid JSON: %v", err)
	}

	if parsedSpec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi version 3.0.3, got %v", parsedSpec["openapi"])
	}
}

func TestSwaggerHandler_ServeUI(t *testing.T) {
	handler := handlers.NewSwaggerHandler()

	request := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeUI(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %s", contentType)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Errorf("expected body to contain SwaggerUIBundle initialization")
	}
	if !strings.Contains(body, "/swagger/openapi.json") {
		t.Errorf("expected body to reference /swagger/openapi.json")
	}
}
