package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/guard"
)

// runTxClose is a helper that cleans up a transaction test: ensures the
// transaction is rolled back (if still active), closes the session, and
// checks mock expectations.
func runTxClose(t *testing.T, mock sqlmock.Sqlmock, s *Session, tx Tx) {
	t.Helper()
	_ = tx.Rollback(context.Background())
	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxBeginCommit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectCommit()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	res, err := tx.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Alice")
	if err != nil {
		t.Fatalf("tx.Exec() error = %v", err)
	}
	if res.LastInsertID != 42 {
		t.Errorf("LastInsertID = %d, want 42", res.LastInsertID)
	}
	if res.RowsAffected != 1 {
		t.Errorf("RowsAffected = %d, want 1", res.RowsAffected)
	}

	if err := tx.Commit(context.Background()); err != nil {
		t.Errorf("Commit() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxBeginRollback(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = tx.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Bob")
	if err != nil {
		t.Fatalf("tx.Exec() error = %v", err)
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxCommitAfterRollbackReturnsError(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	// Commit after Rollback must fail
	if err := tx.Commit(context.Background()); err == nil {
		t.Error("Commit() expected error after Rollback, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxDoubleCommitReturnsError(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectCommit()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	if err := tx.Commit(context.Background()); err != nil {
		t.Errorf("First Commit() error = %v", err)
	}

	// Second Commit must fail
	if err := tx.Commit(context.Background()); err == nil {
		t.Error("Second Commit() expected error, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxDoubleRollbackReturnsError(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("First Rollback() error = %v", err)
	}

	// Second Rollback must return ErrTxDone
	if err := tx.Rollback(context.Background()); err == nil {
		t.Error("Second Rollback() expected error, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxExecAfterRollbackReturnsError(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	// Exec after Rollback must fail
	_, err = tx.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Test")
	if err == nil {
		t.Error("Exec() expected error after Rollback, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxQueryWithinTransaction(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM users WHERE id = \\? LIMIT 100").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Alice"))
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	res, err := tx.Query(context.Background(), "SELECT * FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("tx.Query() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if len(res.Rows) != 1 {
		t.Errorf("len(Rows) = %d, want 1", len(res.Rows))
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxQueryRejectsNonSelect(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = tx.Query(context.Background(), "INSERT INTO users (name) VALUES ('test')")
	if err == nil {
		t.Error("tx.Query() expected error for INSERT, got nil")
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxExecBlocksDrop(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = tx.Exec(context.Background(), "DROP TABLE users")
	if err == nil {
		t.Error("tx.Exec() expected error for DROP in default config, got nil")
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxExecBlocksNoWhereDelete(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = tx.Exec(context.Background(), "DELETE FROM users")
	if err == nil {
		t.Error("tx.Exec() expected error for DELETE without WHERE, got nil")
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxAutoRollbackOnTimeout(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.QueryTimeout = 10 * time.Millisecond
	s, mock := newMockSession(t, cfg)

	mock.ExpectBegin()
	mock.ExpectRollback() // auto-rollback will call this when timeout fires

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	// Wait for auto-rollback to fire
	time.Sleep(50 * time.Millisecond)

	// Operations should now fail because the transaction was auto-rolled-back
	_, err = tx.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Late")
	if err == nil {
		t.Error("Exec() expected error after auto-rollback, got nil")
	}

	// Rollback after auto-rollback returns ErrTxDone
	if err := tx.Rollback(context.Background()); err == nil {
		t.Error("Rollback() after auto-rollback expected error, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxExecErrorPropagation(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = tx.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Fail")
	if err == nil {
		t.Fatal("tx.Exec() expected error, got nil")
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxQueryAppliesLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	// Query without LIMIT should have it auto-appended
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "AutoLimit"))
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	res, err := tx.Query(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("tx.Query() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning == "" {
		t.Error("Warning expected for auto-appended LIMIT, got empty")
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxBeginFail(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin().WillReturnError(sql.ErrConnDone)

	_, err := s.Begin(context.Background())
	if err == nil {
		t.Fatal("Begin() expected error, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxCommitFailure(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(sql.ErrConnDone)

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	if err := tx.Commit(context.Background()); err == nil {
		t.Error("Commit() expected error for failed commit, got nil")
	}

	// After failed commit, Rollback should return error too
	if err := tx.Rollback(context.Background()); err == nil {
		t.Error("Rollback() expected error after failed commit, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxWithPolicyWarnAllowsDrop(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DangerousOpPolicy = guard.PolicyWarn
	s, mock := newMockSession(t, cfg)

	mock.ExpectBegin()
	mock.ExpectExec("DROP TABLE users").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = tx.Exec(context.Background(), "DROP TABLE users")
	if err != nil {
		t.Fatalf("tx.Exec() with PolicyWarn error = %v", err)
	}

	if err := tx.Commit(context.Background()); err != nil {
		t.Errorf("Commit() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestTxQueryWithExistingLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 10").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Alice"))
	mock.ExpectRollback()

	tx, err := s.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	res, err := tx.Query(context.Background(), "SELECT * FROM users LIMIT 10")
	if err != nil {
		t.Fatalf("tx.Query() error = %v", err)
	}
	if res.Warning != "" {
		t.Errorf("Warning = %q, want empty for existing LIMIT", res.Warning)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
