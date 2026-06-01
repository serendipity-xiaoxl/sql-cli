# Multi-Database Support Roadmap

## 1. Current State

The project currently supports only MySQL via the `Database` interface in `pkg/db/db.go`:

```go
type Database interface {
    Ping(ctx context.Context) error
    Exec(ctx context.Context, sql string, args ...interface{}) (*result.ExecResult, error)
    Query(ctx context.Context, sql string, args ...interface{}) (*result.QueryResult, error)
    QueryWithLimit(ctx context.Context, sql string, limit int, args ...interface{}) (*result.QueryResult, error)
    QueryWithOffset(ctx context.Context, sql string, limit, offset int, args ...interface{}) (*result.QueryResult, error)
    QueryStream(ctx context.Context, sql string, args ...interface{}) (*result.StreamResult, error)
    Begin(ctx context.Context) (Tx, error)
    Close() error
}
```

The `Session` struct implements this interface directly for MySQL. There is no driver abstraction or dialect layer.

---

## 2. Architecture for Multi-DB Support

### 2.1 Driver Factory Pattern

Introduce a driver registry that maps driver names to factory functions:

```
pkg/db/
  db.go              # Database interface (unchanged)
  session.go         # MySQL Session (refactored to backend-agnostic)
  driver.go          # Driver registry: Register(name, factory), Open(name, dsn, cfg)
  dialect.go         # Dialect interface for SQL differences
  mysql/
    driver.go        # MySQL driver implementation
  postgres/
    driver.go        # PostgreSQL driver implementation
  sqlite/
    driver.go        # SQLite driver implementation
```

### 2.2 Driver Registration

```go
package driver

type Factory func(dsn string, cfg *config.Config) (Database, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) {
    registry[name] = f
}

func Open(driver, dsn string, cfg *config.Config) (Database, error) {
    f, ok := registry[driver]
    if !ok {
        return nil, fmt.Errorf("unknown driver: %s", driver)
    }
    return f(dsn, cfg)
}
```

Drivers self-register in `init()`:
```go
package mysql

func init() {
    driver.Register("mysql", newMySQLSession)
}
```

---

## 3. DSN Pattern Recognition

### 3.1 Automatic Driver Detection

When the CLI user provides a DSN without specifying a driver, auto-detect from the DSN format:

| DSN Pattern | Detected Driver | Example |
|-------------|----------------|---------|
| `user:pass@tcp(host:port)/db` | mysql | `root:pass@tcp(127.0.0.1:3306)/test` |
| `user:pass@unix(/path)/db` | mysql | `root:pass@unix(/var/run/mysqld/mysqld.sock)/test` |
| `postgres://user:pass@host:port/db` | postgres | `postgres://user:pass@localhost:5432/test` |
| `host=... port=... user=... dbname=...` | postgres | `host=localhost port=5432 user=postgres dbname=test` |
| `/path/to/file.db` | sqlite | `/data/mydb.sqlite3` |
| `file:/path/to/file.db` | sqlite | `file:/data/mydb.sqlite3` |
| `:memory:` | sqlite | `:memory:` |
| `user/pass@host:port/db` | oracle | `system/pass@localhost:1521/XEPDB1` |

