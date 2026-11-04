package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Registers pgx driver for database/sql
)

func ConnectPostgres(connectionString string) (*sql.DB, error) {
	databasePool, err := sql.Open("pgx", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database pool: %w", err)
	}

	databasePool.SetMaxOpenConns(25)                 // max number of active TCP connections
	databasePool.SetMaxIdleConns(25)                 // max number of warm standby connections in pool
	databasePool.SetConnMaxLifetime(5 * time.Minute) // max connection reuse lifespan

	connectionTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	if err := databasePool.PingContext(connectionTimeout); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	fmt.Println("Connected to PostgreSQL successfully!")

	if err := CreateTables(databasePool); err != nil {
		return nil, fmt.Errorf("database table setup failed: %w", err)
	}

	return databasePool, nil
}
