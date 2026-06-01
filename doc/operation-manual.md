# sql-cli Operation Manual

## Overview

sql-cli is a Go library and CLI tool for safe, structured MySQL database operations. It is designed for both direct human use and AI Agent consumption, with JSON-friendly result types, automatic safety guards, and streaming support.

### Repository

```
github.com/xiaoxl/sql-cli
```

---

## Build & Install

### Prerequisites

- Go 1.22 or later
- MySQL 8.0+ (or a compatible MySQL-compatible database)

### Build the CLI

```bash
# Using make (recommended)
make build
./qc --help

# Or manually
go build -o qc ./cmd/cli/
./qc --help
```

### Build the Library

```bash
go build ./...
```

### Run Tests

```bash
# All tests
make test

# With race detector
go test ./... -count=1 -race

# Code coverage
make coverage

# Lint
make lint

# Clean build artifacts
make clean
```

---

## CLI Commands

The CLI (`qc`) wraps the library with JSON output. All commands accept a MySQL DSN as a positional argument or via the `SQL_CLI_DSN` environment variable.

### ping -- Health Check

```
sql-cli ping <dsn>
```

Checks database connectivity. Returns `{"status":"ok"}` on success.

```bash
sql-cli ping "user:pass@tcp(127.0.0.1:3306)/mydb"
```

### exec -- Execute DDL/DML

```
sql-cli exec <dsn> <sql>
```

Executes CREATE, ALTER, INSERT, UPDATE, DELETE statements. Returns JSON with `last_insert_id`, `rows_affected`, and `duration_ms`.

```bash
sql-cli exec "root:pass@tcp(127.0.0.1:3306)/test" "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name TEXT)"
sql-cli exec "root:pass@tcp(127.0.0.1:3306)/test" "INSERT INTO users (name) VALUES ('Alice')"
```

### query -- Execute SELECT

```
sql-cli query <dsn> <sql> [flags]
```

Executes a SELECT query with automatic LIMIT enforcement. Returns JSON with `columns`, `rows`, `row_count`, `duration_ms`, `warning`, and `has_more`.

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--limit N` | 100 | Maximum rows to return |
| `--offset N` | 0 | Number of rows to skip (pagination) |
| `--timeout D` | (session default) | Query timeout, e.g. `30s` |

```bash
# Basic query (auto-applies LIMIT 100)
sql-cli query "user:pass@tcp(127.0.0.1:3306)/test" "SELECT * FROM users"

# Paginated query
sql-cli query "user:pass@tcp(127.0.0.1:3306)/test" --limit 10 --offset 20 "SELECT * FROM users"
```

### stream -- Stream Results

```
sql-cli stream <dsn> <sql> [flags]
```

Executes a SELECT query and streams each row as a JSON line. Suitable for large result sets.

```bash
sql-cli stream "user:pass@tcp(127.0.0.1:3306)/test" "SELECT * FROM large_table"
```

Output format (one JSON object per line):

```json
{"row": {"id": 1, "name": "Alice"}, "index": 0}
{"row": {"id": 2, "name": "Bob"}, "index": 1}
```

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--version` | false | Print version and exit |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `QC_DSN` | Default DSN for CLI commands |

---

## Library API

### Opening a Session

```go
import (
    "github.com/xiaoxl/sql-cli/pkg/db"
    "github.com/xiaoxl/sql-cli/pkg/config"
)

// Basic
sess, err := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/mydb")

// With options
sess, err := db.Open("mysql", dsn,
    config.WithDefaultLimit(50),
    config.WithMaxLimit(500),
    config.WithQueryTimeout(10*time.Second),
    config.WithDangerousOpPolicy(guard.PolicyBlock),
)
defer sess.Close()
```

### Configuration Options

All configuration options in `pkg/config`:

