// Package dsn provides DSN (Data Source Name) parsing and driver detection
// for various database types. It auto-detects the database driver from the
// DSN format without requiring an explicit --driver flag.
package dsn

import (
	"fmt"
	"strings"
)

// Known driver identifiers returned by Detect.
const (
	DriverMySQL    = "mysql"
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite3"
	DriverOracle   = "oracle"
)

// Detect identifies the database driver from a DSN string by pattern matching.
// Returns the driver name and nil on success, or an error if the DSN format is
// not recognized.
//
// Supported DSN patterns:
//
//	MySQL:     user:password@tcp(host:port)/dbname, user@tcp(host:port)/dbname
//	           user@unix(/path/to/socket)/dbname
//	PostgreSQL: postgres://user:password@host:port/dbname
//	           postgresql://user:password@host:port/dbname
//	SQLite:    /path/to/file.db, file:path/to/db.sqlite, :memory:
//	Oracle:    user/password@host:port/service, user/password@//host:port/service
func Detect(dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "", fmt.Errorf("empty DSN")
	}

	switch {
	case hasPrefixFold(dsn, "postgres://") || hasPrefixFold(dsn, "postgresql://"):
		return DriverPostgres, nil

	case hasPrefixFold(dsn, "mysql://"):
		return DriverMySQL, nil

	case strings.Contains(dsn, "@tcp(") ||
		strings.Contains(dsn, "@unix(") ||
		strings.Contains(dsn, "@tcp4(") ||
		strings.Contains(dsn, "@tcp6("):
		return DriverMySQL, nil

	case dsn == ":memory:":
		return DriverSQLite, nil

	case hasPrefixFold(dsn, "file:"):
		return DriverSQLite, nil

	case looksLikeSQLitePath(dsn):
		return DriverSQLite, nil

	case looksLikeOracle(dsn):
		return DriverOracle, nil

	case strings.Contains(dsn, "@") && strings.Contains(dsn, "/"):
		// Fallback: user@host/db pattern (MySQL-style without explicit tcp)
		return DriverMySQL, nil

	default:
		return "", fmt.Errorf("unrecognized DSN format: %q", dsn)
	}
}

// hasPrefixFold is a case-insensitive prefix check.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// looksLikeSQLitePath checks if the DSN is a file path that looks like a SQLite
// database file.
func looksLikeSQLitePath(dsn string) bool {
	lower := strings.ToLower(dsn)
	return strings.HasSuffix(lower, ".db") ||
		strings.HasSuffix(lower, ".sqlite") ||
		strings.HasSuffix(lower, ".sqlite3") ||
		strings.HasSuffix(lower, ".s3db") ||
		strings.HasSuffix(lower, ".db3")
}

// looksLikeOracle checks for Oracle-style DSN patterns: user/pass@host or
// user/pass@//host.
func looksLikeOracle(dsn string) bool {
	// Oracle patterns: user/password@host or user/password@//host
	if !strings.Contains(dsn, "@") {
		return false
	}
	// Check if there's a `/` before `@` (user/pass@...)
	atIdx := strings.Index(dsn, "@")
	beforeAt := dsn[:atIdx]
	if strings.Contains(beforeAt, "/") {
		// Check that it's not a MySQL-like tcp() pattern (those always have
		// "@tcp(" with no `/` between user and @)
		beforeAt = strings.TrimSpace(beforeAt)
		slashIdx := strings.LastIndex(beforeAt, "/")
		if slashIdx > 0 {
			// Has both user and password parts separated by /
			after := dsn[atIdx+1:]
			after = strings.TrimSpace(after)
			// Oracle: user/pass@host or user/pass@//host:port/service
			// Not Oracle if it contains tcp( which is MySQL
			if strings.Contains(after, "tcp(") {
				return false
			}
			// Check if it looks like a host:port/service format
			if strings.Contains(after, ":") || strings.HasPrefix(after, "//") {
				return true
			}
			// Simple hostname after @ with / before @ is likely Oracle
			// but could also be other formats, so be conservative
			if len(after) > 0 && !strings.ContainsAny(after, " \t=()") {
				return true
			}
		}
	}
	return false
}
