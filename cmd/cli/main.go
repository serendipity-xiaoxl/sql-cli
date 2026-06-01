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
	_ "github.com/xiaoxl/sql-cli/pkg/db/sqlite"
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

const appVersion = "0.2.0"

type qcEnv struct {
	dsn    string
	driver string
}

func loadDotEnv() qcEnv {
	var env qcEnv
	data, err := os.ReadFile(".env")
	if err != nil {
		return env
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "QC_DSN":
			env.dsn = val
		case "QC_DRIVER":
			env.driver = val
		}
	}
	return env
}

func main() {
	flag.Parse()

	if version {
		fmt.Fprintf(os.Stderr, "qc version %s\n", appVersion)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	qcFile := loadDotEnv()

	cmd := args[0]

	// Resolve DSN from env / .env first, to decide arg positions.
	envDSN := os.Getenv("QC_DSN")
	if envDSN == "" {
		envDSN = qcFile.dsn
	}

	var dsnStr, sql string

	hasDSNArg := len(args) > 1 && args[1] != ""
	needsSQL := cmd == "exec" || cmd == "query" || cmd == "stream"

	switch {
	case len(args) == 0:
		// handled above
	case hasDSNArg && looksLikeDSN(args[1]):
		// Explicit DSN provided: args[1] is DSN, args[2] is SQL
		dsnStr = args[1]
		if len(args) > 2 {
			sql = args[2]
		}
	case !hasDSNArg && envDSN != "" && needsSQL:
		// No DSN arg, env has DSN → only SQL is expected
		dsnStr = envDSN
		// no SQL from args
	case hasDSNArg && envDSN != "" && needsSQL:
		// One arg + env DSN → arg is SQL, not DSN
		dsnStr = envDSN
		sql = args[1]
	case hasDSNArg:
		// Single arg, no env DSN → treat as DSN
		dsnStr = args[1]
	case envDSN != "":
		dsnStr = envDSN
	default:
		fmt.Fprintf(os.Stderr, "DSN is required as argument, via QC_DSN env var, or .env file\n")
		os.Exit(1)
	}

	if dsnStr == "" {
		fmt.Fprintf(os.Stderr, "DSN is required as argument, via QC_DSN env var, or .env file\n")
		os.Exit(1)
	}

	// Driver: CLI flag > .env file > DSN auto-detect
	driver := driverFlag
	if driver == "" {
		driver = qcFile.driver
	}
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
	fmt.Fprintf(os.Stderr, `qc — multi-database CLI for AI Agents (MySQL, PostgreSQL, SQLite)

Usage:
  qc ping                     # DSN from env or .env
  qc ping   <dsn>
  qc exec   [dsn] [<sql>]     [--file <path>] [--transaction] [--continue-on-error]
  qc query  [dsn] [<sql>]     [--limit N] [--offset N] [--timeout D]
  qc stream [dsn] <sql>       [--limit N] [--timeout D]

Flags:
  --driver <name>       database driver (mysql/postgres/sqlite, auto-detected)
  --limit int           query row limit (0=default 100)
  --offset int          query row offset
  --timeout d           query timeout (0=default 30s)
  --file, -f <path>     SQL file to execute (exec only)
  --transaction         wrap batch in a single transaction
  --continue-on-error   continue after statement failures
  --force               skip dangerous operation confirmation
  --version             print version and exit

Config (.env in current directory):
  QC_DSN=user:pass@tcp(host:3306)/db
  QC_DRIVER=mysql

Priority: CLI arguments > environment variables > .env file
`)
}

// looksLikeDSN returns true if s looks like a database connection string.
func looksLikeDSN(s string) bool {
	return strings.Contains(s, "@tcp(") ||
		strings.Contains(s, "@unix(") ||
		strings.HasPrefix(s, "postgres://") ||
		strings.HasPrefix(s, "postgresql://") ||
		strings.HasPrefix(s, "mysql://") ||
		strings.HasPrefix(s, "file:") ||
		strings.HasPrefix(s, "/") ||
		s == ":memory:"
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
