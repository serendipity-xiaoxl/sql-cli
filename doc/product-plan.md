# SQL-CLI Product Plan

## 1. Project Goals and Scope

### Goal
Build a modular Go library (and CLI tool) that provides safe, structured, Agent-oriented database operations for MySQL. The library is designed primarily to be called by AI Agents (LLM-based coding assistants) to interact with databases programmatically.

### Scope (v1)
- MySQL-only support with a generic DB interface to enable future backends (PostgreSQL, SQLite).
- Core operations: connection management, DDL/DML execution, manual transactions, safe queries with mandatory LIMIT, streaming queries, structured JSON-compatible results.
- Configuration-driven safety constraints: max row limits, timeout, dangerous operation blocking, concurrent query limits.
- Complete unit test coverage.

### Out of Scope (v1)
- Visual or interactive CLI TUI.
- ORM or query builder.
- Database migration tooling.
- Replication, sharding, or cluster management.
- Built-in SQL editor or REPL.

---

## 2. Feature List with Priorities

### P0 — Core / Must-Have

| ID | Feature | Description |
|----|---------|-------------|
| F-01 | Connection Management | Open/Close/Ping with configurable pool (max conns, idle conns, max lifetime). Multi-session support by connection name. |
| F-02 | DDL/DML Execution | Exec() for CREATE, ALTER, INSERT, UPDATE, DELETE. Returns structured result (LastInsertId, RowsAffected, duration). |
| F-03 | Manual Transactions | Begin/Commit/Rollback with timeout-based auto-rollback. All query/exec operations scoped to the transaction handle. |
| F-04 | Safe Queries (Mandatory LIMIT) | SELECT must always have LIMIT — auto-applied if missing, max limit capped by config, warning returned. Parameterized queries to prevent SQL injection. |
| F-05 | Structured Results | Unified JSON-compatible result format: columns, rows, row_count, duration_ms, warning, has_more. |
| F-06 | Streaming Queries | Row-by-row or batch iteration via iterator pattern (Next/Scan/Err/Close). Mandatory LIMIT enforced. Context-based cancellation. |

### P1 — Important

| ID | Feature | Description |
|----|---------|-------------|
| F-07 | Dangerous Operation Guard | Block or warn on DROP, TRUNCATE, DELETE/UPDATE without WHERE (configurable). |
| F-08 | Timeout Control | All methods accept context.Context for query/exec timeout. |
| F-09 | QueryWithLimit | Separate method allowing Agent to specify per-page size, capped by global max. |
| F-10 | Logging | Structured logging via `log/slog` with optional SQL parameter sanitization. |
| F-11 | Configuration System | Centralized config for: max rows, timeout, stream buffer size, connection pool, danger ops policy, LIMIT default value. |
| F-12 | Multi-Session Registry | Named connection registry — Agent can open, list, get, and close sessions by name. |

### P2 — Nice-to-Have

| ID | Feature | Description |
|----|---------|-------------|
| F-13 | Cursor-based Pagination | Support `LIMIT ? OFFSET ?` based continuation for streaming. |
| F-14 | Concurrent Query Limits | Per-session cap on concurrent queries (configurable). |
| F-15 | Sensitive Data Protection | SQL parameter sanitization in logs (mask passwords, secrets). |
| F-16 | CLI Wrapper | Command-line entry point wrapping the library for direct human use. |

---

## 3. Acceptance Criteria per Feature

### F-01 Connection Management
- [ ] `Open(driver, dsn string, options ...Option) (*Session, error)` works with MySQL driver.
- [ ] Connection pool parameters (MaxOpenConns, MaxIdleConns, ConnMaxLifetime) are configurable via Option pattern.
- [ ] `Close()` gracefully drains pool and returns error on failure.
- [ ] `Ping()` verifies connectivity and returns error if unreachable.
- [ ] Multiple sessions can coexist with distinct names.

