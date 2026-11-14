package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"task-manager/internal/apperrors"
	"task-manager/internal/dto"
	"task-manager/internal/middleware"
	"task-manager/internal/models"
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
		RespondWithError(responseWriter, err)
		return
	}

	var input dto.CreateTaskDTO
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		RespondWithError(responseWriter, apperrors.BadRequest("malformed JSON body", err))
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	createdTask, err := handler.taskService.CreateTask(request.Context(), userID, input)
	if err != nil {
		RespondWithError(responseWriter, err)
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
		RespondWithError(responseWriter, err)
		return
	}

	var input dto.BatchCreateTaskDTO
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		RespondWithError(responseWriter, apperrors.BadRequest("malformed JSON body", err))
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	batchResponse, err := handler.taskService.BatchCreateTasks(request.Context(), userID, input)
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusCreated, batchResponse)
}

func (handler *TaskHandler) GetTaskByID(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	taskID, err := parseIDParam(request)
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	task, err := handler.taskService.GetTaskByID(request.Context(), userID, taskID)
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, task)
}

func (handler *TaskHandler) GetAllTasks(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	tasks, err := handler.taskService.GetAllTasks(request.Context(), userID)
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, tasks)
}

func (handler *TaskHandler) UpdateTask(responseWriter http.ResponseWriter, request *http.Request) {
	userID, err := middleware.GetUserIDFromContext(request.Context())
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	taskID, err := parseIDParam(request)
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	var input dto.UpdateTaskDTO
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		RespondWithError(responseWriter, apperrors.BadRequest("malformed JSON body", err))
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	updatedTask, err := handler.taskService.UpdateTask(request.Context(), userID, taskID, input)
	if err != nil {
		RespondWithError(responseWriter, err)
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
		RespondWithError(responseWriter, err)
		return
	}

	taskID, err := parseIDParam(request)
	if err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	if err := handler.taskService.DeleteTask(request.Context(), userID, taskID); err != nil {
		RespondWithError(responseWriter, err)
		return
	}

	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "task deleted successfully"})
}

// parseIDParam extracts and validates the "id" path parameter from the URL.
func parseIDParam(request *http.Request) (int64, error) {
	taskID, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil || taskID <= 0 {
		return 0, apperrors.BadRequest("invalid task id in path: must be a positive integer", err)
	}
	return taskID, nil
}
