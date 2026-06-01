# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`qc` — a multi-database Go library and CLI (MySQL, PostgreSQL, SQLite) for safe, structured database operations. Designed for AI Agent consumption. All output is JSON.

## Commands

```bash
make build      # → ./qc binary
make test       # go test ./... -count=1
make lint       # go vet ./...
make coverage   # go test -cover ./...
make clean      # rm ./qc
```

## Architecture

### Driver Registry (`pkg/db/db.go`)

Factory pattern with `init()` self-registration. `db.Open(driver, dsn, opts...)` looks up the factory from a registry map. Each database backend lives in its own package and registers itself:

- `pkg/db/mysql/` — `go-sql-driver/mysql`
- `pkg/db/postgres/` — `pgx/v5` via stdlib adapter
- `pkg/db/sqlite/` — `modernc.org/sqlite` (pure Go)

### Session (`pkg/db/session.go`)

Single `Session` struct shared by all backends. Wraps a `*sqlx.DB`. Key methods:

- `Exec(ctx, sql, args...)` — DDL/DML with guard checks
- `Query/QueryWithLimit/QueryWithOffset` — SELECT with LIMIT enforcement
- `QueryStream` — goroutine-based channel streaming
- `Begin` — returns `Tx` with auto-rollback timeout

### Placeholder Rebind

MySQL/SQLite use `?`; PostgreSQL uses `$N`. Before every DB call, `s.Rebind(sql)` converts `?` to the driver's native format. `sqlx` handles this automatically based on driver name.

### Safety Guards (`pkg/guard/`)

- `PolicyPrompt` (default): DROP/TRUNCATE require confirmation
- `PolicyBlock`: absolute rejection
- `PolicyWarn`/`PolicyAllow`: warn or permit
- `RejectNoWhere`: UPDATE/DELETE without WHERE always blocked

### SQL Normalization (`internal/sqlnorm/`)

- `Operation()` — extract SQL keyword
- `HasWHERE/HasLIMIT/HasOFFSET` — clause detection
- `AppendLIMIT/AppendOFFSET` — safe clause appending
- `SplitStatements()` — batch SQL splitter (handles strings, comments, backticks)
- `IsSELECT/RequiresWHERE` — statement classification

### Batch Execution (`cmd/cli/batch.go`)

`batchExec()` executes multiple statements with optional transaction wrapping and continue-on-error support.

### DSN Detection (`pkg/dsn/`)

Auto-detects driver from DSN format: `postgres://` → postgres, `/path/file.db` → sqlite, `user@tcp(...)` → mysql.

## Key Patterns

- **Functional options**: `config.WithDefaultLimit(50)`, `config.WithQueryTimeout(10s)`, etc.
- **Tests use `go-sqlmock`**: no real DB needed. Helper: `newMockSession(t, cfg)` returns `(*Session, sqlmock.Sqlmock)`. External packages use `db.NewTestSession(name, dsn, cfg, sqlxDB)`.
- **`.env` file**: CLI reads `QC_DSN` and `QC_DRIVER` from `.env` (standard KEY=VALUE format). Priority: CLI args > env vars > `.env`.
- **Results**: `result.ExecResult` (LastInsertID, RowsAffected, DurationMs) and `result.QueryResult` (Columns, Rows, RowCount, DurationMs, Warning, HasMore). JSON-tagged for direct serialization.
- **Error sentinels**: `guard.ErrDangerousOp`, `guard.ErrDangerousOpPrompt`, `db.ErrUnconditionalModify`, `db.ErrNonSelectQuery`, `db.ErrTxDone`

## Package Layout

```
cmd/cli/            CLI entry point (main.go, batch.go)
internal/
  sanitize/         SQL param sanitization
  sqlnorm/          SQL parsing, clause detection, batch splitting
pkg/
  config/           Config struct + functional options
  db/               Core: Database interface, Session, Exec, Query, Stream, Tx
    mysql/          MySQL driver (self-registers)
    postgres/       PostgreSQL driver (self-registers)
    sqlite/         SQLite driver (self-registers)
  dsn/              DSN format auto-detection
  guard/            Dangerous operation policies
  registry/         Multi-session registry
  result/           ExecResult, QueryResult, StreamResult
```

## Team Workflow

See `doc/agent-team-init.md`. Multi-agent team: product-manager → developer → test-engineer → code-reviewer → commit.
