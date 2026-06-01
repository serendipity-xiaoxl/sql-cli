// Package postgres provides a PostgreSQL driver factory for the db package
// registry. Import this package to register the "postgres" driver:
//
//	import _ "github.com/xiaoxl/sql-cli/pkg/db/postgres"
//
// The factory uses github.com/jackc/pgx/v5 via the stdlib adapter, which
// provides a database/sql compatible interface. sqlx automatically detects
// the "pgx" driver and uses $N placeholder bind mode.
//
// DSN format: postgres://user:password@host:port/dbname?sslmode=disable
package postgres

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
)

func init() {
	db.RegisterDriver("postgres", factory)
	db.RegisterDriver("pgx", factory)
}

// factory creates a *db.Session backed by a PostgreSQL connection pool.
// The underlying driver is pgx v5 via its stdlib adapter. The connection is
// not opened until the first query; DSN validation errors surface lazily.
func factory(dsn string, cfg *config.Config) (*db.Session, error) {
	sqlxDB, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlxDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlxDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlxDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db.NewSessionFromDB(dsn, cfg, sqlxDB), nil
}
