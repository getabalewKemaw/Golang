package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task-manager/internal/models"
)

var ErrTaskNotFound = errors.New("task not found")

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	GetByID(ctx context.Context, taskID int64) (*models.Task, error)
	GetAll(ctx context.Context) ([]models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	Delete(ctx context.Context, taskID int64) error
}

type PostgresTaskRepository struct {
	databasePool *sql.DB
}

func NewPostgresTaskRepository(databasePool *sql.DB) *PostgresTaskRepository {
	return &PostgresTaskRepository{
		databasePool: databasePool,
	}
}

func (repo *PostgresTaskRepository) Create(ctx context.Context, task *models.Task) error {
	insertQuery := `
		INSERT INTO tasks (title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id;`
	currentTime := time.Now()
	task.CreatedAt = currentTime
	task.UpdatedAt = currentTime

	err := repo.databasePool.QueryRowContext(
		ctx,
		insertQuery,
		task.Title,
		task.Description,
		task.Status,
		task.CreatedAt,
		task.UpdatedAt,
	).Scan(&task.ID)

	if err != nil {
		return fmt.Errorf("failed to insert task: %w", err)
	}

	return nil
}

func (repo *PostgresTaskRepository) GetByID(ctx context.Context, taskID int64) (*models.Task, error) {
	selectQuery := `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks
		WHERE id = $1;`

	var task models.Task

	err := repo.databasePool.QueryRowContext(ctx, selectQuery, taskID).Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Status,
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

func (repo *PostgresTaskRepository) GetAll(ctx context.Context) ([]models.Task, error) {
	selectAllQuery := `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks
		ORDER BY created_at DESC;`

	rows, err := repo.databasePool.QueryContext(ctx, selectAllQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query all tasks: %w", err)
	}
	defer rows.Close()

	var taskList []models.Task

	for rows.Next() {
		var task models.Task
		scanErr := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&task.Status,
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
		SET title = $1, description = $2, status = $3, updated_at = $4
		WHERE id = $5;`

	task.UpdatedAt = time.Now()

	result, err := repo.databasePool.ExecContext(
		ctx,
		updateQuery,
		task.Title,
		task.Description,
		task.Status,
		task.UpdatedAt,
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute task update: %w", err)
	}

	// Verify that at least one row was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

func (repo *PostgresTaskRepository) Delete(ctx context.Context, taskID int64) error {
	deleteQuery := `DELETE FROM tasks WHERE id = $1;`

	result, err := repo.databasePool.ExecContext(ctx, deleteQuery, taskID)
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
