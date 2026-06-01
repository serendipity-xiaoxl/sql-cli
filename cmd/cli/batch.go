package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/xiaoxl/sql-cli/pkg/db"
)

// StmtResult holds the result of executing a single SQL statement in a batch.
// The Statement field records which SQL was executed.
type StmtResult struct {
	Statement    string `json:"statement"`
	LastInsertID int64  `json:"last_insert_id,omitempty"`
	RowsAffected int64  `json:"rows_affected"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

// batchExec executes multiple SQL statements sequentially and returns a per-
// statement result array. When useTransaction is true, all statements are
// wrapped in a single transaction that commits on success and rolls back on
// any failure (or continues if continueOnError is set). When useTransaction
// is false, each statement runs independently.
func batchExec(ctx context.Context, database *db.Session, statements []string, useTransaction, continueOnError bool) []*StmtResult {
	if useTransaction {
		return batchExecInTx(ctx, database, statements, continueOnError)
	}
	return batchExecDirect(ctx, database, statements, continueOnError)
}

func batchExecInTx(ctx context.Context, database *db.Session, statements []string, continueOnError bool) []*StmtResult {
	tx, err := database.Begin(ctx)
	if err != nil {
		return []*StmtResult{{
			Statement: "",
			Error:     fmt.Sprintf("begin transaction: %v", err),
		}}
	}

	results := make([]*StmtResult, 0, len(statements))
	var firstErr error

	for i, stmt := range statements {
		res, err := tx.Exec(ctx, stmt)
		r := &StmtResult{Statement: stmt, DurationMs: 0}
		if err != nil {
			r.Error = err.Error()
			if firstErr == nil {
				firstErr = err
			}
			if !continueOnError {
				if rbErr := tx.Rollback(ctx); rbErr != nil {
					slog.Warn("transaction rollback after statement failure failed",
						"statement", i+1,
						"exec_error", err,
						"rollback_error", rbErr,
					)
				}
				results = append(results, r)
				slog.Warn("batch statement failed, stopping", "statement", i+1, "error", err)
				break
			}
			slog.Warn("batch statement failed in transaction (continuing)", "statement", i+1, "error", err)
		} else {
			r.LastInsertID = res.LastInsertID
			r.RowsAffected = res.RowsAffected
			r.DurationMs = res.DurationMs
		}
		results = append(results, r)
	}

	// If there were errors and we didn't already rollback, rollback now
	if firstErr != nil && !continueOnError {
		// already rolled back inside the loop
		_ = 0
	} else if firstErr != nil {
		// continueOnError was true — rollback the whole transaction
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			slog.Warn("transaction rollback after partial failure failed",
				"rollback_error", rbErr,
			)
		}
	} else {
		if err := tx.Commit(ctx); err != nil {
			results = append(results, &StmtResult{
				Statement: "",
				Error:     fmt.Sprintf("commit: %v", err),
			})
		}
	}

	return results
}

func batchExecDirect(ctx context.Context, database *db.Session, statements []string, continueOnError bool) []*StmtResult {
	results := make([]*StmtResult, 0, len(statements))

	for i, stmt := range statements {
		res, err := database.Exec(ctx, stmt)
		r := &StmtResult{Statement: stmt}
		if err != nil {
			r.Error = err.Error()
			if !continueOnError {
				results = append(results, r)
				slog.Warn("batch statement failed, stopping", "statement", i+1, "error", err)
				break
			}
			slog.Warn("batch statement failed (continuing)", "statement", i+1, "error", err)
		} else {
			r.LastInsertID = res.LastInsertID
			r.RowsAffected = res.RowsAffected
			r.DurationMs = res.DurationMs
		}
		results = append(results, r)
	}

	return results
}