### F-02 DDL/DML Execution
- [ ] `Exec(ctx, sql, args...)` returns `Result` with `LastInsertId`, `RowsAffected`, `Duration`.
- [ ] Supports: CREATE TABLE, ALTER TABLE, INSERT, UPDATE, DELETE.
- [ ] DELETE/UPDATE without WHERE is blocked by default (configurable).
- [ ] Execution time is measured and included in the result.
- [ ] Error contains the SQL statement context.

### F-03 Manual Transactions
- [ ] `Begin(ctx) (*Tx, error)` creates a new transaction.
- [ ] `Tx.Commit()` and `Tx.Rollback()` work correctly.
- [ ] `Tx.Exec()` and `Tx.Query()` use the same transaction handle.
- [ ] Transaction auto-rollback after configurable timeout.
- [ ] Double-commit or double-rollback returns an error.
- [ ] Operations after commit/rollback return an error.

### F-04 Safe Queries
- [ ] SELECT without LIMIT auto-appends `LIMIT <default>` (default: 100).
- [ ] Auto-appended LIMIT returns a warning in the result.
- [ ] `QueryWithLimit(ctx, sql, limit, args...)` allows explicit LIMIT.
- [ ] Max LIMIT value is bounded by global config (default: 1000).
- [ ] Parameterized queries prevent SQL injection.
- [ ] Non-SELECT statements are rejected by Query/QueryWithLimit.

### F-05 Structured Results
- [ ] `QueryResult` JSON serialization matches the specified schema.
- [ ] `columns` is `[]string`, `rows` is `[][]interface{}`.
- [ ] `row_count`, `duration_ms` are populated.
- [ ] `warning` field is populated on auto-applied LIMIT.
- [ ] `has_more` flag indicates incomplete results.

### F-06 Streaming Queries
- [ ] `StreamResult` supports `Next() bool`, `Scan(dest...)`, `Err() error`, `Close()`.
- [ ] Rows are fetched lazily, not buffered entirely in memory.
- [ ] Context cancellation terminates the stream and frees resources.
- [ ] `Close()` is idempotent and safe to call multiple times.
- [ ] Mandatory LIMIT is enforced identically to regular queries.
- [ ] All rows consumed or stream closed releases the DB connection.

### F-07 Dangerous Operation Guard
- [ ] Config has `DangerousOpPolicy` with modes: `Block`, `Warn`, `Allow`.
- [ ] Default mode blocks DROP TABLE, TRUNCATE, DELETE/UPDATE without WHERE.
- [ ] Blocked operations return an explicit `ErrDangerousOp` error type.

### F-08 Timeout Control
- [ ] All Exec/Query/QueryStream methods accept `context.Context`.
- [ ] Context deadline is respected and results in context cancellation error.

### F-09 QueryWithLimit
- [ ] Accepts explicit limit parameter.
- [ ] Limit is clamped to the configured maximum.
- [ ] If SQL already contains LIMIT, the explicit parameter is ignored (or overridden per config).

### F-10 Logging
- [ ] All operations log at appropriate levels (INFO for success, WARN for auto-LIMIT, ERROR for failures).
- [ ] SQL parameters can be sanitized via configuration to avoid logging sensitive data.

### F-11 Configuration System
- [ ] Centralized `Config` struct with documented defaults.
- [ ] Options applied via functional options pattern.
- [ ] Config includes: MaxRows, DefaultLimit, QueryTimeout, StreamBatchSize, MaxOpenConns, MaxIdleConns, ConnMaxLifetime, DangerousOpPolicy, LogSanitizeParams.

### F-12 Multi-Session Registry
- [ ] `NewRegistry()` creates a session manager.
- [ ] `registry.Open(name, driver, dsn, opts...)` creates and stores a session.
- [ ] `registry.Get(name)` retrieves a session by name.
- [ ] `registry.List()` returns all session names and status.
- [ ] `registry.Close(name)` closes a specific session.
- [ ] `registry.CloseAll()` gracefully closes all sessions.

---

## 4. Non-Goals (explicitly out of scope for v1)

