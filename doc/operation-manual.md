# qc Operation Manual

`qc` is a MySQL CLI tool for safe, structured database operations. All output is JSON. Built-in safety guards prevent accidental data loss. Designed for AI Agents and developers.

Version: 0.1.0

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

Place the `qc` binary in your `PATH` after building.

---

## Quick Start

```bash
# Test connection
qc ping "test:test@123@tcp(115.29.209.119:3306)/test"

# Create table
qc exec "test:test@123@tcp(115.29.209.119:3306)/test" \
  "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100), age INT)"

# Insert data
qc exec "test:test@123@tcp(115.29.209.119:3306)/test" \
  "INSERT INTO users (name, age) VALUES ('Alice', 25), ('Bob', 30)"

# Query with auto LIMIT
qc query "test:test@123@tcp(115.29.209.119:3306)/test" "SELECT * FROM users"

# Paginated query
qc query "test:test@123@tcp(115.29.209.119:3306)/test" \
  --limit 10 --offset 20 "SELECT * FROM users"

# Stream large datasets
qc stream "test:test@123@tcp(115.29.209.119:3306)/test" "SELECT * FROM large_table"
```

---

## Command Reference

### Conventions

- DSN format: `user:pass@tcp(host:port)/dbname`
- All output is JSON
- Set `QC_DSN` environment variable to omit the DSN argument

### ping — Health Check

```
qc ping <dsn>
```

| Argument | Required | Description |
|----------|----------|-------------|
| dsn | no* | Connection string (optional if `QC_DSN` is set) |

Success output:

```json
{"status":"ok"}
```

### exec — Execute Write Operations

```
qc exec <dsn> <sql>
```

| Argument | Required | Description |
|----------|----------|-------------|
| dsn | no* | Connection string |
| sql | yes | SQL statement to execute |

Supported SQL:

- `CREATE TABLE` / `ALTER TABLE`
- `INSERT`
- `UPDATE` (WHERE clause required)
- `DELETE` (WHERE clause required)

**Note**: `DROP TABLE` and `TRUNCATE TABLE` are blocked by default. `UPDATE`/`DELETE` without WHERE are also rejected.

Success output:

```json
{"last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
```

### query — Execute Queries

```
qc query <dsn> <sql> [flags]
```

| Arg / Flag | Required | Default | Description |
|------------|----------|---------|-------------|
| dsn | no* | — | Connection string |
| sql | yes | — | SELECT statement |
| `--limit N` | no | 100 | Max rows to return |
| `--offset N` | no | 0 | Rows to skip |
| `--timeout D` | no | 30s | Query timeout (e.g., `10s`, `1m`) |

**Key behaviors**:

- If SQL has no `LIMIT`, `LIMIT 100` is auto-appended
- `--limit` is capped at the global max (1000)
- When returned rows equal the limit, `has_more` is `true`
- Non-SELECT statements are rejected

Success output:

```json
{
  "columns": ["id", "name", "age"],
  "rows": [[1, "Alice", 25], [2, "Bob", 30]],
  "row_count": 2,
  "duration_ms": 45,
  "warning": "LIMIT 100 applied automatically",
  "has_more": false
}
```

### stream — Streaming Queries

```
qc stream <dsn> <sql> [flags]
```

| Arg / Flag | Required | Default | Description |
|------------|----------|---------|-------------|
| dsn | no* | — | Connection string |
| sql | yes | — | SELECT statement |
| `--limit N` | no | 100 | Max rows to return |
| `--timeout D` | no | 30s | Query timeout |

Unlike `query`, `stream` outputs one JSON object per line, ideal for large datasets.

Output (one line per row):

```json
{"row": {"id": 1, "name": "Alice"}, "index": 0}
{"row": {"id": 2, "name": "Bob"}, "index": 1}
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--version` | Print version |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `QC_DSN` | Default DSN, makes the dsn argument optional |

---

## Safety Features

`qc` has built-in safety guards to prevent accidental data loss.

### LIMIT Enforcement

All queries have automatic row limits to prevent full table scans.

| Setting | Default | Description |
|---------|---------|-------------|
| Default LIMIT | 100 | Auto-appended when SQL has no LIMIT |
| Max LIMIT | 1000 | Hard cap for any query |

