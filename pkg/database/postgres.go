package database

import (
	"context"      //a built in go package used to control timeouts and cancellations.
	"database/sql" //a universal database manager used for handling conn pools and the db stuffs.
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" //the postgres driver from github
)

func ConnectPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(25)                 // max number of open tcp connections
	db.SetMaxIdleConns(25)                 // max number of idle tcp connections in the pool
	db.SetConnMaxLifetime(5 * time.Minute) // max amount of time a tcp connection may be reused

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	return db, nil
}