**Detection order**: Check DSN prefixes in priority order (postgres://, file:, etc.), then fall through to pattern matching. MySQL DSN (user@tcp(...)) checked before Oracle since Oracle DSNs use `/` separators that overlap with MySQL's format.

### 3.2 CLI Usage

```bash
# Explicit driver
qc --driver postgres "postgres://user:pass@localhost:5432/db" query "SELECT * FROM users"

# Auto-detected from DSN
qc "postgres://user:pass@localhost:5432/db" query "SELECT * FROM users"

# SQLite (local file, auto-detect from .db/.sqlite extension or /path pattern)
qc "/data/mydb.sqlite3" query "SELECT * FROM users"
```

### 3.3 Registry API

```go
// --driver flag maps to driver name
// If --driver is empty, DSN is analyzed for auto-detection

// Library API: explicit driver name
sess, err := db.Open("mysql", dsn)
sess, err := db.Open("postgres", "postgres://user:pass@localhost:5432/db")
```

---

## 4. Dialect Abstraction

Different databases have different SQL syntax. The dialect layer abstracts these differences so that safety features (LIMIT enforcement, WHERE detection) work across backends.

### 4.1 Dialect Interface

```go
type Dialect interface {
    // Name returns the dialect identifier.
    Name() string

    // Placeholder returns the parameter placeholder style.
    //   mysql:  "?"
    //   pg:     "$1", "$2", ...
    //   sqlite: "?"
    //   oracle: ":1", ":2", ... or ":name"
    Placeholder(pos int) string

    // QuoteIdentifier quotes a column/table name.
    //   mysql:  `name`
    //   pg:     "name"
    //   sqlite: "name" or `name`
    //   oracle: "name"
    QuoteIdentifier(name string) string

    // SupportsReturning returns true if the dialect supports RETURNING.
    //   pg/sqlite: true
    //   mysql/oracle: false (MySQL 8.0 has limited support via OUT parameters)
    SupportsReturning() bool

    // LimitSyntax returns the LIMIT clause string.
    //   mysql/pg/sqlite: "LIMIT n"
    //   oracle (old):    "FETCH FIRST n ROWS ONLY"
    LimitSyntax(limit int) string

    // OffsetSyntax returns the OFFSET clause string.
    //   mysql/pg/sqlite: "OFFSET n"
    //   oracle (old):    "OFFSET n ROWS"
    OffsetSyntax(offset int) string

    // MatchLimit returns true if the SQL contains a dialect-specific LIMIT pattern.
    //   mysql/pg/sqlite: match "LIMIT"
    //   oracle/sqlserver: match "FETCH FIRST" / "OFFSET ... FETCH"
    MatchLimit(sql string) bool

    // MatchOffset returns true if the SQL contains a dialect-specific OFFSET pattern.
    MatchOffset(sql string) bool

    // SanitizeSQL normalizes any dialect-specific syntax before safety analysis.
    // For example, convert Oracle's "FETCH FIRST n ROWS ONLY" to "LIMIT n".
    SanitizeSQL(sql string) string
}
```

### 4.2 SQL Dialect Variations

| Feature | MySQL | PostgreSQL | SQLite | Oracle |
|---------|-------|------------|--------|--------|
| Param placeholder | `?` | `$N` | `?` or `$NNN` | `:N` or `:name` |
| Identifier quote | `` ` `` | `"` | `"` or `` ` `` | `"` |
| Auto-increment | `AUTO_INCREMENT` | `SERIAL` / `IDENTITY` | `AUTOINCREMENT` | `IDENTITY` / `SEQUENCE` |
| LIMIT syntax | `LIMIT n` | `LIMIT n` | `LIMIT n` | `FETCH FIRST n ROWS ONLY` |
| OFFSET syntax | `OFFSET n` | `OFFSET n` | `OFFSET n` | `OFFSET n ROWS` |
| RETURNING | Not supported | `RETURNING *` | `RETURNING *` | `RETURNING ... INTO` |
| DDL differences | `ENGINE=InnoDB` | `WITH (OIDS=FALSE)` | No engine | `TABLESPACE` |
| Boolean type | `TINYINT(1)` | `BOOLEAN` | `INTEGER` | `NUMBER(1)` |
| Now() | `NOW()` | `NOW()` | `datetime('now')` | `SYSDATE` |
| Concurrent safety | `GET_LOCK()` | `pg_advisory_lock()` | `BEGIN IMMEDIATE` | `LOCK TABLE` |

### 4.3 Impact on Existing `internal/sqlnorm`

The current `sqlnorm` package checks for `LIMIT`/`OFFSET`/`WHERE` as plain keywords. This works for MySQL, PostgreSQL, and SQLite which all use these keywords. For Oracle and SQL Server, the dialect would need to either:

1. Normalize before analysis (`SanitizeSQL` converting `FETCH FIRST n ROWS ONLY` to `LIMIT n`)
2. Or provide dialect-aware `MatchLimit`/`MatchOffset` methods

Strategy: Keep `sqlnorm` keyword-based for the common subset (MySQL/PG/SQLite), handle Oracle/SQL Server via `SanitizeSQL` normalization at the dialect level.

---

## 5. Implementation Priority

### Phase 1: Driver Abstraction (P0)

| Task | Effort | Description |
|------|--------|-------------|
| Driver registry | 1-2 days | `pkg/db/driver.go`: Register, Open, auto-detect |
| Refactor Session | 1 day | Make `Session` use a `sqlx.DB` wrapper that's backend-agnostic |
| DSN auto-detect | 1 day | Pattern matching for mysql/postgres/sqlite/oracle DSNs |
| Update CLI | 1 day | Add `--driver` flag, wire auto-detection, pass driver to Open |
| Update tests | 1 day | Ensure existing MySQL tests still pass with registry |

**Deliverable**: MySQL works exactly as before, but goes through the driver registry. No functional change. All existing tests pass.

### Phase 2: PostgreSQL Support (P1)

| Task | Effort | Description |
|------|--------|-------------|
| PG driver | 2-3 days | `pkg/db/postgres/driver.go` wrapping `lib/pq` or `pgx` |
| Dialect for PG | 1 day | Placeholder `$N` conversion, RETURNING support |
| LIMIT enforcement | 1 day | PG uses `LIMIT` keyword (same as MySQL) |
| SQL injection | 1 day | Parameterized queries work the same with `$N` placeholders |
| Integration tests | 2 days | Docker PG container for CI testing |

**Dependencies**: Phase 1 complete.

**PG-specific concerns**:
- `lib/pq` does not support `$N` placeholders through `sqlx` automatically — `sqlx.Named()` and rebinding needed.
- `pgx` is the modern alternative but has a different API from `database/sql`.
- The `QueryStream` implementation must handle PG's `COPY` protocol for efficiency.

### Phase 3: SQLite Support (P1)

| Task | Effort | Description |
|------|--------|-------------|
| SQLite driver | 1-2 days | `pkg/db/sqlite/driver.go` wrapping `mattn/go-sqlite3` |
| Dialect for SQLite | 0.5 day | Mostly compatible with MySQL keyword-wise |
| File-based DSN | 0.5 day | Auto-detect from file path or `sqlite://` prefix |
| CGO considerations | 1 day | `mattn/go-sqlite3` requires CGO; document or provide pure-Go alternative |

**SQLite-specific concerns**:
- `mattn/go-sqlite3` requires CGO and a C compiler.
- Pure-Go alternatives: `modernc.org/sqlite` (no CGO, slower).
- Concurrent write limits: SQLite uses file-level locking, not row-level.
- LIMIT/OFFSET syntax matches MySQL — no dialect complexity.

### Phase 4: Oracle Support (P2)

| Task | Effort | Description |
|------|--------|-------------|
| Oracle driver | 3-5 days | `pkg/db/oracle/driver.go` wrapping `godror/godror` |
| Dialect for Oracle | 2-3 days | `FETCH FIRST n ROWS ONLY`, `OFFSET n ROWS`, `:N` placeholders |
| RETURNING INTO | 1 day | Different syntax for transaction-based RETURNING |
| DSN parsing | 1 day | Oracle EZConnect format: `host:port/service` |
| Integration tests | 3 days | Docker Oracle XE is large (~3GB) and slow to start |

**Oracle-specific concerns**:
- `godror` depends on Oracle Instant Client libraries (C dependency).
- LIMIT enforcement requires SQL rewriting: `SELECT * FROM (SELECT a.*, ROWNUM r FROM (query) a) WHERE r <= n` or `FETCH FIRST n ROWS ONLY` (12c+).
- Placeholders use `:name` or `:N` positional syntax.

---

## 6. CLI Interaction Model

### Current
```bash
qc exec <dsn> <sql>
# driver is always "mysql", hardcoded
```

### Target
```bash
# Auto-detect from DSN
qc exec "postgres://user:pass@localhost:5432/db" "CREATE TABLE ..."

# Explicit driver (when auto-detect is ambiguous)
qc exec --driver sqlite "/path/to/db.sqlite" "CREATE TABLE ..."

# Environment variable override
export QC_DRIVER=postgres
qc exec "postgres://user:pass@localhost:5432/db" "SELECT 1"
```

### Migration Path

1. Add `--driver` flag, default `"mysql"` (backward compatible).
2. Add auto-detection logic, fall back to `"mysql"` if unrecognized.
3. When auto-detection is enabled by default (v2.0), drop the implicit `"mysql"` fallback.

---

## 7. Testing Strategy

### Unit Tests
- Driver registry: `TestRegister`, `TestOpen`, `TestUnknownDriver`
- DSN auto-detection: all known DSN formats
- Dialect: placeholder generation, identifier quoting, LIMIT syntax
- Each driver implementation tested with its own mock or test driver

### Integration Tests
- MySQL: existing Docker-based setup (port 3306)
- PostgreSQL: Docker `postgres:16` container (port 5432)
- SQLite: in-memory database (no external process needed)
- Oracle: Docker `gvenzl/oracle-xe` (port 1521, slow, optional CI)

### CI Matrix
```yaml
strategy:
  matrix:
    db: [mysql, postgres, sqlite]
```

---

## 8. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| CGO dependency (SQLite, Oracle) | Build complexity, cross-compilation issues | Offer pure-Go alternatives where available (`modernc.org/sqlite`) |
| Oracle driver licensing | Legal/compliance | Confirm `godror` license compatibility; Oracle Free Use Terms for development |
| SQL syntax differences | LIMIT enforcement may break on Oracle | Dialect layer with `SanitizeSQL` normalization before keyword checks |
| Placeholder mismatch | SQL injection risk if $N not handled | Test all placeholder styles with parameterized queries |
| PG COPY protocol | Streaming optimization gap | Phase 2 streaming uses basic `sql.Rows` iteration; COPY can be optimized later |

---

## 9. Summary Roadmap

```
Phase 1 (v1.1): Driver abstraction + registry
  └─ No visible changes — MySQL works as before

Phase 2 (v1.2): PostgreSQL support
  └─ pgx driver, $N placeholders, RETURNING, Docker integration tests

Phase 3 (v1.3): SQLite support
  └─ File-based DSN, CGO consideration, in-memory testing

Phase 4 (v2.0): Oracle support + auto-detect default
  └─ Oracle XE, FETCH FIRST normalization, DSN auto-detect as default
```