| Option | Default | Description |
|--------|---------|-------------|
| `WithName(n)` | DSN | Session identifier |
| `WithRejectNoWhere(b)` | true | Reject UPDATE/DELETE without WHERE |
| `WithDefaultLimit(n)` | 100 | Default LIMIT for queries without one |
| `WithMaxLimit(n)` | 1000 | Maximum allowed LIMIT value |
| `WithMaxRows(n)` | 1000 | Maximum rows per query |
| `WithQueryTimeout(d)` | 30s | Default query timeout |
| `WithMaxOpenConns(n)` | 25 | Max open connections in pool |
| `WithMaxIdleConns(n)` | 5 | Max idle connections |
| `WithConnMaxLifetime(d)` | 5m | Max connection reuse duration |
| `WithStreamBatchSize(n)` | 50 | Stream channel buffer size |
| `WithDangerousOpPolicy(p)` | PolicyBlock | How to handle DROP/TRUNCATE |
| `WithLogSanitizeParams(b)` | false | Sanitize SQL params in logs |
| `WithMaxConcurrentQueries(n)` | 0 | Max concurrent queries (0 = unlimited) |

### Executing Statements

```go
ctx := context.Background()

// INSERT
res, err := sess.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")
fmt.Printf("LastInsertID: %d\n", res.LastInsertID)

// UPDATE with WHERE (reject without WHERE by default)
res, err := sess.Exec(ctx, "UPDATE users SET name = ? WHERE id = ?", "Bob", 1)
fmt.Printf("RowsAffected: %d\n", res.RowsAffected)

// CREATE TABLE
_, err := sess.Exec(ctx, "CREATE TABLE IF NOT EXISTS items (id INT AUTO_INCREMENT PRIMARY KEY, name TEXT)")

// DROP TABLE (blocked by default)
_, err := sess.Exec(ctx, "DROP TABLE items")
// returns guard.ErrDangerousOp

// DROP TABLE with PolicyWarn
cfg.DangerousOpPolicy = guard.PolicyWarn
_, err := sess.Exec(ctx, "DROP TABLE items")
// allowed with warning logged
```

### Querying Data

```go
// Basic query (auto-appends LIMIT 100)
res, err := sess.Query(ctx, "SELECT * FROM users")
fmt.Printf("Columns: %v\n", res.Columns)
fmt.Printf("Rows: %v\n", res.Rows)
fmt.Printf("Warning: %s\n", res.Warning) // "LIMIT 100 applied automatically"
fmt.Printf("HasMore: %v\n", res.HasMore) // true if row count reached limit

// With explicit limit
res, err := sess.QueryWithLimit(ctx, "SELECT * FROM users", 50)

// With pagination (LIMIT 10 OFFSET 20)
res, err := sess.QueryWithOffset(ctx, "SELECT * FROM users", 10, 20)

// With QueryOptions
res, err := sess.QueryWithOptions(ctx, "SELECT * FROM users", db.QueryOptions{
    Limit:  10,
    Offset: 20,
})

// Parameterized query
res, err := sess.Query(ctx, "SELECT * FROM users WHERE id = ?", 42)

// Non-SELECT is rejected
_, err := sess.Query(ctx, "DELETE FROM users")
// returns ErrNonSelectQuery
```

### Streaming Queries

```go
sr, err := sess.QueryStream(ctx, "SELECT * FROM large_table")
if err != nil {
    log.Fatal(err)
}
defer sr.Close() // close early to stop streaming

for sr.Next() {
    row := sr.Scan()
    fmt.Println(row)
    // row is map[string]interface{} — column names as keys
}

if err := sr.Err(); err != nil {
    log.Fatal(err)
}
```

### Transactions

```go
tx, err := sess.Begin(ctx)
if err != nil {
    log.Fatal(err)
}
defer tx.Rollback(ctx) // safe — no-op if committed

tx.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Charlie")
tx.Exec(ctx, "UPDATE accounts SET balance = balance - 100 WHERE id = ?", 1)

if err := tx.Commit(ctx); err != nil {
    log.Fatal(err)
}
```

