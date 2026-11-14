package apperrors

import (
	"fmt"
	"net/http"
)

// AppError is a custom error struct carrying HTTP metadata and root causes.
type AppError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Internal   error  `json:"-"`
}

// Error satisfies the standard library error interface.
func (appError *AppError) Error() string {
	if appError.Internal != nil {
		return fmt.Sprintf("%s: %v", appError.Message, appError.Internal)
	}
	return appError.Message
}

// Unwrap allows standard library errors.Is and errors.As to inspect appError.Internal.
func (appError *AppError) Unwrap() error {
	return appError.Internal
}

func New(statusCode int, message string, internal error) *AppError {
	return &AppError{
		StatusCode: statusCode,
		Message:    message,
		Internal:   internal,
	}
}

func NotFound(message string, internal error) *AppError {
	return &AppError{
		StatusCode: http.StatusNotFound,
		Message:    message,
		Internal:   internal,
	}
}

func BadRequest(message string, internal error) *AppError {
	return &AppError{
		StatusCode: http.StatusBadRequest,
		Message:    message,
		Internal:   internal,
	}
}

func Unauthorized(message string, internal error) *AppError {
	return &AppError{
		StatusCode: http.StatusUnauthorized,
		Message:    message,
		Internal:   internal,
	}
}

func Forbidden(message string, internal error) *AppError {
	return &AppError{
		StatusCode: http.StatusForbidden,
		Message:    message,
		Internal:   internal,
	}
}

func Conflict(message string, internal error) *AppError {
	return &AppError{
		StatusCode: http.StatusConflict,
		Message:    message,
		Internal:   internal,
	}
}

func Internal(internal error) *AppError {
	return &AppError{
		StatusCode: http.StatusInternalServerError,
		Message:    "internal server error",
		Internal:   internal,
	}
}
