package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"task-manager/internal/apperrors"
	"task-manager/internal/middleware"
	"task-manager/internal/repositories"
	"task-manager/internal/services"
	"task-manager/pkg/auth"
)

func writeJSON(responseWriter http.ResponseWriter, statusCode int, data any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(data)
}

func writeErrorJSON(responseWriter http.ResponseWriter, statusCode int, message string) {
	requestID := responseWriter.Header().Get(middleware.RequestIDHeader)
	payload := map[string]any{
		"error": message,
	}
	if requestID != "" {
		payload["request_id"] = requestID
	}
	writeJSON(responseWriter, statusCode, payload)
}

// RespondWithError translates domain, sentinel, and custom typed errors into standard JSON HTTP responses.
func RespondWithError(responseWriter http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	requestID := responseWriter.Header().Get(middleware.RequestIDHeader)

	var appError *apperrors.AppError
	if errors.As(err, &appError) {
		if appError.StatusCode >= http.StatusInternalServerError && appError.Internal != nil {
			slog.Error("application error encountered",
				slog.String("request_id", requestID),
				slog.Int("status", appError.StatusCode),
				slog.Any("error", appError.Internal),
			)
		}
		writeErrorJSON(responseWriter, appError.StatusCode, appError.Message)
		return
	}

	if errors.Is(err, context.DeadlineExceeded) {
		writeErrorJSON(responseWriter, http.StatusGatewayTimeout, "request timed out")
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}

	switch {
	case errors.Is(err, repositories.ErrTaskNotFound), errors.Is(err, repositories.ErrUserNotFound):
		writeErrorJSON(responseWriter, http.StatusNotFound, err.Error())
		return

	case errors.Is(err, repositories.ErrDuplicateEmail), errors.Is(err, repositories.ErrOptimisticLockConflict):
		writeErrorJSON(responseWriter, http.StatusConflict, err.Error())
		return

	case errors.Is(err, services.ErrInvalidCredentials),
		errors.Is(err, services.ErrTokenRevoked),
		errors.Is(err, auth.ErrInvalidToken),
		errors.Is(err, auth.ErrExpiredToken),
		errors.Is(err, auth.ErrInvalidTokenType),
		errors.Is(err, middleware.ErrUserNotAuthenticated):
		writeErrorJSON(responseWriter, http.StatusUnauthorized, err.Error())
		return

	case errors.Is(err, services.ErrInvalidTaskID), errors.Is(err, services.ErrInvalidUserID):
		writeErrorJSON(responseWriter, http.StatusBadRequest, err.Error())
		return
	}

	slog.Error("unhandled internal error",
		slog.String("request_id", requestID),
		slog.Any("error", err),
	)
	writeErrorJSON(responseWriter, http.StatusInternalServerError, "internal server error")
}
