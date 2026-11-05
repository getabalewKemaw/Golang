package routes

import (
	"net/http"

	"task-manager/internal/handlers"
	"task-manager/internal/middleware"
	"task-manager/internal/repositories"
)

func RegisterTaskRoutes(
	router *http.ServeMux,
	taskHandler *handlers.TaskHandler,
	jwtAccessSecret string,
	tokenRepository repositories.TokenRepository,
) {
	authMiddleware := middleware.RequireAuth(jwtAccessSecret, tokenRepository)

	router.Handle("POST /api/v1/tasks", authMiddleware(http.HandlerFunc(taskHandler.CreateTask)))
	router.Handle("GET /api/v1/tasks", authMiddleware(http.HandlerFunc(taskHandler.GetAllTasks)))
	router.Handle("GET /api/v1/tasks/{id}", authMiddleware(http.HandlerFunc(taskHandler.GetTaskByID)))
	router.Handle("PUT /api/v1/tasks/{id}", authMiddleware(http.HandlerFunc(taskHandler.UpdateTask)))
	router.Handle("DELETE /api/v1/tasks/{id}", authMiddleware(http.HandlerFunc(taskHandler.DeleteTask)))
}
