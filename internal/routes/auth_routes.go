package routes

import (
	"net/http"

	"task-manager/internal/handlers"
)

func RegisterAuthRoutes(router *http.ServeMux, authHandler *handlers.AuthHandler) {
	router.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	router.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	router.HandleFunc("POST /api/v1/auth/refresh", authHandler.RefreshToken)
	router.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
}
