package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/guard"
	"github.com/xiaoxl/sql-cli/pkg/result"
)

var (
	// ErrTxDone is returned when an operation is attempted on a committed or
	// rolled back transaction.
	ErrTxDone = errors.New("transaction is already committed or rolled back")
)

// txStatus tracks the lifecycle of a transaction.
type txStatus int

const (
	txActive txStatus = iota
	txCommitted
	txRolledBack
)

// transaction wraps a sqlx.Tx with safety guards and auto-rollback.
type transaction struct {
	tx     *sqlx.Tx
	cfg    *config.Config
	logger *slog.Logger

	mu     sync.Mutex
	status txStatus
	done   chan struct{} // closed when committed or rolled back
}

// newTransaction creates a transaction wrapper and starts auto-rollback.
// The txCtx is a context with the configured timeout — when it expires the
// transaction is automatically rolled back unless already committed.
func newTransaction(tx *sqlx.Tx, cfg *config.Config, logger *slog.Logger, txCtx context.Context, cancel context.CancelFunc) *transaction {
	t := &transaction{
		tx:     tx,
		cfg:    cfg,
		logger: logger,
		done:   make(chan struct{}),
	}

	// Auto-rollback goroutine: fires if the timeout context expires before
	// the transaction is committed or rolled back normally.
	go func() {
		defer cancel()

		select {
		case <-txCtx.Done():
			// Timeout or parent context cancelled
			t.mu.Lock()
			if t.status == txActive {
				if err := t.tx.Rollback(); err != nil {
					t.logger.Warn("transaction auto-rollback failed",
						"error", err,
					)
				} else {
					t.logger.Warn("transaction auto-rolled back due to timeout")
				}
				t.status = txRolledBack
			}
			t.mu.Unlock()
		case <-t.done:
			// Committed or rolled back normally — nothing to do
		}
	}()

	return t
}

// checkActive returns an error if the transaction is no longer active.
func (t *transaction) checkActive() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status != txActive {
		return ErrTxDone
	}
	return nil
}

// markCommitted transitions to committed state and signals the auto-rollback
// goroutine. Must be called BEFORE the actual database commit so that the
// goroutine doesn't race with it.
func (t *transaction) markCommitted() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.status {
	case txCommitted:
		return fmt.Errorf("commit: %w", ErrTxDone)
	case txRolledBack:
		return fmt.Errorf("commit: %w", ErrTxDone)
	case txActive:
		t.status = txCommitted
		close(t.done)
		return nil
	default:
		return fmt.Errorf("commit: unexpected status %v", t.status)
	}
}

// markRolledBack transitions to rolled-back state if still active. Returns
// an error if the transaction was already committed. Idempotent if already
// rolled back.
func (t *transaction) markRolledBack() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.status {
	case txCommitted:
		return ErrTxDone
	case txRolledBack:
		return nil // idempotent
	case txActive:
		t.status = txRolledBack
		close(t.done)
		return nil
	default:
		return ErrTxDone
	}
}

// Commit commits the transaction. Returns an error if already done.
func (t *transaction) Commit(ctx context.Context) error {
	if err := t.markCommitted(); err != nil {
		return err
	}

	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	t.logger.Info("transaction committed")
	return nil
}

// Rollback rolls back the transaction. Returns an error if already done.
func (t *transaction) Rollback(ctx context.Context) error {
	if err := t.markRolledBack(); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}

	if err := t.tx.Rollback(); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}

	t.logger.Info("transaction rolled back")
	return nil
}

// Exec executes a DDL or DML statement within the transaction.
func (t *transaction) Exec(ctx context.Context, sqlStr string, args ...interface{}) (*result.ExecResult, error) {
	if err := t.checkActive(); err != nil {
		return nil, fmt.Errorf("exec %s: %w", sqlnorm.Operation(sqlStr), err)
	}

	op := sqlnorm.Operation(sqlStr)

	// Guard: dangerous operations (DROP, TRUNCATE)
	if err := guard.Check(t.cfg.DangerousOpPolicy, sqlStr); err != nil {
		return nil, fmt.Errorf("exec %s: %w", op, err)
	}

	// Guard: UPDATE/DELETE without WHERE
	if t.cfg.RejectNoWhere && sqlnorm.RequiresWHERE(op) && !sqlnorm.HasWHERE(sqlStr) {
		return nil, fmt.Errorf("exec %s: %w", op, ErrUnconditionalModify)
	}

	start := time.Now()
	res, err := t.tx.ExecContext(ctx, sqlStr, args...)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("exec %s: %w", op, err)
	}

	lastID, _ := res.LastInsertId()
	rowsAffected, _ := res.RowsAffected()

	t.logger.Info("tx exec",
		"op", op,
		"duration_ms", duration.Milliseconds(),
		"rows_affected", rowsAffected,
		"last_insert_id", lastID,
	)

	return result.NewExecResult(lastID, rowsAffected, duration), nil
}

