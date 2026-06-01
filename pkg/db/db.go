// Package db defines the Database and Tx interfaces for database operations.
// Implementations (e.g., MySQL) live in internal/ and pkg/db/mysql/.
package db

import (
	"context"

	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/result"
)

// Database is the common interface for database operations on a single session.
// Every method accepts context.Context for timeout and cancellation support.
type Database interface {
	// Ping verifies a connection to the database is still alive.
	Ping(ctx context.Context) error

	// Exec executes a DDL or DML statement and returns the result.
	Exec(ctx context.Context, sql string, args ...interface{}) (*result.ExecResult, error)

	// Query executes a SELECT with forced LIMIT enforcement.
	Query(ctx context.Context, sql string, args ...interface{}) (*result.QueryResult, error)

	// QueryWithLimit executes a SELECT with a caller-specified page size.
	// The limit is capped by the session's configured MaxLimit.
	QueryWithLimit(ctx context.Context, sql string, limit int, args ...interface{}) (*result.QueryResult, error)

	// QueryStream returns a StreamResult iterator for row-by-row or batch processing.
	QueryStream(ctx context.Context, sql string, args ...interface{}) (*result.StreamResult, error)

	// Begin starts a new transaction.
	Begin(ctx context.Context) (Tx, error)

	// Close closes the session's database connection.
	Close() error
}

// Tx is the interface for transaction-scoped operations.
type Tx interface {
	// Commit commits the transaction.
	Commit(ctx context.Context) error

	// Rollback aborts the transaction.
	Rollback(ctx context.Context) error

	// Exec executes a statement within the transaction.
	Exec(ctx context.Context, sql string, args ...interface{}) (*result.ExecResult, error)

	// Query executes a SELECT within the transaction.
	Query(ctx context.Context, sql string, args ...interface{}) (*result.QueryResult, error)
}

// Open creates a new Session with the given driver, DSN, and options.
func Open(driver, dsn string, options ...config.Option) (*Session, error) {
	cfg := config.DefaultConfig()
	for _, opt := range options {
		opt(cfg)
	}
	return newSession(driver, dsn, cfg)
}
