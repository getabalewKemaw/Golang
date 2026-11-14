package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-manager/internal/dto"
	"task-manager/internal/models"
	"task-manager/internal/repositories"
	"task-manager/internal/services"
)

// mockTaskRepository implements repositories.TaskRepository using first-class function fields.
type mockTaskRepository struct {
	createFunc  func(ctx context.Context, task *models.Task) error
	getByIDFunc func(ctx context.Context, userID, taskID int64) (*models.Task, error)
	getAllFunc  func(ctx context.Context, userID int64) ([]models.Task, error)
	updateFunc  func(ctx context.Context, task *models.Task) error
	deleteFunc  func(ctx context.Context, userID, taskID int64) error
}

var _ repositories.TaskRepository = (*mockTaskRepository)(nil)

func (mock *mockTaskRepository) Create(ctx context.Context, task *models.Task) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, task)
	}
	return nil
}

func (mock *mockTaskRepository) GetByID(ctx context.Context, userID, taskID int64) (*models.Task, error) {
	if mock.getByIDFunc != nil {
		return mock.getByIDFunc(ctx, userID, taskID)
	}
	return nil, nil
}

func (mock *mockTaskRepository) GetAll(ctx context.Context, userID int64) ([]models.Task, error) {
	if mock.getAllFunc != nil {
		return mock.getAllFunc(ctx, userID)
	}
	return nil, nil
}

func (mock *mockTaskRepository) Update(ctx context.Context, task *models.Task) error {
	if mock.updateFunc != nil {
		return mock.updateFunc(ctx, task)
	}
	return nil
}

func (mock *mockTaskRepository) Delete(ctx context.Context, userID, taskID int64) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, userID, taskID)
	}
	return nil
}

func TestGetTaskByID_ValidationErrors(testingT *testing.T) {
	mockRepository := &mockTaskRepository{}
	taskService := services.NewTaskService(mockRepository)
	testContext := context.Background()

	// 1. Invalid User ID
	_, err := taskService.GetTaskByID(testContext, 0, 10)
	if !errors.Is(err, services.ErrInvalidUserID) {
		testingT.Fatalf("expected ErrInvalidUserID, got: %v", err)
	}

	// 2. Invalid Task ID
	_, err = taskService.GetTaskByID(testContext, 1, 0)
	if !errors.Is(err, services.ErrInvalidTaskID) {
		testingT.Fatalf("expected ErrInvalidTaskID, got: %v", err)
	}
}

