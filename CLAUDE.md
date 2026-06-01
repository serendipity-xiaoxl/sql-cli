# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go library (and optional CLI) for MySQL database management, designed to be called by AI Agents. Provides connection management, DDL/DML execution, manual transactions, safe read-only queries with forced LIMIT, and streaming queries.

## Tech Stack

- **Language**: Go (latest stable)
- **Database driver**: `github.com/go-sql-driver/mysql`
- **Optional deps**: `github.com/jmoiron/sqlx` (named params, struct mapping), `github.com/pkg/errors` (error wrapping)
- **Logging**: `log/slog` (stdlib structured logging)

## Architecture

### Core Interface

A `DB` interface abstracts all database operations (Connect, Close, Exec, Query, Begin, Ping), allowing future implementations for PostgreSQL, SQLite, etc. MySQL is the first implementation.

### Package Layout (planned)

```
cmd/            # CLI entry point (optional)
pkg/            # Public API
  db.go         # DB interface definition
  session.go    # Session struct, Open/Close
  executor.go   # Exec (DDL/DML)
  query.go      # Query (safe SELECT with forced LIMIT)
  stream.go     # QueryStream (streaming iteration)
  transaction.go # Begin/Commit/Rollback
  options.go    # Functional options pattern
  result.go     # Structured result types
internal/       # Private implementation
  mysql/        # MySQL driver implementation
  limit/        # LIMIT enforcement logic
  safety/       # Dangerous operation interception
```

### Key Design Decisions

- **Functional options pattern** for configuration (MaxConns, IdleConns, MaxLifetime, MaxLimit, QueryTimeout, StreamBufferSize)
- **Multi-session**: manage multiple DB connections by name
- **Context-driven**: every method accepts `context.Context` for timeout/cancellation
- **Structured JSON results**: all query results include columns, rows, row_count, duration_ms, warning
- **LIMIT enforcement**: SELECT without LIMIT auto-appends `LIMIT <default>` (configurable), with configurable max limit cap
- **Unsafe operation guard**: configurable blacklist/whitelist for DROP, TRUNCATE, etc.
- **DELETE/UPDATE without WHERE protection**: configurable rejection
- **Transaction timeout**: automatic rollback on timeout
- **Streaming**: returns iterator (Next/Scan/Err/Close) with cancel support, respecting LIMIT

## Commands

```bash
# Build
go build ./...

# Run tests
go test ./...

# Run a single test
go test -run TestName ./pkg/...

# Run tests with coverage
go test -cover ./...

# Lint
go vet ./...

# Generate mocks (when mockgen is added)
go generate ./...
```

## Team Workflow

This project uses a multi-agent team. See `doc/agent-team-init.md` for the full specification.

Workflow: Product Manager → Developer → Test Engineer → Code Reviewer → Commit
