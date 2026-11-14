package dto

import (
	"task-manager/pkg/auth"
	"task-manager/pkg/validator"
	"time"
)

type RegisterRequestDTO struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

func (dto *RegisterRequestDTO) Validate() map[string]string {
	return validator.ValidateStruct(dto)
}

type LoginRequestDTO struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (dto *LoginRequestDTO) Validate() map[string]string {
	return validator.ValidateStruct(dto)
}

type RefreshTokenRequestDTO struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (dto *RefreshTokenRequestDTO) Validate() map[string]string {
	return validator.ValidateStruct(dto)
}

// LogoutRequestDTO represents optional parameters when logging out (e.g. revoking the refresh token).
type LogoutRequestDTO struct {
	RefreshToken string `json:"refresh_token" validate:"omitempty"`
}

func (dto *LogoutRequestDTO) Validate() map[string]string {
	return validator.ValidateStruct(dto)
}

// UserResponseDTO represents safe user data exposed in HTTP responses.
type UserResponseDTO struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthResponseDTO is returned upon successful registration or login.
type AuthResponseDTO struct {
	User   UserResponseDTO `json:"user"`
	Tokens auth.TokenPair  `json:"tokens"`
}
