package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/xiaoxl/sql-cli/pkg/config"

	_ "github.com/go-sql-driver/mysql"
)

func init() {
	RegisterDriver("mysql", defaultMySQLFactory)
}

func defaultMySQLFactory(dsn string, cfg *config.Config) (*Session, error) {
	return newSession("mysql", dsn, cfg)
}

// Session represents a single database session with a connection pool.
// It implements the Database interface for MySQL.
type Session struct {
	name   string
	dsn    string
	cfg    *config.Config
	db     *sqlx.DB
	logger *slog.Logger
	mu     sync.Mutex
	active int // current concurrent queries
}

// newSessionWithDB creates a Session with an existing sqlx.DB (used by tests with sqlmock).
func newSessionWithDB(name, dsn string, cfg *config.Config, db *sqlx.DB) *Session {
	s := &Session{
		name:   name,
		dsn:    dsn,
		cfg:    cfg,
		db:     db,
		logger: slog.Default().With("component", "session"),
	}
	if cfg.Name != "" {
		s.name = cfg.Name
	}
	return s
}

// NewTestSession creates a Session with an existing sqlx.DB (for external tests).
func NewTestSession(name, dsn string, cfg *config.Config, db *sqlx.DB) *Session {
	return newSessionWithDB(name, dsn, cfg, db)
}

// NewSessionFromDB creates a Session from an existing sqlx.DB connection.
// Used by driver factories (e.g., pkg/db/mysql) to construct a Session with
// full pool configuration.
func NewSessionFromDB(dsn string, cfg *config.Config, sqlxDB *sqlx.DB) *Session {
	s := &Session{
		name:   dsn,
		dsn:    dsn,
		cfg:    cfg,
		db:     sqlxDB,
		logger: slog.Default().With("component", "session"),
	}
	if cfg.Name != "" {
		s.name = cfg.Name
	}
	return s
}

// newSession creates a new Session with the given configuration.
func newSession(driver, dsn string, cfg *config.Config) (*Session, error) {
	db, err := sqlx.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN for %s: %w", driver, err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	s := &Session{
		name:   dsn,
		dsn:    dsn,
		cfg:    cfg,
		db:     db,
		logger: slog.Default().With("component", "session"),
	}

	if cfg.Name != "" {
		s.name = cfg.Name
	}

	return s, nil
}

// Ping verifies the database connection is alive.
func (s *Session) Ping(ctx context.Context) error {
	start := time.Now()
	err := s.db.PingContext(ctx)
	s.logger.Debug("ping",
		"duration_ns", time.Since(start).Nanoseconds(),
		"error", err,
	)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

// Close closes the database connection pool.
func (s *Session) Close() error {
	s.logger.Info("closing session")
	return s.db.Close()
}

// acquireConcurrencySlot blocks if MaxConcurrentQueries is set and we've reached the limit.
func (s *Session) acquireConcurrencySlot(ctx context.Context) error {
	if s.cfg.MaxConcurrentQueries <= 0 {
		return nil
	}
	s.mu.Lock()
	if s.active >= s.cfg.MaxConcurrentQueries {
		s.mu.Unlock()
		return fmt.Errorf("max concurrent queries reached (%d)", s.cfg.MaxConcurrentQueries)
	}
	s.active++
	s.mu.Unlock()
	return nil
}

func (s *Session) releaseConcurrencySlot() {
	if s.cfg.MaxConcurrentQueries <= 0 {
		return
	}
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
}

// Name returns the session's identifier.
func (s *Session) Name() string {
	return s.name
}

// DSN returns the session's data source name.
func (s *Session) DSN() string {
	return s.dsn
}

// Config returns the session's configuration.
func (s *Session) Config() *config.Config {
	return s.cfg
}

// PoolStats holds connection pool statistics.
type PoolStats struct {
	OpenConnections int `json:"open_connections"`
	InUse           int `json:"in_use"`
	Idle            int `json:"idle"`
	MaxOpenConns    int `json:"max_open_conns"`
}

// Begin starts a new transaction with an auto-rollback timeout.
// The returned Tx supports Commit, Rollback, Exec, and Query (with full safety guards).
// If the session's QueryTimeout is set, the transaction is automatically rolled
// back when the timeout expires unless Commit or Rollback is called first.
func (s *Session) Begin(ctx context.Context) (Tx, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	// Create a timeout context for auto-rollback
	txCtx := ctx
	var cancel context.CancelFunc
	if s.cfg.QueryTimeout > 0 {
		txCtx, cancel = context.WithTimeout(ctx, s.cfg.QueryTimeout)
	} else {
		txCtx, cancel = context.WithCancel(ctx)
	}

	s.logger.Info("transaction started")
	return newTransaction(tx, s.cfg, s.logger.With("component", "transaction"), txCtx, cancel), nil
}

// Stats returns current connection pool statistics.
func (s *Session) Stats() PoolStats {
	st := s.db.Stats()
	return PoolStats{
		OpenConnections: st.OpenConnections,
		InUse:           st.InUse,
		Idle:            st.Idle,
		MaxOpenConns:    st.MaxOpenConnections,
	}
}
