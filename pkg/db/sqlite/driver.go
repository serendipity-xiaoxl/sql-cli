// Package sqlite provides a SQLite driver factory for the db package registry.
// Uses modernc.org/sqlite (pure Go, no CGO).
//
// Import this package to register the "sqlite3" and "sqlite" drivers:
//
//	import _ "github.com/xiaoxl/sql-cli/pkg/db/sqlite"
package sqlite

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
	_ "modernc.org/sqlite"
)

func init() {
	db.RegisterDriver("sqlite3", factory)
	db.RegisterDriver("sqlite", factory)
}

// factory creates a *db.Session backed by a SQLite database.
// Supports file paths, :memory:, and file: URI DSNs.
func factory(dsn string, cfg *config.Config) (*db.Session, error) {
	sqlxDB, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlxDB.SetMaxOpenConns(1)
	sqlxDB.SetMaxIdleConns(1)
	sqlxDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db.NewSessionFromDB(dsn, cfg, sqlxDB), nil
}
