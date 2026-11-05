package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"task-manager/internal/repositories"
	"task-manager/pkg/auth"
)

type contextKey string

const (
	UserIDContextKey    contextKey = "authenticatedUserID"
	UserEmailContextKey contextKey = "authenticatedUserEmail"
)

var (
	ErrUserNotAuthenticated = errors.New("unauthorized: missing or invalid user id in context")
)

func GetUserIDFromContext(ctx context.Context) (int64, error) {
	userID, ok := ctx.Value(UserIDContextKey).(int64)
	if !ok || userID <= 0 {
		return 0, ErrUserNotAuthenticated
	}
	return userID, nil
}

func GetUserEmailFromContext(ctx context.Context) (string, error) {
	userEmail, ok := ctx.Value(UserEmailContextKey).(string)
	if !ok || userEmail == "" {
		return "", ErrUserNotAuthenticated
	}
	return userEmail, nil
}

// RequireAuth enforces valid JWT access token authentication on protected endpoints
// and verifies that the presented token has not been revoked on logout.
func RequireAuth(jwtAccessSecret string, tokenRepository repositories.TokenRepository) func(http.Handler) http.Handler {
	return func(nextHandler http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			authorizationHeader := request.Header.Get("Authorization")
			if authorizationHeader == "" {
				writeUnauthorized(responseWriter, "missing authorization header")
				return
			}

			headerParts := strings.SplitN(authorizationHeader, " ", 2)
			if len(headerParts) != 2 || !strings.EqualFold(headerParts[0], "Bearer") {
				writeUnauthorized(responseWriter, "invalid authorization header format: must be 'Bearer <token>'")
				return
			}

			tokenString := strings.TrimSpace(headerParts[1])
			claims, err := auth.ValidateToken(tokenString, jwtAccessSecret, auth.TokenTypeAccess)
			if err != nil {
				writeUnauthorized(responseWriter, "invalid or expired access token")
				return
			}

			if tokenRepository != nil {
				tokenHash := auth.HashToken(tokenString)
				isRevoked, checkErr := tokenRepository.IsTokenRevoked(request.Context(), tokenHash)
				if checkErr == nil && isRevoked {
					writeUnauthorized(responseWriter, "token has been revoked")
					return
				}
			}

			requestContext := context.WithValue(request.Context(), UserIDContextKey, claims.UserID)
			requestContext = context.WithValue(requestContext, UserEmailContextKey, claims.Email)

			nextHandler.ServeHTTP(responseWriter, request.WithContext(requestContext))
		})
	}
}

func writeUnauthorized(responseWriter http.ResponseWriter, errorMessage string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(responseWriter).Encode(map[string]string{"error": errorMessage})
}
