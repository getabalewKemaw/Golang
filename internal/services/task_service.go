package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-manager/internal/dto"
	"task-manager/internal/models"
	"task-manager/internal/repositories"
	"task-manager/pkg/cache"
)

var (
	ErrInvalidTaskID = errors.New("invalid task id: must be greater than zero")
	ErrInvalidUserID = errors.New("invalid user id: must be greater than zero")
)

type TaskService interface {
	CreateTask(ctx context.Context, userID int64, input dto.CreateTaskDTO) (*models.Task, error)
	GetTaskByID(ctx context.Context, userID, taskID int64) (*models.Task, error)
	GetAllTasks(ctx context.Context, userID int64) ([]models.Task, error)
	UpdateTask(ctx context.Context, userID, taskID int64, input dto.UpdateTaskDTO) (*models.Task, error)
	DeleteTask(ctx context.Context, userID, taskID int64) error
	BatchCreateTasks(ctx context.Context, userID int64, input dto.BatchCreateTaskDTO) (*dto.BatchCreateResponseDTO, error)
}

type taskService struct {
	taskRepository repositories.TaskRepository
	cacheService   cache.CacheService
}

var _ TaskService = (*taskService)(nil)

func NewTaskService(taskRepository repositories.TaskRepository, cacheService cache.CacheService) TaskService {
	if cacheService == nil {
		cacheService = cache.NewNoOpCache()
	}
	return &taskService{
		taskRepository: taskRepository,
		cacheService:   cacheService,
	}
}

func (service *taskService) CreateTask(ctx context.Context, userID int64, input dto.CreateTaskDTO) (*models.Task, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}

	taskStatus := models.StatusPending
	if strings.TrimSpace(input.Status) != "" {
		taskStatus = models.TaskStatus(input.Status)
	}

	task := &models.Task{
		UserID:      userID,
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		Status:      taskStatus,
	}

	if err := service.taskRepository.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("service failed to create task: %w", err)
	}

	// Invalidate user tasks cache on new creation
	_ = service.cacheService.Delete(ctx, fmt.Sprintf("tasks:user:%d", userID))

	return task, nil
}

func (service *taskService) GetTaskByID(ctx context.Context, userID, taskID int64) (*models.Task, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	if taskID <= 0 {
		return nil, ErrInvalidTaskID
	}

	task, err := service.taskRepository.GetByID(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (service *taskService) GetAllTasks(ctx context.Context, userID int64) ([]models.Task, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}

	cacheKey := fmt.Sprintf("tasks:user:%d", userID)

	// 1. Cache-Aside Pattern: Check if user task list exists in cache
	if cachedData, err := service.cacheService.Get(ctx, cacheKey); err == nil && len(cachedData) > 0 {
		var cachedTasks []models.Task
		if unmarshalErr := json.Unmarshal(cachedData, &cachedTasks); unmarshalErr == nil {
			return cachedTasks, nil
		}
	}

	// 2. Cache Miss: Query PostgreSQL database
	tasks, err := service.taskRepository.GetAll(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service failed to fetch user tasks: %w", err)
	}

	if tasks == nil {
		tasks = []models.Task{}
	}

	// 3. Store result in Cache with 5-minute TTL
	if encodedBytes, marshalErr := json.Marshal(tasks); marshalErr == nil {
		_ = service.cacheService.Set(ctx, cacheKey, encodedBytes, 5*time.Minute)
	}

	return tasks, nil
}

func (service *taskService) UpdateTask(ctx context.Context, userID, taskID int64, input dto.UpdateTaskDTO) (*models.Task, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	if taskID <= 0 {
		return nil, ErrInvalidTaskID
	}

	existingTask, err := service.taskRepository.GetByID(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}

	// Optimistic concurrency check: if the client provided a version, verify it matches
	if input.Version != nil && *input.Version != existingTask.Version {
		return nil, repositories.ErrOptimisticLockConflict
	}

	if input.Title != nil {
		existingTask.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		existingTask.Description = strings.TrimSpace(*input.Description)
	}
	if input.Status != nil {
		existingTask.Status = models.TaskStatus(*input.Status)
	}

	if err := service.taskRepository.Update(ctx, existingTask); err != nil {
		return nil, fmt.Errorf("service failed to update task: %w", err)
	}

	// Invalidate user tasks cache on update
	_ = service.cacheService.Delete(ctx, fmt.Sprintf("tasks:user:%d", userID))

	return existingTask, nil
}

func (service *taskService) DeleteTask(ctx context.Context, userID, taskID int64) error {
	if userID <= 0 {
		return ErrInvalidUserID
	}
	if taskID <= 0 {
		return ErrInvalidTaskID
	}

	if err := service.taskRepository.Delete(ctx, userID, taskID); err != nil {
		return err
	}

	// Invalidate user tasks cache on deletion
	_ = service.cacheService.Delete(ctx, fmt.Sprintf("tasks:user:%d", userID))

	return nil
}

type batchTaskJob struct {
	index int
	data  dto.CreateTaskDTO
}

type batchTaskOutcome struct {
	index int
	task  *models.Task
	err   error
}

func (service *taskService) BatchCreateTasks(
	ctx context.Context,
	userID int64,
	input dto.BatchCreateTaskDTO,
) (*dto.BatchCreateResponseDTO, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}

	totalTasks := len(input.Tasks)
	if totalTasks == 0 {
		return &dto.BatchCreateResponseDTO{
			TotalProcessed: 0,
			SuccessCount:   0,
			FailureCount:   0,
			Results:        []dto.BatchItemResult{},
		}, nil
	}

	workerCount := 5
	if totalTasks < workerCount {
		workerCount = totalTasks
	}

	jobsChannel := make(chan batchTaskJob, totalTasks)
	resultsChannel := make(chan batchTaskOutcome, totalTasks)

	for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
		go func() {
			for job := range jobsChannel {
				if ctx.Err() != nil {
					resultsChannel <- batchTaskOutcome{
						index: job.index,
						task:  nil,
						err:   ctx.Err(),
					}
					continue
				}

				createdTask, err := service.CreateTask(ctx, userID, job.data)
				resultsChannel <- batchTaskOutcome{
					index: job.index,
					task:  createdTask,
					err:   err,
				}
			}
		}()
	}

	go func() {
		defer close(jobsChannel)
		for index, taskData := range input.Tasks {
			select {
			case <-ctx.Done():
				return
			case jobsChannel <- batchTaskJob{index: index, data: taskData}:
			}
		}
	}()

	results := make([]dto.BatchItemResult, totalTasks)
	successCount := 0
	failureCount := 0

	for collectedCount := 0; collectedCount < totalTasks; collectedCount++ {
		select {
		case outcome := <-resultsChannel:
			itemResult := dto.BatchItemResult{
				Index: outcome.index,
			}

			if outcome.err != nil {
				itemResult.Success = false
				itemResult.Error = outcome.err.Error()
				failureCount++
			} else {
				itemResult.Success = true
				itemResult.Task = outcome.task
				successCount++
			}

			results[outcome.index] = itemResult

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	_ = service.cacheService.Delete(ctx, fmt.Sprintf("tasks:user:%d", userID))

	return &dto.BatchCreateResponseDTO{
		TotalProcessed: totalTasks,
		SuccessCount:   successCount,
		FailureCount:   failureCount,
		Results:        results,
	}, nil
}
