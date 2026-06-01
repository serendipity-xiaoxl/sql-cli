package db

import (
	"context"
	"fmt"
	"time"

	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/result"
)

// QueryStream executes a SELECT query and returns a StreamResult for row-by-row
// iteration. The same LIMIT enforcement as Query applies (auto-append, capping).
// Columns are available after the first call to StreamResult.Next() returns true.
//
// The caller must call StreamResult.Close() to release resources if iteration is
// stopped early. After Next() returns false, check Err() for any iteration error.
func (s *Session) QueryStream(ctx context.Context, sqlStr string, args ...interface{}) (*result.StreamResult, error) {
	op := sqlnorm.Operation(sqlStr)

	// Reject non-SELECT statements
	if !sqlnorm.IsSELECT(op) {
		return nil, fmt.Errorf("query %s: %w", op, ErrNonSelectQuery)
	}

	// Enforce LIMIT
	hasExistingLimit := sqlnorm.HasLIMIT(sqlStr)
	appliedLimit := 0

	if hasExistingLimit {
		// SQL already has LIMIT — pass through
	} else {
		// Auto-append LIMIT
		appliedLimit = s.cfg.DefaultLimit
		if appliedLimit > s.cfg.MaxLimit {
			appliedLimit = s.cfg.MaxLimit
		}
		sqlStr = sqlnorm.AppendLIMIT(sqlStr, appliedLimit)
	}

	// Acquire concurrency slot
	if err := s.acquireConcurrencySlot(ctx); err != nil {
		return nil, err
	}

	// Build warning (logged after stream completes)
	var warning string
	if !hasExistingLimit {
		if appliedLimit > s.cfg.MaxLimit {
			warning = fmt.Sprintf("LIMIT capped to %d (max allowed)", appliedLimit)
		} else {
			warning = fmt.Sprintf("LIMIT %d applied automatically", appliedLimit)
		}
	}

	// Create buffered channel sized by StreamBatchSize
	bufSize := s.cfg.StreamBatchSize
	if bufSize <= 0 {
		bufSize = 50
	}
	rowChan := make(chan result.StreamRow, bufSize)
	sr := result.NewStreamResult(nil, rowChan)

	start := time.Now()

	go func() {
		defer s.releaseConcurrencySlot()
		defer close(rowChan)
		defer sr.SetProducerDone()

		// Apply query timeout inside the goroutine so the derived context
		// lives as long as the stream, not just until QueryStream returns.
		queryCtx := ctx
		if s.cfg.QueryTimeout > 0 {
			if _, hasDeadline := ctx.Deadline(); !hasDeadline {
				var cancel context.CancelFunc
				queryCtx, cancel = context.WithTimeout(ctx, s.cfg.QueryTimeout)
				defer cancel()
			}
		}

		// Check for context cancellation before executing query
		select {
		case <-queryCtx.Done():
			sr.SetError(queryCtx.Err())
			return
		default:
		}

		rows, err := s.db.QueryxContext(queryCtx, sqlStr, args...)
		if err != nil {
			sr.SetError(fmt.Errorf("query %s: %w", op, err))
			return
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			sr.SetError(fmt.Errorf("query %s: %w", op, err))
			return
		}
		sr.SetColumns(columns)

		rowCount := 0
		for rows.Next() {
			row := make(map[string]interface{})
			if err := rows.MapScan(row); err != nil {
				sr.SetError(fmt.Errorf("query %s: %w", op, err))
				return
			}
		convertRowBytes(row)
			rowCount++

			select {
			case rowChan <- result.StreamRow{
				Row:   row,
				Index: int64(rowCount - 1),
			}:
			case <-sr.Done():
				// Caller closed the stream early
				return
			case <-queryCtx.Done():
				sr.SetError(queryCtx.Err())
				return
			}
		}

		if err := rows.Err(); err != nil {
			sr.SetError(fmt.Errorf("query %s: %w", op, err))
			return
		}

		duration := time.Since(start)
		s.logger.Info("query_stream",
			"duration_ms", duration.Milliseconds(),
			"row_count", rowCount,
			"warning", warning,
		)
	}()

	return sr, nil
}
