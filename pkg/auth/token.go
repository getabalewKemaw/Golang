package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

var (
	ErrInvalidToken     = errors.New("invalid or malformed token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidTokenType = errors.New("unexpected token type")
)

// Claims defines custom JWT claims including user identification and token type.
type Claims struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	TokenType TokenType `json:"token_type"`
	jwt.RegisteredClaims
}

// TokenPair contains both access and refresh tokens returned to the client.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// GenerateTokenPair generates a fresh access token and refresh token for the user.
func GenerateTokenPair(userID int64, email string, accessSecret, refreshSecret string) (*TokenPair, error) {
	currentTime := time.Now()

	// 1. Generate short-lived Access Token
	accessExpirationTime := currentTime.Add(AccessTokenDuration)
	accessClaims := Claims{
		UserID:    userID,
		Email:     email,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(currentTime),
			ExpiresAt: jwt.NewNumericDate(accessExpirationTime),
		},
	}

	accessTokenObject := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessTokenObject.SignedString([]byte(accessSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 2. Generate long-lived Refresh Token
	refreshExpirationTime := currentTime.Add(RefreshTokenDuration)
	refreshClaims := Claims{
		UserID:    userID,
		Email:     email,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(currentTime),
			ExpiresAt: jwt.NewNumericDate(refreshExpirationTime),
		},
	}

	refreshTokenObject := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshTokenObject.SignedString([]byte(refreshSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(AccessTokenDuration.Seconds()),
	}, nil
}

// ValidateToken verifies token signature, expiration, and ensures matching token type.
func ValidateToken(tokenString string, secret string, expectedType TokenType) (*Claims, error) {
	parsedToken, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, isHMAC := token.Method.(*jwt.SigningMethodHMAC); !isHMAC {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok || !parsedToken.Valid {
		return nil, ErrInvalidToken
	}

	// Explicit expiration verification: ensures ExpiresAt is present and not past time.Now()
	if claims.ExpiresAt == nil || time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrExpiredToken
	}

	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrInvalidTokenType, expectedType, claims.TokenType)
	}

	return claims, nil
}

// HashToken calculates the SHA-256 hex digest of a token string for safe revocation storage.
func HashToken(tokenString string) string {
	hash := sha256.Sum256([]byte(tokenString))
	return hex.EncodeToString(hash[:])
}
