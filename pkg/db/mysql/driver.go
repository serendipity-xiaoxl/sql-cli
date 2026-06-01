// Package mysql provides a MySQL driver factory for the db package registry.
// Import this package to register the "mysql" driver:
//
//	import _ "github.com/xiaoxl/sql-cli/pkg/db/mysql"
package mysql

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
)

func init() {
	db.RegisterDriver("mysql", factory)
}

// factory creates a *db.Session backed by a MySQL connection pool.
// The connection is not opened until the first query, so DSN validation
// errors are surfaced lazily.
func factory(dsn string, cfg *config.Config) (*db.Session, error) {
	sqlxDB, err := sqlx.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlxDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlxDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlxDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db.NewSessionFromDB(dsn, cfg, sqlxDB), nil
}
