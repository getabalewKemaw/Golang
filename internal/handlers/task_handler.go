package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"task-manager/internal/dto"
	"task-manager/internal/middleware"
	"task-manager/internal/models"
	"task-manager/internal/repositories"
	"task-manager/internal/services"
	"task-manager/internal/workers"
)

type TaskHandler struct {
	taskService services.TaskService
	eventPool   *workers.TaskEventWorkerPool
}

func NewTaskHandler(taskService services.TaskService, eventPool *workers.TaskEventWorkerPool) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
		eventPool:   eventPool,
	}
}

func (handler *TaskHandler) CreateTask(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var input dto.CreateTaskDTO
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": "malformed JSON body"})
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	createdTask, err := handler.taskService.CreateTask(request.Context(), userID, input)
	if err != nil {
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if handler.eventPool != nil {
		handler.eventPool.Enqueue(workers.TaskEvent{
			TaskID:    createdTask.ID,
			UserID:    createdTask.UserID,
			Title:     createdTask.Title,
			EventType: workers.TaskEventCreated,
			Timestamp: time.Now(),
		})
	}

	writeJSON(responseWriter, http.StatusCreated, createdTask)
}

func (handler *TaskHandler) CreateTasksBatch(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var input dto.BatchCreateTaskDTO
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": "malformed JSON body"})
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	batchResponse, err := handler.taskService.BatchCreateTasks(request.Context(), userID, input)
	if err != nil {
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusCreated, batchResponse)
}

func (handler *TaskHandler) GetTaskByID(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	taskID, err := parseIDParam(request)
	if err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	task, err := handler.taskService.GetTaskByID(request.Context(), userID, taskID)
	if err != nil {
		if errors.Is(err, repositories.ErrTaskNotFound) {
			writeJSON(responseWriter, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusOK, task)
}

func (handler *TaskHandler) GetAllTasks(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	tasks, err := handler.taskService.GetAllTasks(request.Context(), userID)
	if err != nil {
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusOK, tasks)
}

func (handler *TaskHandler) UpdateTask(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	taskID, err := parseIDParam(request)
	if err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var input dto.UpdateTaskDTO
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": "malformed JSON body"})
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	updatedTask, err := handler.taskService.UpdateTask(request.Context(), userID, taskID, input)
	if err != nil {
		if errors.Is(err, repositories.ErrTaskNotFound) {
			writeJSON(responseWriter, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if handler.eventPool != nil {
		eventType := workers.TaskEventUpdated
		if updatedTask.Status == models.StatusCompleted {
			eventType = workers.TaskEventCompleted
		}
		handler.eventPool.Enqueue(workers.TaskEvent{
			TaskID:    updatedTask.ID,
			UserID:    updatedTask.UserID,
			Title:     updatedTask.Title,
			EventType: eventType,
			Timestamp: time.Now(),
		})
	}

	writeJSON(responseWriter, http.StatusOK, updatedTask)
}

func (handler *TaskHandler) DeleteTask(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	taskID, err := parseIDParam(request)
	if err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := handler.taskService.DeleteTask(request.Context(), userID, taskID); err != nil {
		if errors.Is(err, repositories.ErrTaskNotFound) {
			writeJSON(responseWriter, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "task deleted successfully"})
}

// parseIDParam extracts and validates the "id" path parameter from the URL.
func parseIDParam(request *http.Request) (int64, error) {
	taskID, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil || taskID <= 0 {
		return 0, errors.New("invalid task id in path: must be a positive integer")
	}
	return taskID, nil
}

// writeJSON sets the Content-Type header and serializes data to JSON.
func writeJSON(responseWriter http.ResponseWriter, statusCode int, data any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(data)
}
