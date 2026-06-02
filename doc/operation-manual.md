# qc Operation Manual

`qc` is a multi-database CLI for MySQL, PostgreSQL, and SQLite. All output is JSON. Built-in safety guards. Designed for AI Agents and developers.

Version: 0.3.0

---

## Install

```bash
make build      # produces qc binary
make test       # run tests
make lint       # code check
```

---

## Quick Start

Create a `.env` file in your project directory (compatible with docker-compose and other tools):

```bash
echo 'QC_DSN=test:test@123@tcp(127.0.0.1:3306)/mydb' > .env
echo 'QC_DRIVER=mysql' >> .env
```

Then no need to type DSN every time:

```bash
qc ping                        # test connection
qc exec "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100))"
qc exec "INSERT INTO users (name) VALUES ('Alice')"
qc query "SELECT * FROM users"
qc query --limit 10 --offset 0 "SELECT * FROM users"
qc stream "SELECT * FROM large_table"
```

Or pass DSN explicitly (overrides .env):

```bash
# MySQL
qc ping "user:pass@tcp(127.0.0.1:3306)/db"

# PostgreSQL
qc ping "postgres://user:pass@127.0.0.1:5432/db"

# SQLite
qc query "/data/mydb.sqlite" "SELECT * FROM users"
```

---

## Commands

DSN priority: CLI argument > `QC_DSN` env var > `.env` file. Global flags (`--driver`, `--force`, `--limit`, `--timeout`) must appear before the command name.

### ping

```
qc ping [dsn]
```

Success: `{"status":"ok"}`

### exec

```
qc exec [dsn] <sql>
qc exec [dsn] -f <file.sql>              # batch from file
qc exec [dsn] --force <sql>              # skip confirmation
```

| Flag | Description |
|------|-------------|
| `-f, --file <path>` | Execute SQL from file |
| `--transaction` | Wrap all in one transaction |
| `--continue-on-error` | Continue after failures |

DROP/TRUNCATE require confirmation (type `yes`) or use `--force`. UPDATE/DELETE without WHERE are blocked.

Output:

```json
{"last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
```

Batch output (JSON array):

```json
[
  {"statement": "CREATE TABLE...", "rows_affected": 0, "duration_ms": 120},
  {"statement": "INSERT INTO...", "last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
]
```

### query

```
qc query [dsn] <sql> [--limit N] [--offset N] [--timeout D]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--limit N` | 100 | Max rows (capped at 1000) |
| `--offset N` | 0 | Rows to skip |
| `--timeout D` | 30s | Query timeout |

Auto-appends `LIMIT 100` if missing. `has_more` is true when results may be truncated.

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

### stream

```
qc stream [dsn] <sql> [--limit N] [--timeout D]
```

Outputs one JSON object per line:

```json
{"id": 1, "name": "Alice"}
{"id": 2, "name": "Bob"}
```

### shell

```
qc shell [dsn] [--limit N] [--timeout D] [--force]
```

Interactive read-eval-print loop with a persistent connection. Each SQL statement returns one JSON line. Statements separated by `;` execute sequentially on the same connection.

```bash
# Interactive mode
qc shell <dsn>

# Pipeline mode (AI Agent)
echo "SELECT * FROM users; INSERT INTO logs VALUES(1);" | qc shell <dsn>

# Multi-statement transaction
echo "BEGIN; INSERT INTO t VALUES(1); COMMIT;" | qc shell <dsn>
```

| Command | Description |
|---------|-------------|
| `exit` / `quit` | Exit the shell |
| Ctrl+D | Exit the shell |
| `--` prefix | Line comment (skipped) |

Output — one JSON line per statement:

```json
{"statement":"SELECT * FROM users","type":"query","result":{"columns":["id"],"rows":[[1]],"row_count":1,"duration_ms":0}}
{"statement":"INSERT INTO logs VALUES(1)","type":"exec","result":{"last_insert_id":1,"rows_affected":1,"duration_ms":2}}
{"statement":"BAD SQL","type":"exec","error":"syntax error..."}
```

In interactive mode, a `qc> ` prompt appears on stderr. In pipeline mode, stdout contains only JSON. All log output is suppressed (WARN+ only).

### Global Flags

| Flag | Description |
|------|-------------|
| `--driver <name>` | mysql / postgres / sqlite |
| `--force` | Skip confirmation |
| `--limit N` | Query row limit (shell/query) |
| `--offset N` | Query row offset (query) |
| `--timeout D` | Query timeout |
| `--version` | Print version |

---

## Supported Databases

| Database | DSN Format |
|----------|------------|
| MySQL | `user:pass@tcp(host:port)/db` |
| PostgreSQL | `postgres://user:pass@host:port/db` |
| SQLite | `/path/to/file.db` or `:memory:` |

Driver auto-detected. Override with `--driver`:

```bash
qc --driver postgres ping "postgres://..."
```

---

## Connection Lifecycle

Each CLI invocation opens a single connection (MaxOpenConns=1), executes its work, and closes it on exit. The Go library default (MaxOpenConns=25) still applies when embedding `qc` as a library in long-running applications.

### When to use shell vs one-shot commands

| Scenario | Use |
|----------|-----|
| Single SELECT/INSERT/UPDATE | One-shot `query` or `exec` |
| Multi-statement transaction | `shell` or `exec --file --transaction` |
| Stateful session (SET, TEMP TABLE) | `shell` |
| AI Agent with multiple SQL calls | `shell` (pipeline mode) |
| Batch from file | `exec --file` |

---

## Safety

- **LIMIT enforcement**: auto 100, max 1000
- **Confirmation**: DROP/TRUNCATE require typing `yes`; `--force` skips
- **WHERE guard**: UPDATE/DELETE without WHERE are blocked
- **Command isolation**: query/stream only accept SELECT
- **Timeout**: default 30s, auto-cancel
- **Shell**: auto-allows dangerous operations (interactive context)

---

## .env Config

Standard `.env` format (KEY=VALUE, `#` comments):

```bash
QC_DSN=user:pass@tcp(127.0.0.1:3306)/mydb
QC_DRIVER=mysql
```

Priority: CLI args > env vars > `.env` file

---

## Go Library

```go
import (
    _ "github.com/xiaoxl/sql-cli/pkg/db/mysql"
    _ "github.com/xiaoxl/sql-cli/pkg/db/postgres"
    _ "github.com/xiaoxl/sql-cli/pkg/db/sqlite"

    "github.com/xiaoxl/sql-cli/pkg/db"
    "github.com/xiaoxl/sql-cli/pkg/config"
)

sess, _ := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/db",
    config.WithDefaultLimit(50),
    config.WithQueryTimeout(10*time.Second),
)
defer sess.Close()

ctx := context.Background()
sess.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")
q, _ := sess.Query(ctx, "SELECT * FROM users")
q, _ = sess.QueryWithOffset(ctx, "SELECT * FROM users", 10, 20)
sr, _ := sess.QueryStream(ctx, "SELECT * FROM t")
for sr.Next() { row := sr.Scan() }
tx, _ := sess.Begin(ctx)
tx.Commit(ctx)
```

---

## Common Errors

| Error | Solution |
|-------|----------|
| `DSN is required` | Set `.env`, `QC_DSN` env var, or pass DSN |
| `dangerous operation requires confirmation` | Type `yes` or use `--force` |
| `UPDATE/DELETE without WHERE clause` | Add WHERE clause |
| `only SELECT queries are allowed` | Use exec instead |
| `LIMIT capped to N` | Reduce limit or paginate |
| `unknown database driver` | Check `--driver` or DSN format |
