package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
)

var (
	limit   int
	offset  int
	timeout time.Duration
	version bool
)

func init() {
	flag.IntVar(&limit, "limit", 0, "query row limit (0=default)")
	flag.IntVar(&offset, "offset", 0, "query row offset")
	flag.DurationVar(&timeout, "timeout", 0, "query timeout (0=default)")
	flag.BoolVar(&version, "version", false, "print version and exit")
}

const appVersion = "0.1.0"

func main() {
	flag.Parse()

	if version {
		fmt.Fprintf(os.Stderr, "sql-cli version %s\n", appVersion)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := args[0]
	dsn := args[1]
	if dsn == "" {
		dsn = os.Getenv("SQL_CLI_DSN")
	}
	if dsn == "" {
		fmt.Fprintf(os.Stderr, "DSN is required as argument or via SQL_CLI_DSN env var\n")
		os.Exit(1)
	}
	sql := ""
	if len(args) > 2 {
		sql = args[2]
	}

	opts := []config.Option{}
	if timeout > 0 {
		opts = append(opts, config.WithQueryTimeout(timeout))
	}

	sess, err := db.Open("mysql", dsn, opts...)
	if err != nil {
		fatal("open session: %v", err)
	}
	defer sess.Close()

	ctx := context.Background()

	switch cmd {
	case "ping":
		if err := sess.Ping(ctx); err != nil {
			fatal("ping: %v", err)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ok"})

	case "exec":
		res, err := sess.Exec(ctx, sql)
		if err != nil {
			fatal("exec: %v", err)
		}
		json.NewEncoder(os.Stdout).Encode(res)

	case "query":
		var res interface{}
		var err error
		if offset > 0 {
			res, err = sess.QueryWithOffset(ctx, sql, limit, offset)
		} else if limit > 0 {
			res, err = sess.QueryWithLimit(ctx, sql, limit)
		} else {
			res, err = sess.Query(ctx, sql)
		}
		if err != nil {
			fatal("query: %v", err)
		}
		json.NewEncoder(os.Stdout).Encode(res)

	case "stream":
		sr, err := sess.QueryStream(ctx, sql)
		if err != nil {
			fatal("stream: %v", err)
		}
		enc := json.NewEncoder(os.Stdout)
		for sr.Next() {
			if err := enc.Encode(sr.Scan()); err != nil {
				fatal("encode: %v", err)
			}
		}
		if err := sr.Err(); err != nil {
			fatal("stream: %v", err)
		}
		sr.Close()

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `sql-cli — MySQL client for AI Agents

Usage:
  sql-cli ping   <dsn>
  sql-cli exec   <dsn> <sql>
  sql-cli query  <dsn> [sql]   [--limit N] [--offset N] [--timeout D]
  sql-cli stream <dsn> <sql>   [--limit N] [--timeout D]

Flags:
  --limit int    query row limit (0=default)
  --offset int   query row offset
  --timeout d    query timeout (0=default, e.g. 30s)
  --version      print version and exit

Environment:
  SQL_CLI_DSN    default DSN if not provided as argument
`)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
