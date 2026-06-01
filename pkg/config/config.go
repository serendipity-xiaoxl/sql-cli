// Package config provides configuration types and functional options
// for configuring database sessions.
package config

import (
	"time"

	"github.com/xiaoxl/sql-cli/pkg/guard"
)

// Default configuration values.
const (
	DefaultMaxRows        = 1000
	DefaultDefaultLimit   = 100
	DefaultMaxLimit       = 1000
	DefaultQueryTimeout   = 30 * time.Second
	DefaultStreamBatchSize = 50
	DefaultMaxOpenConns   = 25
	DefaultMaxIdleConns   = 5
	DefaultConnMaxLifetime = 5 * time.Minute
)

// Config holds all configuration for a database session.
type Config struct {
	// Name is an optional identifier for the session (used in multi-session registry).
	Name string
	// RejectNoWhere rejects UPDATE/DELETE statements without a WHERE clause.
	RejectNoWhere bool
	// MaxOpenConns is the maximum number of open connections to the database.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections in the pool.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	ConnMaxLifetime time.Duration
	// MaxRows is the maximum number of rows a query may return.
	MaxRows int
	// DefaultLimit is the default LIMIT applied when a SELECT has no LIMIT clause.
	DefaultLimit int
	// MaxLimit is the maximum allowed value for LIMIT clauses.
	MaxLimit int
	// QueryTimeout is the default timeout for query operations.
	QueryTimeout time.Duration
	// StreamBatchSize is the number of rows to buffer in streaming queries.
	StreamBatchSize int
	// DangerousOpPolicy controls how dangerous operations are handled.
	DangerousOpPolicy guard.Policy
	// LogSanitizeParams enables SQL parameter sanitization in logs.
	LogSanitizeParams bool
	// MaxConcurrentQueries limits concurrent queries per session. 0 means unlimited.
	MaxConcurrentQueries int
}

// Option defines a functional option for configuring a Session.
type Option func(*Config)

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Name:                  "",
		RejectNoWhere:         true,
		MaxOpenConns:          DefaultMaxOpenConns,
		MaxIdleConns:          DefaultMaxIdleConns,
		ConnMaxLifetime:       DefaultConnMaxLifetime,
		MaxRows:               DefaultMaxRows,
		DefaultLimit:          DefaultDefaultLimit,
		MaxLimit:              DefaultMaxLimit,
		QueryTimeout:          DefaultQueryTimeout,
		StreamBatchSize:       DefaultStreamBatchSize,
		DangerousOpPolicy:     guard.PolicyBlock,
		LogSanitizeParams:     false,
		MaxConcurrentQueries:  0,
	}
}

// WithName sets an optional identifier for the session.
func WithName(name string) Option {
	return func(c *Config) { c.Name = name }
}

// WithRejectNoWhere sets whether UPDATE/DELETE without WHERE is rejected.
func WithRejectNoWhere(reject bool) Option {
	return func(c *Config) { c.RejectNoWhere = reject }
}

// WithMaxOpenConns sets the maximum number of open connections.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) { c.MaxOpenConns = n }
}

// WithMaxIdleConns sets the maximum number of idle connections.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) { c.MaxIdleConns = n }
}

// WithConnMaxLifetime sets the maximum lifetime of a connection.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) { c.ConnMaxLifetime = d }
}

// WithMaxRows sets the maximum number of rows a query may return.
func WithMaxRows(n int) Option {
	return func(c *Config) { c.MaxRows = n }
}

// WithDefaultLimit sets the default LIMIT for queries without one.
func WithDefaultLimit(n int) Option {
	return func(c *Config) { c.DefaultLimit = n }
}

// WithMaxLimit sets the maximum allowed LIMIT value.
func WithMaxLimit(n int) Option {
	return func(c *Config) { c.MaxLimit = n }
}

// WithQueryTimeout sets the default timeout for queries.
func WithQueryTimeout(d time.Duration) Option {
	return func(c *Config) { c.QueryTimeout = d }
}

// WithStreamBatchSize sets the number of rows per streaming batch.
func WithStreamBatchSize(n int) Option {
	return func(c *Config) { c.StreamBatchSize = n }
}

// WithDangerousOpPolicy sets the policy for dangerous operations.
func WithDangerousOpPolicy(p guard.Policy) Option {
	return func(c *Config) { c.DangerousOpPolicy = p }
}

// WithLogSanitizeParams enables SQL parameter sanitization in logs.
func WithLogSanitizeParams(enabled bool) Option {
	return func(c *Config) { c.LogSanitizeParams = enabled }
}

// WithMaxConcurrentQueries sets the maximum concurrent queries per session.
func WithMaxConcurrentQueries(n int) Option {
	return func(c *Config) { c.MaxConcurrentQueries = n }
}