Transaction features:
- **Auto-rollback**: If the transaction is not committed within `QueryTimeout`, it is automatically rolled back.
- **Safety guards**: Same Exec/Query guards apply inside transactions (DROP blocked, UPDATE without WHERE rejected, LIMIT enforcement).
- **Double-commit/rollback**: Returns `ErrTxDone`.

### Multi-Session Registry

```go
reg := registry.NewRegistry()

// Open named sessions
reg.Open("prod", "mysql", dsn1)
reg.Open("staging", "mysql", dsn2)

// Get by name
sess, err := reg.Get("prod")

// List all sessions
names := reg.List() // ["prod", "staging"]

// Close individual
reg.Close("staging")

// Close all
reg.CloseAll()
```

---

## Result Format

All results are designed for direct JSON serialization and Agent consumption.

### ExecResult (DDL/DML)

```json
{
  "last_insert_id": 42,
  "rows_affected": 1,
  "duration_ms": 15
}
```

| Field | Type | Description |
|-------|------|-------------|
| `last_insert_id` | int64 | Last inserted auto-increment ID; omitted when 0 |
| `rows_affected` | int64 | Number of rows affected by the statement |
| `duration_ms` | int64 | Execution time in milliseconds |
| `error` | string | Error message; omitted when empty |

### QueryResult (SELECT)

```json
{
  "columns": ["id", "name", "email"],
  "rows": [[1, "Alice", "alice@example.com"], [2, "Bob", "bob@example.com"]],
  "row_count": 2,
  "duration_ms": 45,
  "warning": "LIMIT 100 applied automatically",
  "has_more": false
}
```

| Field | Type | Description |
|-------|------|-------------|
| `columns` | []string | Column names in the same order as the query |
| `rows` | [][]interface{} | Row data — each row is an ordered slice of values |
| `row_count` | int | Number of rows returned |
| `duration_ms` | int64 | Query execution time in milliseconds |
| `warning` | string | Warning (e.g., auto-applied LIMIT); omitted when empty |
| `has_more` | bool | True if the row count reached the limit (more rows may exist) |
| `error` | string | Error message; omitted when empty |

### StreamRow

```json
{"row": {"id": 1, "name": "Alice", "email": "alice@example.com"}, "index": 0}
{"row": {"id": 2, "name": "Bob", "email": "bob@example.com"}, "index": 1}
```

Each line is a JSON object with:

| Field | Type | Description |
|-------|------|-------------|
| `row` | map[string]interface{} | Column-name-to-value mapping for one row |
| `index` | int64 | Sequential row index (0-based) |
| `error` | string | Row-level error; omitted when empty |

---

## Safety Features

### 1. Automatic LIMIT Enforcement

All SELECT queries without an explicit LIMIT clause get one appended automatically using `DefaultLimit` (100). The limit is capped at `MaxLimit` (1000). The `has_more` field indicates whether the result set might continue beyond the returned rows.

```go
// Auto-appends: SELECT * FROM users LIMIT 100
res, _ := sess.Query(ctx, "SELECT * FROM users")
// Warning: "LIMIT 100 applied automatically"
// HasMore: true if returned exactly 100 rows
```

### 2. Dangerous Operation Guard

`DROP TABLE` and `TRUNCATE TABLE` are blocked by default (`PolicyBlock`). Can be configured to warn (`PolicyWarn`) or allow silently (`PolicyAllow`).

```go
// Blocked
_, err := sess.Exec(ctx, "DROP TABLE users")
// returns guard.ErrDangerousOp

// Warn (allowed with logged warning)
cfg.DangerousOpPolicy = guard.PolicyWarn
sess.Exec(ctx, "DROP TABLE users") // allowed
```

### 3. Unconditional Modify Protection

`UPDATE` and `DELETE` statements without a `WHERE` clause are rejected by default. This is controlled by `RejectNoWhere` (default: `true`).

