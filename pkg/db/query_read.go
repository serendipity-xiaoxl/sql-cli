package db

import (
	"context"
	"fmt"
	"time"

	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/result"
)

// ErrNotReadOperation is returned when QueryRead is called with a non-read statement.
var ErrNotReadOperation = fmt.Errorf("only read operations are allowed (SELECT, SHOW, DESCRIBE, EXPLAIN)")

// QueryRead executes a read-only query (SELECT, SHOW, DESCRIBE, EXPLAIN, etc.)
// and returns the result set. Unlike Query(), it does not auto-append LIMIT
// for non-SELECT operations. Designed for interactive shell use.
func (s *Session) QueryRead(ctx context.Context, sqlStr string, args ...interface{}) (*result.QueryResult, error) {
	if s.cfg.QueryTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.cfg.QueryTimeout)
			defer cancel()
		}
	}

	op := sqlnorm.Operation(sqlStr)
	if !sqlnorm.IsReadOperation(op) {
		return nil, fmt.Errorf("query %s: %w", op, ErrNotReadOperation)
	}

	// Only enforce LIMIT for SELECT/WITH
	if sqlnorm.IsSELECT(op) && !sqlnorm.HasLIMIT(sqlStr) {
		limit := s.cfg.DefaultLimit
		if limit > s.cfg.MaxLimit {
			limit = s.cfg.MaxLimit
		}
		sqlStr = sqlnorm.AppendLIMIT(sqlStr, limit)
	}

	sqlStr = s.Rebind(sqlStr)
	start := time.Now()
	rows, err := s.db.QueryxContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	var resultRows [][]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}
		convertRowBytes(row)
		rowSlice := make([]interface{}, len(columns))
		for i, col := range columns {
			rowSlice[i] = row[col]
		}
		resultRows = append(resultRows, rowSlice)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	duration := time.Since(start)

	s.logger.Info("query_read",
		"duration_ms", duration.Milliseconds(),
		"row_count", len(resultRows),
		"operation", op,
	)

	res := result.NewQueryResult(columns, resultRows, duration, "")
	return res, nil
}
