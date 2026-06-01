package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
	"github.com/xiaoxl/sql-cli/pkg/dsn"
	"github.com/xiaoxl/sql-cli/pkg/guard"
	"github.com/xiaoxl/sql-cli/pkg/result"

	_ "github.com/xiaoxl/sql-cli/pkg/db/postgres"
)

var (
	limit           int
	offset          int
	timeout         time.Duration
	version         bool
	filePath        string
	useTransaction  bool
	continueOnError bool
	force           bool
	forceDeprecated bool
	driverFlag      string
)

func init() {
	flag.IntVar(&limit, "limit", 0, "query row limit (0=default)")
	flag.IntVar(&offset, "offset", 0, "query row offset")
	flag.DurationVar(&timeout, "timeout", 0, "query timeout (0=default)")
	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.StringVar(&filePath, "file", "", "SQL file to execute (batch mode)")
	flag.StringVar(&filePath, "f", "", "SQL file to execute (shorthand)")
	flag.BoolVar(&useTransaction, "transaction", false, "wrap batch statements in a transaction")
	flag.BoolVar(&continueOnError, "continue-on-error", false, "continue batch after statement failures")
	flag.BoolVar(&force, "force", false, "skip confirmation prompts for dangerous operations")
	flag.BoolVar(&forceDeprecated, "yes", false, "alias for --force")
	flag.StringVar(&driverFlag, "driver", "", "database driver (auto-detected from DSN if not set)")
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
	dsnStr := args[1]
	if dsnStr == "" {
		dsnStr = os.Getenv("SQL_CLI_DSN")
	}
	if dsnStr == "" {
		fmt.Fprintf(os.Stderr, "DSN is required as argument or via SQL_CLI_DSN env var\n")
		os.Exit(1)
	}
	sql := ""
	if len(args) > 2 {
		sql = args[2]
	}

	driver := driverFlag
	if driver == "" {
		var detErr error
		driver, detErr = dsn.Detect(dsnStr)
		if detErr != nil {
			fatal("detect driver: %v", detErr)
		}
	}

	opts := []config.Option{}
	if timeout > 0 {
		opts = append(opts, config.WithQueryTimeout(timeout))
	}

	sess, err := db.Open(driver, dsnStr, opts...)
	if err != nil {
		fatal("open session: %v", err)
	}
	defer sess.Close()

	ctx := context.Background()

	confirmed := force || forceDeprecated

	switch cmd {
	case "ping":
		if err := sess.Ping(ctx); err != nil {
			fatal("ping: %v", err)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ok"})

	case "exec":
		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				fatal("exec: reading file: %v", err)
			}
			statements := sqlnorm.SplitStatements(string(data))
			if len(statements) == 0 {
				fatal("exec: no SQL statements found in %s", filePath)
			}
			if confirmed {
				sess.Config().DangerousOpPolicy = guard.PolicyAllow
			}
			results := batchExec(ctx, sess, statements, useTransaction, continueOnError)
			json.NewEncoder(os.Stdout).Encode(results)
			break
		}
		res, err := execWithConfirmation(ctx, sess, sql)
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

func execWithConfirmation(ctx context.Context, sess *db.Session, sql string) (*result.ExecResult, error) {
	res, err := sess.Exec(ctx, sql)
	if err == nil || !errors.Is(err, guard.ErrDangerousOpPrompt) {
		return res, err
	}

	if force || forceDeprecated {
		sess.Config().DangerousOpPolicy = guard.PolicyAllow
		return sess.Exec(ctx, sql)
	}

	fmt.Fprintf(os.Stderr, "WARNING: %q\nType 'yes' to confirm: ", sql)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		sess.Config().DangerousOpPolicy = guard.PolicyAllow
		return sess.Exec(ctx, sql)
	}

	return nil, fmt.Errorf("dangerous operation cancelled by user: %s", sql)
}

func usage() {
	fmt.Fprintf(os.Stderr, `sql-cli -- MySQL client for AI Agents

Usage:
  sql-cli ping   <dsn>
  sql-cli exec   <dsn> [<sql>]   [--file <path>] [--transaction] [--continue-on-error] [--timeout D]
  sql-cli query  <dsn> [<sql>]   [--limit N] [--offset N] [--timeout D]
  sql-cli stream <dsn> <sql>     [--limit N] [--timeout D]

Commands:
  exec --file <path>    Read and execute SQL statements from a file (batch mode)
  exec --transaction    Wrap all batch statements in a single transaction
  exec --continue-on-error  Continue batch execution after statement failures

Flags:
  --limit int           query row limit (0=default)
  --offset int          query row offset
  --timeout d           query timeout (0=default, e.g. 30s)
  --file, -f <path>     SQL file to execute (batch mode, exec only)
  --transaction         wrap batch statements in a transaction (exec --file only)
  --continue-on-error   continue batch after statement failures (exec --file only)
  --driver <name>       database driver (auto-detected from DSN if not set)
  --force               skip confirmation prompts for dangerous operations
  --yes                 alias for --force
  --version             print version and exit

Environment:
  SQL_CLI_DSN           default DSN if not provided as argument
`)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
