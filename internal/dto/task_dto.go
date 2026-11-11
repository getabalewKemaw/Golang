package dto

import (
	"task-manager/internal/models"
	"task-manager/pkg/validator"
)

// CreateTaskDTO represents the payload when creating a new task.
type CreateTaskDTO struct {
	Title       string `json:"title" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	Status      string `json:"status" validate:"omitempty,oneof=pending in_progress completed"`
}

func (dto *CreateTaskDTO) Validate() map[string]string {
	return validator.ValidateStruct(dto)
}

type UpdateTaskDTO struct {
	Title       *string `json:"title,omitempty" validate:"omitempty,min=1,max=100"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	Status      *string `json:"status,omitempty" validate:"omitempty,oneof=pending in_progress completed"`
}

func (dto *UpdateTaskDTO) Validate() map[string]string {
	return validator.ValidateStruct(dto)
}

// BatchCreateTaskDTO represents the incoming payload for creating multiple tasks concurrently.
type BatchCreateTaskDTO struct {
	Tasks []CreateTaskDTO `json:"tasks" validate:"required,min=1,max=100,dive"`
}

func (dto *BatchCreateTaskDTO) Validate() map[string]string {
	return validator.ValidateStruct(dto)
}

// BatchItemResult represents the individual result of a single task in a batch operation.
type BatchItemResult struct {
	Index   int          `json:"index"`
	Success bool         `json:"success"`
	Task    *models.Task `json:"task,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// BatchCreateResponseDTO aggregates the results of concurrent batch task creation.
type BatchCreateResponseDTO struct {
	TotalProcessed int               `json:"total_processed"`
	SuccessCount   int               `json:"success_count"`
	FailureCount   int               `json:"failure_count"`
	Results        []BatchItemResult `json:"results"`
}