```go
// Blocked
_, err := sess.Exec(ctx, "DELETE FROM users")
// returns ErrUnconditionalModify

// Allowed
_, err := sess.Exec(ctx, "DELETE FROM users WHERE id = 1")

// Disable protection
cfg.RejectNoWhere = false
```

### 4. Non-SELECT Query Rejection

`Query()` and `QueryStream()` reject any SQL that is not a SELECT or WITH statement.

### 5. Query Timeout

All queries have a configurable timeout. The session's `QueryTimeout` (default: 30s) is applied unless the context already has a deadline.

### 6. Concurrency Limits

`MaxConcurrentQueries` (default: 0 = unlimited) limits concurrent queries per session.

---

## Testing Guide

### Running Tests

```bash
# All tests
go test ./... -count=1

# With race detector
go test ./... -count=1 -race

# Coverage
go test ./... -count=1 -coverprofile=coverage.out
go tool cover -func=coverage.out
```

### Package Test Structure

All tests use `github.com/DATA-DOG/go-sqlmock` to mock MySQL, so no real database is needed.

```go
func newMockSession(t *testing.T, cfg *config.Config) (*Session, sqlmock.Sqlmock) {
    db, mock, _ := sqlmock.New(sqlmock.MonitorPingsOption(true))
    sdb := sqlx.NewDb(db, "sqlmock")
    s := NewTestSession("test", "mock://", cfg, sdb)
    return s, mock
}
```

### Test Pattern

```go
func TestQuerySelectWithoutLimit(t *testing.T) {
    s, mock := newMockSession(t, config.DefaultConfig())

    mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
        WillReturnRows(sqlmock.NewRows([]string{"name"}))

    res, err := s.Query(context.Background(), "SELECT * FROM users")
    assert.NoError(t, err)
    assert.Equal(t, "LIMIT 100 applied automatically", res.Warning)
    assert.False(t, res.HasMore)

    mock.ExpectClose()
    s.Close()
    mock.ExpectationsWereMet()
}
```

### Integration Tests with Docker MySQL

For tests that need a real MySQL instance:

```bash
# Start MySQL 8.0 container
docker run -d \
  --name sql-cli-mysql \
  -e MYSQL_ROOT_PASSWORD=testpass \
  -e MYSQL_DATABASE=testdb \
  -p 3307:3306 \
  mysql:8

# Wait for MySQL to be ready
sleep 10

# Run integration tests
QC_TEST_DSN="root:testpass@tcp(127.0.0.1:3307)/testdb" go test ./... -count=1

# Stop and remove when done
docker stop sql-cli-mysql && docker rm sql-cli-mysql
```

### Current Coverage

| Package | Coverage |
|---------|----------|
| `internal/sanitize` | 100.0% |
| `internal/sqlnorm` | 98.7% |
| `pkg/config` | 92.6% |
| `pkg/db` | 90.4% |
| `pkg/guard` | 100.0% |
| `pkg/registry` | 100.0% |
| `pkg/result` | 100.0% |

---

## Package Layout

```
sql-cli/
  cmd/cli/              CLI entry point
  internal/
    sanitize/           SQL parameter sanitization
    sqlnorm/            SQL normalization (operation, WHERE, LIMIT, OFFSET detection)
      pagination.go     Cursor-based pagination (ApplyCursor)
  pkg/
    config/             Configuration system (functional options)
    db/                 Core library (Session, Exec, Query, Stream, Transaction)
      db.go             Database/Tx interfaces
      session.go        Session (Open, Close, Ping, Begin, concurrency)
      executor.go       Exec implementation
      query.go          Query, QueryWithLimit, QueryWithOffset, QueryWithOptions
      stream.go         QueryStream (channel-based streaming)
      transaction.go    Transaction wrapper (auto-rollback, safety guards)
    guard/              Dangerous operation policy enforcement
    registry/           Multi-session registry
    result/             Structured result types (ExecResult, QueryResult, StreamResult)
  doc/
    product-plan.md     Feature specifications and priorities
    operation-manual.md This document
    test-report.md      Test coverage report
```
