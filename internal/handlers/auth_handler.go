package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"task-manager/internal/dto"
	"task-manager/internal/repositories"
	"task-manager/internal/services"
	"task-manager/pkg/auth"
)

type AuthHandler struct {
	authService services.AuthService
}

func NewAuthHandler(authService services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (handler *AuthHandler) Register(responseWriter http.ResponseWriter, request *http.Request) {
	var input dto.RegisterRequestDTO

	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": "malformed JSON body"})
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	authResponse, err := handler.authService.Register(request.Context(), input)
	if err != nil {
		if errors.Is(err, repositories.ErrDuplicateEmail) {
			writeJSON(responseWriter, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusCreated, authResponse)
}

func (handler *AuthHandler) Login(responseWriter http.ResponseWriter, request *http.Request) {
	var input dto.LoginRequestDTO

	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": "malformed JSON body"})
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	authResponse, err := handler.authService.Login(request.Context(), input)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusOK, authResponse)
}

func (handler *AuthHandler) RefreshToken(responseWriter http.ResponseWriter, request *http.Request) {
	var input dto.RefreshTokenRequestDTO

	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": "malformed JSON body"})
		return
	}

	if validationErrors := input.Validate(); validationErrors != nil {
		writeJSON(responseWriter, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}

	newTokens, err := handler.authService.RefreshToken(request.Context(), input)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) || errors.Is(err, auth.ErrExpiredToken) || errors.Is(err, auth.ErrInvalidTokenType) || errors.Is(err, services.ErrTokenRevoked) {
			writeJSON(responseWriter, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusOK, newTokens)
}

func (handler *AuthHandler) Logout(responseWriter http.ResponseWriter, request *http.Request) {
	var accessTokenString string
	authorizationHeader := request.Header.Get("Authorization")
	if authorizationHeader != "" {
		headerParts := strings.SplitN(authorizationHeader, " ", 2)
		if len(headerParts) == 2 && strings.EqualFold(headerParts[0], "Bearer") {
			accessTokenString = strings.TrimSpace(headerParts[1])
		}
	}

	var input dto.LogoutRequestDTO
	if request.Body != nil {
		_ = json.NewDecoder(request.Body).Decode(&input)
	}

	if accessTokenString == "" && input.RefreshToken == "" {
		writeJSON(responseWriter, http.StatusBadRequest, map[string]string{
			"error": "provide an access token in the Authorization header or a refresh_token in the request body",
		})
		return
	}

	err := handler.authService.Logout(request.Context(), accessTokenString, input.RefreshToken)
	if err != nil {
		writeJSON(responseWriter, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(responseWriter, http.StatusOK, map[string]string{
		"message": "successfully logged out and invalidated token(s)",
	})
}
