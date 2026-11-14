package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CreateTables executes DDL queries to ensure our application tables exist and are linked.
func CreateTables(databasePool *sql.DB) error {
	schemaQuery := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		status VARCHAR(50) NOT NULL DEFAULT 'pending',
		version INT NOT NULL DEFAULT 1,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS revoked_tokens (
		id BIGSERIAL PRIMARY KEY,
		token_hash VARCHAR(64) NOT NULL UNIQUE,
		token_type VARCHAR(20) NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	DO $$
	BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name='tasks' AND column_name='user_id'
		) THEN
			ALTER TABLE tasks ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
		END IF;

		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name='tasks' AND column_name='version'
		) THEN
			ALTER TABLE tasks ADD COLUMN version INT NOT NULL DEFAULT 1;
		END IF;
	END $$;`

	queryTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	_, err := databasePool.ExecContext(queryTimeout, schemaQuery)
	if err != nil {
		return fmt.Errorf("failed to execute table schema: %w", err)
	}

	fmt.Println("Database tables 'users' & 'tasks' verified/created successfully!")
	return nil
}
