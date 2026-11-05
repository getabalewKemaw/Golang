package services

import (
	"context"
	"errors"
	"fmt"

	"task-manager/internal/dto"
	"task-manager/internal/models"
	"task-manager/internal/repositories"
	"task-manager/pkg/auth"
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
	jwtAccessSecret  string
	jwtRefreshSecret string
}

func NewAuthService(
	userRepository repositories.UserRepository,
	tokenRepository repositories.TokenRepository,
	jwtAccessSecret string,
	jwtRefreshSecret string,
) *AuthServiceImpl {
	return &AuthServiceImpl{
		userRepository:   userRepository,
		tokenRepository:  tokenRepository,
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

	createErr := service.userRepository.Create(ctx, &newUser)
	if createErr != nil {
		return nil, createErr
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

	passwordMatches := auth.CheckPasswordHash(loginInput.Password, user.PasswordHash)
	if !passwordMatches {
		return nil, ErrInvalidCredentials
	}

	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, service.jwtAccessSecret, service.jwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to issue tokens: %w", err)
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
	refreshTokenHash := auth.HashToken(refreshInput.RefreshToken)
	isRevoked, checkErr := service.tokenRepository.IsTokenRevoked(ctx, refreshTokenHash)
	if checkErr != nil {
		return nil, fmt.Errorf("failed to check token revocation: %w", checkErr)
	}
	if isRevoked {
		return nil, ErrTokenRevoked
	}

	claims, err := auth.ValidateToken(refreshInput.RefreshToken, service.jwtRefreshSecret, auth.TokenTypeRefresh)
	if err != nil {
		return nil, err
	}

	user, err := service.userRepository.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, auth.ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to verify user: %w", err)
	}

	newTokens, err := auth.GenerateTokenPair(user.ID, user.Email, service.jwtAccessSecret, service.jwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to issue refreshed tokens: %w", err)
	}

	return newTokens, nil
}

func (service *AuthServiceImpl) Logout(ctx context.Context, accessTokenString string, refreshTokenString string) error {
	if accessTokenString != "" {
		accessClaims, err := auth.ValidateToken(accessTokenString, service.jwtAccessSecret, auth.TokenTypeAccess)
		if err == nil && accessClaims.ExpiresAt != nil {
			accessTokenHash := auth.HashToken(accessTokenString)
			_ = service.tokenRepository.RevokeToken(ctx, accessTokenHash, string(auth.TokenTypeAccess), accessClaims.ExpiresAt.Time)
		}
	}

	if refreshTokenString != "" {
		refreshClaims, err := auth.ValidateToken(refreshTokenString, service.jwtRefreshSecret, auth.TokenTypeRefresh)
		if err == nil && refreshClaims.ExpiresAt != nil {
			refreshTokenHash := auth.HashToken(refreshTokenString)
			_ = service.tokenRepository.RevokeToken(ctx, refreshTokenHash, string(auth.TokenTypeRefresh), refreshClaims.ExpiresAt.Time)
		}
	}

	return nil
}