The `warning` and `has_more` fields in output indicate data truncation.

### Dangerous Operation Guard

The following operations are blocked by default:

| Operation | Default |
|-----------|---------|
| `DROP TABLE` | Blocked |
| `TRUNCATE TABLE` | Blocked |
| `UPDATE` without `WHERE` | Blocked |
| `DELETE` without `WHERE` | Blocked |

### Non-Query Isolation

The `query` and `stream` commands only accept SELECT. Any write operation is rejected, preventing accidental use of the wrong command.

### Query Timeout

Every query has a timeout (default 30 seconds). Timed-out queries are cancelled automatically.

---

## Output Format

### Write Result (exec)

```json
{
  "last_insert_id": 42,
  "rows_affected": 1,
  "duration_ms": 15
}
```

| Field | Type | Description |
|-------|------|-------------|
| `last_insert_id` | int | Auto-increment ID (omitted when 0) |
| `rows_affected` | int | Number of affected rows |
| `duration_ms` | int | Execution time in milliseconds |
| `error` | string | Present only on error |

### Query Result (query)

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
| `columns` | []string | Column names in query order |
| `rows` | [][]any | Data rows, each row follows columns order |
| `row_count` | int | Number of rows returned |
| `duration_ms` | int | Query execution time in ms |
| `warning` | string | Present when LIMIT is auto-applied or capped |
| `has_more` | bool | `true` if data may be truncated |
| `error` | string | Present only on error |

### Stream Row (stream)

```json
{"row": {"id": 1, "name": "Alice"}, "index": 0}
```

| Field | Type | Description |
|-------|------|-------------|
| `row` | object | Column-to-value mapping |
| `index` | int | Row index (0-based) |

---

## Go Library Usage

Import `qc` as a Go library for programmatic access.

### Import

```bash
go get github.com/xiaoxl/sql-cli
```

### Basic Usage

```go
sess, _ := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/mydb",
    config.WithDefaultLimit(50),
    config.WithQueryTimeout(10 * time.Second),
)
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
```

### Config Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithDefaultLimit` | int | 100 | Auto LIMIT value |
| `WithMaxLimit` | int | 1000 | LIMIT hard cap |
| `WithMaxRows` | int | 1000 | Max returned rows |
| `WithQueryTimeout` | duration | 30s | Query timeout |
| `WithMaxOpenConns` | int | 25 | Connection pool size |
| `WithMaxIdleConns` | int | 5 | Max idle connections |
| `WithConnMaxLifetime` | duration | 5m | Connection reuse time |
| `WithStreamBatchSize` | int | 50 | Stream buffer size |
| `WithDangerousOpPolicy` | Policy | Block | Block/Warn/Allow |
| `WithRejectNoWhere` | bool | true | Reject UPDATE/DELETE without WHERE |
| `WithLogSanitizeParams` | bool | false | Sanitize logged params |
| `WithMaxConcurrentQueries` | int | 0 | Concurrency cap (0=unlimited) |

### Multi-Session

```go
reg := registry.NewRegistry()
reg.Open("prod", "mysql", "user:pass@tcp(prod-db:3306)/db")
reg.Open("dev", "mysql", "user:pass@tcp(dev-db:3306)/db")
prod, _ := reg.Get("prod")
reg.CloseAll()
```

---

## Testing

```bash
make test            # Unit tests
make coverage        # Coverage report
go test -race ./...  # Race detection
```

Integration testing with Docker MySQL:

```bash
docker run -d --name qc-mysql \
  -e MYSQL_ROOT_PASSWORD=testpass \
  -e MYSQL_DATABASE=testdb \
  -p 3307:3306 mysql:8

QC_DSN="root:testpass@tcp(127.0.0.1:3307)/testdb" go test ./...
docker stop qc-mysql && docker rm qc-mysql
```

---

## Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `dangerous operation blocked` | Tried DROP/TRUNCATE | Adjust policy if intentional |
| `UPDATE/DELETE without WHERE clause` | Missing WHERE | Add WHERE clause |
| `only SELECT queries are allowed` | Used query/stream for write | Use exec instead |
| `LIMIT capped to N` | Limit exceeds max | Reduce limit or use pagination |
| `transaction is already committed or rolled back` | Double commit/rollback | Check transaction logic |
