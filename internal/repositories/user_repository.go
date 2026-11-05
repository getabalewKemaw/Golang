package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-manager/internal/models"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateEmail = errors.New("user with this email already exists")
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, userID int64) (*models.User, error)
}

type PostgresUserRepository struct {
	databasePool *sql.DB
}

func NewPostgresUserRepository(databasePool *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{
		databasePool: databasePool,
	}
}

func (repo *PostgresUserRepository) Create(ctx context.Context, user *models.User) error {
	insertQuery := `
		INSERT INTO users (email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at;`

	currentTime := time.Now()
	user.CreatedAt = currentTime
	user.UpdatedAt = currentTime

	err := repo.databasePool.QueryRowContext(
		ctx,
		insertQuery,
		user.Email,
		user.PasswordHash,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "users_email_key") {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (repo *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	selectQuery := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1;`

	var user models.User

	err := repo.databasePool.QueryRowContext(ctx, selectQuery, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}

	return &user, nil
}

func (repo *PostgresUserRepository) GetByID(ctx context.Context, userID int64) (*models.User, error) {
	selectQuery := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1;`

	var user models.User

	err := repo.databasePool.QueryRowContext(ctx, selectQuery, userID).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by id: %w", err)
	}

	return &user, nil
}
