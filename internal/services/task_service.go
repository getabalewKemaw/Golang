package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"task-manager/internal/dto"
	"task-manager/internal/models"
	"task-manager/internal/repositories"
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
}

type taskService struct {
	taskRepository repositories.TaskRepository
}

func NewTaskService(taskRepository repositories.TaskRepository) TaskService {
	return &taskService{
		taskRepository: taskRepository,
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

	tasks, err := service.taskRepository.GetAll(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service failed to fetch user tasks: %w", err)
	}

	if tasks == nil {
		tasks = []models.Task{}
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

	return nil
}
