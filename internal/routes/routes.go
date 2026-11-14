package routes

import (
	"database/sql"
	"fmt"
	"net/http"

	"task-manager/internal/handlers"
	"task-manager/internal/repositories"
	"task-manager/pkg/telemetry"
)

func SetupRouter(
	taskHandler *handlers.TaskHandler,
	authHandler *handlers.AuthHandler,
	jwtAccessSecret string,
	tokenRepository repositories.TokenRepository,
	databasePool *sql.DB,
) *http.ServeMux {
	router := http.NewServeMux()

	healthHandler := func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		databaseStatus := "connected"
		if databasePool == nil || databasePool.Ping() != nil {
			databaseStatus = "disconnected"
		}
		responseWriter.WriteHeader(http.StatusOK)
		fmt.Fprintf(responseWriter, `{"status":"ok","database":"%s"}`, databaseStatus)
	}

	router.HandleFunc("GET /api/v1/health", healthHandler)
	router.HandleFunc("GET /health", healthHandler) // Root alias

	// Expose standard Prometheus metrics scrape endpoint
	router.Handle("GET /metrics", telemetry.MetricsHandler())

	// Swagger / OpenAPI documentation routes
	swaggerHandler := handlers.NewSwaggerHandler()
	router.HandleFunc("GET /swagger/openapi.json", swaggerHandler.ServeOpenAPISpec)
	router.HandleFunc("GET /swagger/", swaggerHandler.ServeUI)
	router.HandleFunc("GET /swagger", func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Redirect(responseWriter, request, "/swagger/", http.StatusMovedPermanently)
	})
	router.HandleFunc("GET /docs", func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Redirect(responseWriter, request, "/swagger/", http.StatusMovedPermanently)
	})

	// Root welcome route (exact match only via /{$})
	router.HandleFunc("GET /{$}", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/plain")
		responseWriter.WriteHeader(http.StatusOK)
		responseWriter.Write([]byte("Task Manager API is running. Access endpoints under /api/v1/ or documentation at /swagger/"))
	})

	RegisterAuthRoutes(router, authHandler)
	RegisterTaskRoutes(router, taskHandler, jwtAccessSecret, tokenRepository)

	return router
}
