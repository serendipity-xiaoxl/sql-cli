# qc Operation Manual

`qc` is a multi-database CLI tool supporting MySQL, PostgreSQL, and SQLite. All output is JSON. Built-in safety guards prevent accidental data loss. Designed for AI Agents and developers.

Version: 0.2.0

---

## Installation

### Requirements

- Go 1.22+

### Build

```bash
make build      # Produces qc binary
make test       # Run tests
make lint       # Code check
```

Place `qc` in your `PATH` after building.

---

## Quick Start

```bash
# MySQL
qc ping "test:test@123@tcp(127.0.0.1:3306)/mydb"
qc exec "test:test@123@tcp(127.0.0.1:3306)/mydb" "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100))"
qc query "test:test@123@tcp(127.0.0.1:3306)/mydb" "SELECT * FROM users"

# PostgreSQL (DSN auto-detected)
qc ping "postgres://user:pass@127.0.0.1:5432/mydb"
qc query "postgres://user:pass@127.0.0.1:5432/mydb" "SELECT * FROM users"

# SQLite
qc query "/data/mydb.sqlite" "SELECT * FROM users"
qc query ":memory:" "SELECT 1"
```

---

## Supported Databases

| Database | DSN Format | Driver |
|----------|------------|--------|
| MySQL | `user:pass@tcp(host:port)/db` | `mysql` |
| PostgreSQL | `postgres://user:pass@host:port/db` | `postgres` / `pgx` |
| SQLite | `/path/to/file.db` or `:memory:` | `sqlite` / `sqlite3` |

DSN is auto-detected. Use `--driver` to override:

```bash
qc --driver postgres "postgres://..." query "SELECT 1"
```

---

## Command Reference

### Conventions

- All output is JSON
- DSN is optional if `QC_DSN` is set
- Use `--driver` to specify database type explicitly

### ping — Health Check

```
qc ping <dsn>
```

Success: `{"status":"ok"}`

### exec — Write Operations

```
qc exec <dsn> <sql>
qc exec <dsn> -f <file.sql>                 # Read SQL from file
qc exec <dsn> -f <file.sql> --transaction   # All in one transaction
qc exec <dsn> -f <file.sql> --continue-on-error  # Continue on error
```

| Arg / Flag | Required | Description |
|------------|----------|-------------|
| dsn | no* | Connection string |
| sql | no** | SQL statement (or use -f) |
| `-f, --file` | no | Read and execute .sql file |
| `--transaction` | no | Wrap all statements in a transaction |
| `--continue-on-error` | no | Continue after individual failures |
| `--force` | no | Skip dangerous op confirmation |

Supported SQL: CREATE TABLE, ALTER TABLE, INSERT, UPDATE, DELETE. DROP/TRUNCATE require confirmation (or `--force`).

Success output:

```json
{"last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
```

### query — Execute Queries

```
qc query <dsn> <sql> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--limit N` | 100 | Max rows to return |
| `--offset N` | 0 | Rows to skip |
| `--timeout D` | 30s | Query timeout |

Auto-appends `LIMIT 100` if missing, capped at 1000. `has_more` is true when result count equals limit.

Output:

```json
{
  "columns": ["id", "name"],
  "rows": [[1, "Alice"], [2, "Bob"]],
  "row_count": 2,
  "duration_ms": 45,
  "warning": "LIMIT 100 applied automatically",
  "has_more": false
}
```

### stream — Streaming Queries

```
qc stream <dsn> <sql> [--limit N] [--timeout D]
```

Outputs one JSON object per line:

```json
{"row": {"id": 1, "name": "Alice"}, "index": 0}
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `QC_DSN` | Default DSN for all commands |

### Global Flags

| Flag | Description |
|------|-------------|
| `--driver <name>` | Specify database driver |
| `--force` | Skip dangerous operation prompt |
| `--version` | Print version |

---

## Safety Features

### LIMIT Enforcement

All queries have automatic row limits (default 100, max 1000). `has_more` indicates truncation.

### Dangerous Operation Confirmation

DROP/TRUNCATE prompt for confirmation. UPDATE/DELETE without WHERE are rejected outright. Use `--force` to skip prompts.

### Command Isolation

query and stream accept only SELECT — write operations are rejected.

### Query Timeout

Default 30 seconds. Timed-out queries are cancelled automatically.

---

## Go Library Usage

```go
import (
    _ "github.com/xiaoxl/sql-cli/pkg/db/mysql"
    _ "github.com/xiaoxl/sql-cli/pkg/db/postgres"
    _ "github.com/xiaoxl/sql-cli/pkg/db/sqlite"

    "github.com/xiaoxl/sql-cli/pkg/db"
    "github.com/xiaoxl/sql-cli/pkg/config"
    "github.com/xiaoxl/sql-cli/pkg/registry"
)

// MySQL
sess, _ := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/db",
    config.WithDefaultLimit(50),
    config.WithQueryTimeout(10 * time.Second),
)

// PostgreSQL
sess, _ := db.Open("postgres", "postgres://user:pass@127.0.0.1:5432/db")

// SQLite
sess, _ := db.Open("sqlite", "/data/mydb.sqlite")

defer sess.Close()
ctx := context.Background()

// Write
res, _ := sess.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")

// Query
q, _ := sess.Query(ctx, "SELECT * FROM users")
// Paginate
q, _ = sess.QueryWithOffset(ctx, "SELECT * FROM users", 10, 20)
// Stream
sr, _ := sess.QueryStream(ctx, "SELECT * FROM large_table")
for sr.Next() { row := sr.Scan() }
// Transaction
tx, _ := sess.Begin(ctx)
tx.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Bob")
tx.Commit(ctx)

// Multi-session
reg := registry.NewRegistry()
reg.Open("prod", "mysql", "user:pass@tcp(prod:3306)/db")
reg.Open("dev", "mysql", "user:pass@tcp(dev:3306)/db")
prod, _ := reg.Get("prod")
reg.CloseAll()
```

### Config Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithDefaultLimit(n)` | 100 | Auto LIMIT |
| `WithMaxLimit(n)` | 1000 | LIMIT hard cap |
| `WithQueryTimeout(d)` | 30s | Query timeout |
| `WithMaxOpenConns(n)` | 25 | Max connections |
| `WithMaxIdleConns(n)` | 5 | Max idle connections |
| `WithDangerousOpPolicy(p)` | PolicyPrompt | Block/Warn/Allow/Prompt |
| `WithRejectNoWhere(b)` | true | Reject UPDATE/DELETE without WHERE |

---

## Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `dangerous operation requires confirmation` | DROP/TRUNCATE blocked | Type `yes` or use `--force` |
| `UPDATE/DELETE without WHERE clause` | Missing WHERE | Add WHERE clause |
| `only SELECT queries are allowed` | Wrong command | Use exec instead |
| `LIMIT capped to N` | Limit exceeds max | Reduce or paginate |
| `transaction already committed or rolled back` | Double commit/rollback | Check tx logic |
| `unknown database driver` | Driver not registered | Check import |
| `unrecognized DSN format` | DSN not parseable | Use `--driver` explicitly |
