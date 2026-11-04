package routes

import (
	"net/http"

	"task-manager/internal/handlers"
)

func RegisterTaskRoutes(router *http.ServeMux, taskHandler *handlers.TaskHandler) {
	router.HandleFunc("POST /api/v1/tasks", taskHandler.CreateTask)
	router.HandleFunc("GET /api/v1/tasks", taskHandler.GetAllTasks)
	router.HandleFunc("GET /api/v1/tasks/{id}", taskHandler.GetTaskByID)
	router.HandleFunc("PUT /api/v1/tasks/{id}", taskHandler.UpdateTask)
	router.HandleFunc("DELETE /api/v1/tasks/{id}", taskHandler.DeleteTask)
}
