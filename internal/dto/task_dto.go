package dto

import "task-manager/pkg/validator"

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
