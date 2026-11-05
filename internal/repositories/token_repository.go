package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type TokenRepository interface {
	RevokeToken(ctx context.Context, tokenHash string, tokenType string, expiresAt time.Time) error
	IsTokenRevoked(ctx context.Context, tokenHash string) (bool, error)
}

type PostgresTokenRepository struct {
	databasePool *sql.DB
}

func NewPostgresTokenRepository(databasePool *sql.DB) *PostgresTokenRepository {
	return &PostgresTokenRepository{
		databasePool: databasePool,
	}
}

func (repo *PostgresTokenRepository) RevokeToken(ctx context.Context, tokenHash string, tokenType string, expiresAt time.Time) error {
	insertQuery := `
		INSERT INTO revoked_tokens (token_hash, token_type, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_hash) DO NOTHING;`

	_, err := repo.databasePool.ExecContext(ctx, insertQuery, tokenHash, tokenType, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	return nil
}

func (repo *PostgresTokenRepository) IsTokenRevoked(ctx context.Context, tokenHash string) (bool, error) {
	checkQuery := `
		SELECT EXISTS (
			SELECT 1 FROM revoked_tokens
			WHERE token_hash = $1 AND expires_at > NOW()
		);`

	var isRevoked bool
	err := repo.databasePool.QueryRowContext(ctx, checkQuery, tokenHash).Scan(&isRevoked)
	if err != nil {
		return false, fmt.Errorf("failed to verify token revocation status: %w", err)
	}

	return isRevoked, nil
}
