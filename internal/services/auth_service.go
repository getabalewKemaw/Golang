package services

import (
	"context"
	"errors"
	"fmt"

	"task-manager/internal/dto"
	"task-manager/internal/models"
	"task-manager/internal/repositories"
	"task-manager/pkg/auth"
	"task-manager/pkg/database"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrTokenRevoked       = errors.New("token has been revoked")
)

type AuthService interface {
	Register(ctx context.Context, registrationInput dto.RegisterRequestDTO) (*dto.AuthResponseDTO, error)
	Login(ctx context.Context, loginInput dto.LoginRequestDTO) (*dto.AuthResponseDTO, error)
	RefreshToken(ctx context.Context, refreshInput dto.RefreshTokenRequestDTO) (*auth.TokenPair, error)
	Logout(ctx context.Context, accessTokenString string, refreshTokenString string) error
}

type AuthServiceImpl struct {
	userRepository   repositories.UserRepository
	tokenRepository  repositories.TokenRepository
	taskRepository   repositories.TaskRepository
	transactor       database.Transactor
	jwtAccessSecret  string
	jwtRefreshSecret string
}

var _ AuthService = (*AuthServiceImpl)(nil)

func NewAuthService(
	userRepository repositories.UserRepository,
	tokenRepository repositories.TokenRepository,
	taskRepository repositories.TaskRepository,
	transactor database.Transactor,
	jwtAccessSecret string,
	jwtRefreshSecret string,
) *AuthServiceImpl {
	return &AuthServiceImpl{
		userRepository:   userRepository,
		tokenRepository:  tokenRepository,
		taskRepository:   taskRepository,
		transactor:       transactor,
		jwtAccessSecret:  jwtAccessSecret,
		jwtRefreshSecret: jwtRefreshSecret,
	}
}

func (service *AuthServiceImpl) Register(ctx context.Context, registrationInput dto.RegisterRequestDTO) (*dto.AuthResponseDTO, error) {
	hashedPassword, err := auth.HashPassword(registrationInput.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := models.User{
		Email:        registrationInput.Email,
		PasswordHash: hashedPassword,
	}

	// Atomically create the user and their initial onboarding tasks inside a single transaction
	transactionErr := service.transactor.WithinTransaction(ctx, func(transactionContext context.Context) error {
		if err := service.userRepository.Create(transactionContext, &newUser); err != nil {
			return err
		}

		welcomeTask1 := models.Task{
			UserID:      newUser.ID,
			Title:       "Welcome to Task Manager!",
			Description: "Explore the API documentation and get started.",
			Status:      models.StatusPending,
		}
		if err := service.taskRepository.Create(transactionContext, &welcomeTask1); err != nil {
			return fmt.Errorf("failed to create welcome task: %w", err)
		}

		welcomeTask2 := models.Task{
			UserID:      newUser.ID,
			Title:       "Try the Batch API",
			Description: "Create multiple tasks concurrently using POST /api/v1/tasks/batch.",
			Status:      models.StatusPending,
		}
		if err := service.taskRepository.Create(transactionContext, &welcomeTask2); err != nil {
			return fmt.Errorf("failed to create batch intro task: %w", err)
		}

		return nil
	})

	if transactionErr != nil {
		return nil, transactionErr
	}

	tokens, err := auth.GenerateTokenPair(newUser.ID, newUser.Email, service.jwtAccessSecret, service.jwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to issue tokens: %w", err)
	}

	return &dto.AuthResponseDTO{
		User: dto.UserResponseDTO{
			ID:        newUser.ID,
			Email:     newUser.Email,
			CreatedAt: newUser.CreatedAt,
		},
		Tokens: *tokens,
	}, nil
}

func (service *AuthServiceImpl) Login(ctx context.Context, loginInput dto.LoginRequestDTO) (*dto.AuthResponseDTO, error) {
	user, err := service.userRepository.GetByEmail(ctx, loginInput.Email)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to retrieve user: %w", err)
	}

	isPasswordValid := auth.CheckPasswordHash(loginInput.Password, user.PasswordHash)
	if !isPasswordValid {
		return nil, ErrInvalidCredentials
	}

	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, service.jwtAccessSecret, service.jwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &dto.AuthResponseDTO{
		User: dto.UserResponseDTO{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
		Tokens: *tokens,
	}, nil
}

func (service *AuthServiceImpl) RefreshToken(ctx context.Context, refreshInput dto.RefreshTokenRequestDTO) (*auth.TokenPair, error) {
	claims, err := auth.ValidateToken(refreshInput.RefreshToken, service.jwtRefreshSecret, auth.TokenTypeRefresh)
	if err != nil {
		return nil, err
	}

	refreshTokenHash := auth.HashToken(refreshInput.RefreshToken)
	isRevoked, err := service.tokenRepository.IsTokenRevoked(ctx, refreshTokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to check token revocation: %w", err)
	}
	if isRevoked {
		return nil, ErrTokenRevoked
	}

	newTokens, err := auth.GenerateTokenPair(claims.UserID, claims.Email, service.jwtAccessSecret, service.jwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new token pair: %w", err)
	}

	revokeErr := service.tokenRepository.RevokeToken(ctx, refreshTokenHash, string(auth.TokenTypeRefresh), claims.ExpiresAt.Time)
	if revokeErr != nil {
		return nil, fmt.Errorf("failed to rotate old refresh token: %w", revokeErr)
	}

	return newTokens, nil
}

func (service *AuthServiceImpl) Logout(ctx context.Context, accessTokenString string, refreshTokenString string) error {
	var collectedErrors []error

	if accessTokenString != "" {
		claims, err := auth.ValidateToken(accessTokenString, service.jwtAccessSecret, auth.TokenTypeAccess)
		if err == nil {
			tokenHash := auth.HashToken(accessTokenString)
			if revokeErr := service.tokenRepository.RevokeToken(ctx, tokenHash, string(auth.TokenTypeAccess), claims.ExpiresAt.Time); revokeErr != nil {
				collectedErrors = append(collectedErrors, fmt.Errorf("failed to revoke access token: %w", revokeErr))
			}
		}
	}

	if refreshTokenString != "" {
		claims, err := auth.ValidateToken(refreshTokenString, service.jwtRefreshSecret, auth.TokenTypeRefresh)
		if err == nil {
			tokenHash := auth.HashToken(refreshTokenString)
			if revokeErr := service.tokenRepository.RevokeToken(ctx, tokenHash, string(auth.TokenTypeRefresh), claims.ExpiresAt.Time); revokeErr != nil {
				collectedErrors = append(collectedErrors, fmt.Errorf("failed to revoke refresh token: %w", revokeErr))
			}
		}
	}

	if len(collectedErrors) > 0 {
		return errors.Join(collectedErrors...)
	}

	return nil
}
