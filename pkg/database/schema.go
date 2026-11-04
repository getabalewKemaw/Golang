package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func CreateTables(databasePool *sql.DB) error {
	schemaQuery := `
	CREATE TABLE IF NOT EXISTS tasks (
		id BIGSERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		status VARCHAR(50) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`

	queryTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	_, err := databasePool.ExecContext(queryTimeout, schemaQuery)
	if err != nil {
		return fmt.Errorf("failed to execute table schema: %w", err)
	}

	fmt.Println("Database table 'tasks' verified/created successfully!")
	return nil
}
