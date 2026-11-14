package database

import (
	"context"
	"database/sql"
	"fmt"
)

// QueryExecutor defines common database operations shared by *sql.DB and *sql.Tx.
type QueryExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Ensure both standard library types satisfy the QueryExecutor interface at compile time.
var (
	_ QueryExecutor = (*sql.DB)(nil)
	_ QueryExecutor = (*sql.Tx)(nil)
)

// transactionContextKey is a private type used to store active transactions inside context.Context.
type transactionContextKey struct{}

// GetExecutor extracts an active *sql.Tx from context if available, otherwise returning the fallback pool.
func GetExecutor(ctx context.Context, databasePool *sql.DB) QueryExecutor {
	if databaseTransaction, ok := ctx.Value(transactionContextKey{}).(*sql.Tx); ok && databaseTransaction != nil {
		return databaseTransaction
	}
	return databasePool
}

// Transactor defines an abstract transaction manager that can execute operations atomically.
type Transactor interface {
	WithinTransaction(ctx context.Context, operation func(transactionContext context.Context) error) error
}

// PostgresTransactor manages PostgreSQL transactions using *sql.DB.
type PostgresTransactor struct {
	databasePool *sql.DB
}

var _ Transactor = (*PostgresTransactor)(nil)

// NewPostgresTransactor creates a new PostgresTransactor instance.
func NewPostgresTransactor(databasePool *sql.DB) *PostgresTransactor {
	return &PostgresTransactor{
		databasePool: databasePool,
	}
}

// WithinTransaction runs the provided operation callback inside a database transaction.
// It rolls back if operation returns an error or panics, and commits if operation succeeds.
func (transactor *PostgresTransactor) WithinTransaction(
	ctx context.Context,
	operation func(transactionContext context.Context) error,
) error {
	databaseTransaction, err := transactor.databasePool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin database transaction: %w", err)
	}

	// Panic safety: rollback transaction before bubbling up the panic
	defer func() {
		if panicValue := recover(); panicValue != nil {
			_ = databaseTransaction.Rollback()
			panic(panicValue)
		}
	}()

	transactionContext := context.WithValue(ctx, transactionContextKey{}, databaseTransaction)

	if err := operation(transactionContext); err != nil {
		_ = databaseTransaction.Rollback()
		return err
	}

	if err := databaseTransaction.Commit(); err != nil {
		return fmt.Errorf("failed to commit database transaction: %w", err)
	}

	return nil
}