func TestGetTaskByID_Success(testingT *testing.T) {
	expectedTask := &models.Task{
		ID:          42,
		UserID:      1,
		Title:       "Learn Go Interfaces",
		Description: "Master consumer-defined interfaces and zero-dependency mocks",
		Status:      models.StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockRepository := &mockTaskRepository{
		getByIDFunc: func(ctx context.Context, userID, taskID int64) (*models.Task, error) {
			if userID == 1 && taskID == 42 {
				return expectedTask, nil
			}
			return nil, repositories.ErrTaskNotFound
		},
	}

	taskService := services.NewTaskService(mockRepository)
	actualTask, err := taskService.GetTaskByID(context.Background(), 1, 42)
	if err != nil {
		testingT.Fatalf("unexpected error fetching task: %v", err)
	}

	if actualTask.ID != expectedTask.ID {
		testingT.Errorf("expected task ID %d, got %d", expectedTask.ID, actualTask.ID)
	}
	if actualTask.Title != expectedTask.Title {
		testingT.Errorf("expected title '%s', got '%s'", expectedTask.Title, actualTask.Title)
	}
}

func TestGetTaskByID_NotFound(testingT *testing.T) {
	mockRepository := &mockTaskRepository{
		getByIDFunc: func(ctx context.Context, userID, taskID int64) (*models.Task, error) {
			return nil, repositories.ErrTaskNotFound
		},
	}

	taskService := services.NewTaskService(mockRepository)
	_, err := taskService.GetTaskByID(context.Background(), 1, 999)
	if err == nil {
		testingT.Fatal("expected error, got nil")
	}

	if !errors.Is(err, repositories.ErrTaskNotFound) {
		testingT.Fatalf("expected repositories.ErrTaskNotFound, got: %v", err)
	}
}

func TestCreateTask_Success(testingT *testing.T) {
	mockRepository := &mockTaskRepository{
		createFunc: func(ctx context.Context, task *models.Task) error {
			task.ID = 101
			return nil
		},
	}

	taskService := services.NewTaskService(mockRepository)
	input := dto.CreateTaskDTO{
		Title:       "Ship Production API",
		Description: "With clean interfaces and typed errors",
		Status:      "pending",
	}

	createdTask, err := taskService.CreateTask(context.Background(), 1, input)
	if err != nil {
		testingT.Fatalf("unexpected error creating task: %v", err)
	}

	if createdTask.ID != 101 {
		testingT.Errorf("expected generated ID 101, got %d", createdTask.ID)
	}
	if createdTask.Title != input.Title {
		testingT.Errorf("expected title '%s', got '%s'", input.Title, createdTask.Title)
	}
}

func TestBatchCreateTasks_ContextCanceled(testingT *testing.T) {
	mockRepository := &mockTaskRepository{}
	taskService := services.NewTaskService(mockRepository)

	canceledContext, cancelFunc := context.WithCancel(context.Background())
	cancelFunc() // Cancel immediately

	input := dto.BatchCreateTaskDTO{
		Tasks: []dto.CreateTaskDTO{
			{Title: "Task 1", Description: "Desc 1"},
		},
	}

	_, err := taskService.BatchCreateTasks(canceledContext, 1, input)
	if err == nil {
		testingT.Fatal("expected context canceled error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		testingT.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestUpdateTask_OptimisticLockConflict(testingT *testing.T) {
	currentTask := &models.Task{
		ID:        50,
		UserID:    1,
		Title:     "Old Title",
		Status:    models.StatusPending,
		Version:   2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepository := &mockTaskRepository{
		getByIDFunc: func(ctx context.Context, userID, taskID int64) (*models.Task, error) {
			return currentTask, nil
		},
	}

	taskService := services.NewTaskService(mockRepository)
	staleVersion := int64(1)
	newTitle := "New Title"
	input := dto.UpdateTaskDTO{
		Title:   &newTitle,
		Version: &staleVersion,
	}

	_, err := taskService.UpdateTask(context.Background(), 1, 50, input)
	if err == nil {
		testingT.Fatal("expected optimistic lock conflict, got nil")
	}

	if !errors.Is(err, repositories.ErrOptimisticLockConflict) {
		testingT.Fatalf("expected ErrOptimisticLockConflict, got: %v", err)
	}
}

func TestUpdateTask_Success(testingT *testing.T) {
	currentTask := &models.Task{
		ID:        50,
		UserID:    1,
		Title:     "Old Title",
		Status:    models.StatusPending,
		Version:   2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepository := &mockTaskRepository{
		getByIDFunc: func(ctx context.Context, userID, taskID int64) (*models.Task, error) {
			return currentTask, nil
		},
		updateFunc: func(ctx context.Context, task *models.Task) error {
			task.Version = 3
			return nil
		},
	}

	taskService := services.NewTaskService(mockRepository)
	matchingVersion := int64(2)
	newTitle := "Updated Title"
	input := dto.UpdateTaskDTO{
		Title:   &newTitle,
		Version: &matchingVersion,
	}

	updatedTask, err := taskService.UpdateTask(context.Background(), 1, 50, input)
	if err != nil {
		testingT.Fatalf("unexpected error updating task: %v", err)
	}

	if updatedTask.Title != "Updated Title" {
		testingT.Errorf("expected updated title 'Updated Title', got: %s", updatedTask.Title)
	}
	if updatedTask.Version != 3 {
		testingT.Errorf("expected incremented version 3, got: %d", updatedTask.Version)
	}
}

