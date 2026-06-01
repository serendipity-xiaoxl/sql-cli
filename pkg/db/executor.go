package db

import (
	"context"
	"fmt"
	"time"

	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/guard"
	"github.com/xiaoxl/sql-cli/pkg/result"
)

// ErrUnconditionalModify is returned when UPDATE/DELETE lacks a WHERE clause.
var ErrUnconditionalModify = fmt.Errorf("UPDATE/DELETE without WHERE clause")

// Exec executes a DDL or DML statement and returns the result.
// Supported operations: CREATE TABLE, ALTER TABLE, INSERT, UPDATE, DELETE.
//
// DELETE and UPDATE without a WHERE clause are rejected when RejectNoWhere is true.
// DROP and TRUNCATE are subject to the DangerousOpPolicy configuration.
func (s *Session) Exec(ctx context.Context, sqlStr string, args ...interface{}) (*result.ExecResult, error) {
	// Apply query timeout from config if no deadline is set
	if s.cfg.QueryTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.cfg.QueryTimeout)
			defer cancel()
		}
	}

	op := sqlnorm.Operation(sqlStr)

	// Guard: check dangerous operations (DROP, TRUNCATE)
	if err := guard.Check(s.cfg.DangerousOpPolicy, sqlStr); err != nil {
		return nil, fmt.Errorf("exec %s: %w", op, err)
	}

	// Guard: check UPDATE/DELETE without WHERE
	if s.cfg.RejectNoWhere && sqlnorm.RequiresWHERE(op) && !sqlnorm.HasWHERE(sqlStr) {
		return nil, fmt.Errorf("exec %s: %w", op, ErrUnconditionalModify)
	}

	sqlStr = s.Rebind(sqlStr)

	start := time.Now()
	res, err := s.db.ExecContext(ctx, sqlStr, args...)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("exec %s: %w", op, err)
	}

	lastID, _ := res.LastInsertId()
	rowsAffected, _ := res.RowsAffected()

	s.logger.Info("exec",
		"op", op,
		"duration_ms", duration.Milliseconds(),
		"rows_affected", rowsAffected,
		"last_insert_id", lastID,
	)

	return result.NewExecResult(lastID, rowsAffected, duration), nil
}
