package routes

import (
	"database/sql"
	"fmt"
	"net/http"

	"task-manager/internal/handlers"
	"task-manager/internal/repositories"
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

	// Root welcome route (exact match only via /{$})
	router.HandleFunc("GET /{$}", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/plain")
		responseWriter.WriteHeader(http.StatusOK)
		responseWriter.Write([]byte("Task Manager API is running. Access endpoints under /api/v1/"))
	})

	RegisterAuthRoutes(router, authHandler)
	RegisterTaskRoutes(router, taskHandler, jwtAccessSecret, tokenRepository)

	return router
}
