package routes

import (
	"database/sql"
	"fmt"
	"net/http"

	"task-manager/internal/handlers"
)

func SetupRouter(taskHandler *handlers.TaskHandler, databasePool *sql.DB) *http.ServeMux {
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

	router.HandleFunc("GET /", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/plain")
		responseWriter.WriteHeader(http.StatusOK)
		responseWriter.Write([]byte("Task Manager API is running. Access endpoints under /api/v1/"))
	})

	RegisterTaskRoutes(router, taskHandler)

	return router
}