- PostgreSQL, SQLite, or any non-MySQL backend support (interface defined but no implementation).
- Visual / interactive user interface (TUI, web UI).
- ORM features: model mapping, relationship loading, auto-migration.
- Query builder or SQL generation.
- Database migration framework (versioned migrations, up/down scripts).
- Connection proxy, read/write splitting, or load balancing.
- Data export/import utilities.
- SQL syntax validation or linting.
- Replication monitoring or cluster management.
- Connection string generation or secret management.

---

## 5. Architecture Decisions Summary

### 5.1 Package Layout

```
sql-cli/
├── cmd/cli/             # CLI entry point (P2)
├── pkg/
│   ├── db/              # DB interface + MySQL implementation
│   │   ├── mysql/       # MySQL driver implementation
│   ├── config/          # Configuration system
│   ├── result/          # Structured result types
│   ├── stream/          # Streaming query types
│   ├── registry/        # Multi-session registry
│   └── guard/           # Dangerous operation guard
├── internal/
│   └── sanitize/        # SQL parameter sanitization (P2)
├── doc/                 # Documentation
└── go.mod
```

### 5.2 Core Interface

```go
type Database interface {
    Open(driver, dsn string, opts ...Option) (*Session, error)
    Close() error
    Ping(ctx context.Context) error
    Exec(ctx context.Context, sql string, args ...interface{}) (*Result, error)
    Query(ctx context.Context, sql string, args ...interface{}) (*QueryResult, error)
    QueryWithLimit(ctx context.Context, sql string, limit int, args ...interface{}) (*QueryResult, error)
    QueryStream(ctx context.Context, sql string, args ...interface{}) (*StreamResult, error)
    Begin(ctx context.Context) (*Tx, error)
}
```

### 5.3 Design Rationale

1. **Interface-based abstraction** — `Database` interface allows future backend implementations (PostgreSQL, SQLite) without changing callers.

2. **Functional options pattern** — `Option` functions provide extensible, backward-compatible configuration.

3. **Iterator-based streaming** — `StreamResult` wraps `sql.Rows` with `Next()/Scan()/Err()/Close()`. This is idiomatic Go and avoids channel complexities (goroutine lifecycle, ordering guarantees).

4. **Mandatory LIMIT enforcement** — Implemented via SQL parsing (simple regex for `SELECT` without `LIMIT`). This is a safety-critical feature; false positives are acceptable (query rejected) over false negatives.

5. **Transaction timeout** — Achieved via `context.WithTimeout` passed from `Begin`. The context is stored on the `Tx` and used for all subsequent operations; auto-rollback runs via a `defer`-based cleanup goroutine.

6. **Unified result types** — `Result` (for DML) and `QueryResult` (for SELECT) are plain structs with JSON tags, designed for direct serialization by the calling Agent.

7. **Separation of concerns** — `pkg/db` handles core operations, `pkg/config` isolates configuration, `pkg/registry` manages sessions, `pkg/guard` implements safety policies. Each is independently testable.

### 5.4 Key Dependencies

| Dependency | Version | Purpose |
|-----------|---------|---------|
| `github.com/go-sql-driver/mysql` | latest | MySQL driver |
| `github.com/jmoiron/sqlx` | latest | Convenience wrappers (NamedExec, StructScan) |
| `github.com/pkg/errors` | latest | Error wrapping with stack traces |

### 5.5 Security & Safety Model

1. **Query safety**: LIMIT enforcement at the library level, not trust-based.
2. **Operation guard**: Configurable policy for dangerous statements (DROP, TRUNCATE, unconditional DELETE/UPDATE).
3. **SQL injection prevention**: Parameterized queries are mandatory; string interpolation is never used for values.
4. **Resource protection**: Connection pooling, context-based timeouts, stream cancellation.
5. **Log safety**: Optional parameter sanitization to avoid logging credentials.

### 5.6 Testing Strategy

- **Unit tests** with `go-sql-driver/mysql` test harness or `sqlmock` for isolation.
- **Integration tests** against a real MySQL instance (Docker-based via `testcontainers-go` or GitHub Actions service containers).
- **Coverage targets**: >80% for `pkg/db`, `pkg/config`, `pkg/guard`, `pkg/registry`.
- **Edge cases**: Empty results, network errors, context cancellation mid-stream, concurrent session access.