// Query executes a SELECT query within the transaction with LIMIT enforcement.
func (t *transaction) Query(ctx context.Context, sqlStr string, args ...interface{}) (*result.QueryResult, error) {
	if err := t.checkActive(); err != nil {
		return nil, fmt.Errorf("query %s: %w", sqlnorm.Operation(sqlStr), err)
	}

	op := sqlnorm.Operation(sqlStr)

	// Reject non-SELECT
	if !sqlnorm.IsSELECT(op) {
		return nil, fmt.Errorf("query %s: %w", op, ErrNonSelectQuery)
	}

	// Enforce LIMIT
	hasExistingLimit := sqlnorm.HasLIMIT(sqlStr)
	appliedLimit := 0

	if hasExistingLimit {
		// Pass through
	} else {
		appliedLimit = t.cfg.DefaultLimit
		if appliedLimit > t.cfg.MaxLimit {
			appliedLimit = t.cfg.MaxLimit
		}
		sqlStr = sqlnorm.AppendLIMIT(sqlStr, appliedLimit)
	}

	// Build warning
	var warning string
	if !hasExistingLimit {
		if appliedLimit > t.cfg.MaxLimit {
			warning = fmt.Sprintf("LIMIT capped to %d (max allowed)", appliedLimit)
		} else {
			warning = fmt.Sprintf("LIMIT %d applied automatically", appliedLimit)
		}
	}

	start := time.Now()
	rows, err := t.tx.QueryxContext(ctx, sqlStr, args...)
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
	hasMore := appliedLimit > 0 && len(resultRows) >= appliedLimit

	t.logger.Info("tx query",
		"duration_ms", duration.Milliseconds(),
		"row_count", len(resultRows),
		"warning", warning,
		"has_more", hasMore,
	)

	res := result.NewQueryResult(columns, resultRows, duration, warning)
	res.HasMore = hasMore
	return res, nil
}

// QueryWithOffset executes a SELECT with pagination within the transaction.
func (t *transaction) QueryWithOffset(ctx context.Context, sqlStr string, limit, offset int, args ...interface{}) (*result.QueryResult, error) {
	if err := t.checkActive(); err != nil {
		return nil, fmt.Errorf("query %s: %w", sqlnorm.Operation(sqlStr), err)
	}

	op := sqlnorm.Operation(sqlStr)

	// Reject non-SELECT
	if !sqlnorm.IsSELECT(op) {
		return nil, fmt.Errorf("query %s: %w", op, ErrNonSelectQuery)
	}

	// Enforce LIMIT
	hasExistingLimit := sqlnorm.HasLIMIT(sqlStr)
	appliedLimit := 0

	if hasExistingLimit {
		// Pass through
	} else {
		appliedLimit = limit
		if appliedLimit <= 0 {
			appliedLimit = t.cfg.DefaultLimit
		}
		if appliedLimit > t.cfg.MaxLimit {
			appliedLimit = t.cfg.MaxLimit
		}
		sqlStr = sqlnorm.AppendLIMIT(sqlStr, appliedLimit)
	}

	// Enforce OFFSET
	hasExistingOffset := sqlnorm.HasOFFSET(sqlStr)
	if !hasExistingOffset && offset > 0 {
		sqlStr = sqlnorm.AppendOFFSET(sqlStr, offset)
	}

	// Build warning
	var warning string
	if !hasExistingLimit {
		if appliedLimit > t.cfg.MaxLimit {
			warning = fmt.Sprintf("LIMIT capped to %d (max allowed)", appliedLimit)
		} else {
			warning = fmt.Sprintf("LIMIT %d applied automatically", appliedLimit)
		}
	}

	start := time.Now()
	rows, err := t.tx.QueryxContext(ctx, sqlStr, args...)
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
	hasMore := appliedLimit > 0 && len(resultRows) >= appliedLimit

	t.logger.Info("tx query_with_offset",
		"duration_ms", duration.Milliseconds(),
		"row_count", len(resultRows),
		"offset", offset,
		"warning", warning,
		"has_more", hasMore,
	)

	res := result.NewQueryResult(columns, resultRows, duration, warning)
	res.HasMore = hasMore
	return res, nil
}
