package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task-manager/internal/models"
	"task-manager/pkg/database"
)

var (
	ErrTaskNotFound             = errors.New("task not found")
	ErrOptimisticLockConflict   = errors.New("task was modified by another transaction; reload and try again")
)

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	GetByID(ctx context.Context, userID, taskID int64) (*models.Task, error)
	GetAll(ctx context.Context, userID int64) ([]models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	Delete(ctx context.Context, userID, taskID int64) error
}

type PostgresTaskRepository struct {
	databasePool *sql.DB
}

var _ TaskRepository = (*PostgresTaskRepository)(nil)

func NewPostgresTaskRepository(databasePool *sql.DB) *PostgresTaskRepository {
	return &PostgresTaskRepository{
		databasePool: databasePool,
	}
}

func (repo *PostgresTaskRepository) Create(ctx context.Context, task *models.Task) error {
	insertQuery := `
		INSERT INTO tasks (user_id, title, description, status, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, $6)
		RETURNING id, version, created_at, updated_at;`

	currentTime := time.Now()
	task.CreatedAt = currentTime
	task.UpdatedAt = currentTime

	executor := database.GetExecutor(ctx, repo.databasePool)
	err := executor.QueryRowContext(
		ctx,
		insertQuery,
		task.UserID,
		task.Title,
		task.Description,
		task.Status,
		task.CreatedAt,
		task.UpdatedAt,
	).Scan(&task.ID, &task.Version, &task.CreatedAt, &task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert task: %w", err)
	}

	return nil
}

func (repo *PostgresTaskRepository) GetByID(ctx context.Context, userID, taskID int64) (*models.Task, error) {
	selectQuery := `
		SELECT id, user_id, title, description, status, version, created_at, updated_at
		FROM tasks
		WHERE id = $1 AND user_id = $2;`

	var task models.Task

	executor := database.GetExecutor(ctx, repo.databasePool)
	err := executor.QueryRowContext(ctx, selectQuery, taskID, userID).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.Version,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to query task by id: %w", err)
	}

	return &task, nil
}

func (repo *PostgresTaskRepository) GetAll(ctx context.Context, userID int64) ([]models.Task, error) {
	selectAllQuery := `
		SELECT id, user_id, title, description, status, version, created_at, updated_at
		FROM tasks
		WHERE user_id = $1
		ORDER BY created_at DESC;`

	executor := database.GetExecutor(ctx, repo.databasePool)
	rows, err := executor.QueryContext(ctx, selectAllQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks for user: %w", err)
	}
	defer rows.Close()

	var taskList []models.Task

	for rows.Next() {
		var task models.Task

		scanErr := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.Version,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan task row: %w", scanErr)
		}

		taskList = append(taskList, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return taskList, nil
}

func (repo *PostgresTaskRepository) Update(ctx context.Context, task *models.Task) error {
	updateQuery := `
		UPDATE tasks
		SET title = $1, description = $2, status = $3, version = version + 1, updated_at = $4
		WHERE id = $5 AND user_id = $6 AND version = $7
		RETURNING version, updated_at;`

	currentTime := time.Now()

	executor := database.GetExecutor(ctx, repo.databasePool)
	err := executor.QueryRowContext(
		ctx,
		updateQuery,
		task.Title,
		task.Description,
		task.Status,
		currentTime,
		task.ID,
		task.UserID,
		task.Version,
	).Scan(&task.Version, &task.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Determine if the task is completely missing (404) or if version changed (409 Conflict)
			var exists bool
			checkQuery := `SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1 AND user_id = $2);`
			_ = executor.QueryRowContext(ctx, checkQuery, task.ID, task.UserID).Scan(&exists)
			if exists {
				return ErrOptimisticLockConflict
			}
			return ErrTaskNotFound
		}
		return fmt.Errorf("failed to execute task update: %w", err)
	}

	return nil
}

func (repo *PostgresTaskRepository) Delete(ctx context.Context, userID, taskID int64) error {
	deleteQuery := `
		DELETE FROM tasks 
		WHERE id = $1 AND user_id = $2;`

	executor := database.GetExecutor(ctx, repo.databasePool)
	result, err := executor.ExecContext(ctx, deleteQuery, taskID, userID)
	if err != nil {
		return fmt.Errorf("failed to execute task delete: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}
