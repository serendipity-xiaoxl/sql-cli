# qc

A multi-database CLI for MySQL, PostgreSQL, and SQLite. All output is JSON. Built-in safety guards. Designed for AI Agents and developers.

## Quick Start

```bash
# Install
make build

# Configure (once)
echo 'QC_DSN=user:pass@tcp(127.0.0.1:3306)/db' > .env
echo 'QC_DRIVER=mysql' >> .env

# Use
qc ping
qc exec "CREATE TABLE users (id INT, name TEXT)"
qc exec "INSERT INTO users VALUES (1, 'Alice')"
qc query "SELECT * FROM users"
qc shell  # interactive REPL
```

## Commands

| Command | Description |
|---------|-------------|
| `qc ping [dsn]` | Test connection |
| `qc exec [dsn] <sql>` | Execute DDL/DML |
| `qc query [dsn] <sql>` | Execute SELECT |
| `qc stream [dsn] <sql>` | Stream large results |
| `qc shell [dsn]` | Interactive REPL (persistent connection) |

All global flags go before the command: `qc --driver sqlite3 query ":memory:" "SELECT 1"`

## Shell Mode

```bash
# Interactive
qc shell <dsn>

# Pipeline (AI Agent)
echo "SELECT * FROM users; INSERT INTO logs VALUES(1);" | qc shell <dsn>

# Multi-statement transaction
echo "BEGIN; INSERT INTO t VALUES(1); COMMIT;" | qc shell <dsn>
```

## Safety

- Auto LIMIT 100 on queries without one (max 1000)
- DROP/TRUNCATE require confirmation or `--force`
- UPDATE/DELETE without WHERE are blocked
- Query timeout 30s default

## Supported Databases

| Database | DSN Format |
|----------|------------|
| MySQL | `user:pass@tcp(host:port)/db` |
| PostgreSQL | `postgres://user:pass@host:port/db` |
| SQLite | `/path/to/file.db` or `:memory:` |

## Install

Download from [Releases](https://github.com/serendipity-xiaoxl/sql-cli/releases) or build from source:

```bash
git clone https://github.com/serendipity-xiaoxl/sql-cli.git
cd sql-cli
make build      # → ./qc
```

## Go Library

```go
import "github.com/xiaoxl/sql-cli/pkg/db"

sess, _ := db.Open("mysql", dsn)
defer sess.Close()
sess.Exec(ctx, "INSERT INTO users VALUES (?)", "alice")
result, _ := sess.Query(ctx, "SELECT * FROM users")
```

## Documentation

- [English Manual](doc/operation-manual.md)
- [中文手册](doc/operation-manual-zh.md)
- [CLAUDE.md](CLAUDE.md) — architecture & development guide

## License

MIT
