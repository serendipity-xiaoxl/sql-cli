package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/db"
	"github.com/xiaoxl/sql-cli/pkg/guard"
	"github.com/xiaoxl/sql-cli/pkg/result"
)

// ShellOutput wraps a statement execution result for JSON output.
type ShellOutput struct {
	Statement string      `json:"statement"`
	Type      string      `json:"type"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
}

func runShell(sess *db.Session, limit, offset int, force bool) error {
	interactive := isTerminal()

	if interactive {
		fmt.Fprintf(os.Stderr, "qc shell (type 'exit' or 'quit' to leave, Ctrl+D to exit)\n")
		fmt.Fprint(os.Stderr, "qc> ")
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "--") {
			if interactive {
				fmt.Fprint(os.Stderr, "qc> ")
			}
			continue
		}

		statements := sqlnorm.SplitStatements(line)
		for _, stmt := range statements {
			lower := strings.ToLower(stmt)
			if lower == "exit" || lower == "quit" {
				return nil
			}
			output := executeStmt(sess, stmt, limit, offset, force)
			json.NewEncoder(os.Stdout).Encode(output)
		}

		if interactive {
			fmt.Fprint(os.Stderr, "qc> ")
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	return nil
}

func executeStmt(sess *db.Session, sql string, limit, offset int, force bool) ShellOutput {
	ctx := context.Background()
	op := sqlnorm.Operation(sql)

	if sqlnorm.IsReadOperation(op) {
		return executeQuery(sess, ctx, sql, op, limit, offset)
	}
	return executeExec(sess, ctx, sql, force)
}

func executeQuery(sess *db.Session, ctx context.Context, sql, op string, limit, offset int) ShellOutput {
	var (
		res *result.QueryResult
		err error
	)

	if sqlnorm.IsSELECT(op) {
		res, err = sess.QueryWithOptions(ctx, sql, db.QueryOptions{Limit: limit, Offset: offset})
	} else {
		res, err = sess.QueryRead(ctx, sql)
	}

	if err != nil {
		return ShellOutput{
			Statement: sql,
			Type:      "query",
			Error:     err.Error(),
		}
	}

	return ShellOutput{
		Statement: sql,
		Type:      "query",
		Result:    res,
	}
}

func executeExec(sess *db.Session, ctx context.Context, sql string, force bool) ShellOutput {
	if force {
		sess.Config().DangerousOpPolicy = guard.PolicyAllow
	}

	res, err := sess.Exec(ctx, sql)
	if err != nil {
		return ShellOutput{
			Statement: sql,
			Type:      "exec",
			Error:     err.Error(),
		}
	}

	return ShellOutput{
		Statement: sql,
		Type:      "exec",
		Result:    res,
	}
}

func isTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
