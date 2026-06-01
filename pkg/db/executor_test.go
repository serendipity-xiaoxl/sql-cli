package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/guard"
)

// runWithClose wraps a test function with proper Close and expectation checking.
func runWithClose(t *testing.T, mock sqlmock.Sqlmock, s *Session, fn func()) {
	t.Helper()
	fn()
	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestExecInsert(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("INSERT INTO users").
		WillReturnResult(sqlmock.NewResult(42, 1))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Alice")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.LastInsertID != 42 {
			t.Errorf("LastInsertID = %d, want 42", res.LastInsertID)
		}
		if res.RowsAffected != 1 {
			t.Errorf("RowsAffected = %d, want 1", res.RowsAffected)
		}
	})
}

func TestExecUpdateWithWhere(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("UPDATE users SET name").WillReturnResult(sqlmock.NewResult(0, 3))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "UPDATE users SET name = ? WHERE id = ?", "Bob", 1)
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.RowsAffected != 3 {
			t.Errorf("RowsAffected = %d, want 3", res.RowsAffected)
		}
	})
}

func TestExecDeleteWithWhere(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("DELETE FROM users WHERE").WillReturnResult(sqlmock.NewResult(0, 1))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "DELETE FROM users WHERE id = ?", 1)
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.RowsAffected != 1 {
			t.Errorf("RowsAffected = %d, want 1", res.RowsAffected)
		}
	})
}

func TestExecDeleteWithoutWhereBlocked(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	runWithClose(t, mock, s, func() {
		_, err := s.Exec(context.Background(), "DELETE FROM users")
		if err == nil {
			t.Fatal("Exec() expected error for DELETE without WHERE, got nil")
		}
	})
}

func TestExecUpdateWithoutWhereBlocked(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	runWithClose(t, mock, s, func() {
		_, err := s.Exec(context.Background(), "UPDATE users SET name = 'test'")
		if err == nil {
			t.Fatal("Exec() expected error for UPDATE without WHERE, got nil")
		}
	})
}

func TestExecUpdateWithoutWhereAllowed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RejectNoWhere = false
	s, mock := newMockSession(t, cfg)

	mock.ExpectExec("UPDATE users SET name").WillReturnResult(sqlmock.NewResult(0, 5))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "UPDATE users SET name = 'test' WHERE 1=1")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.RowsAffected != 5 {
			t.Errorf("RowsAffected = %d, want 5", res.RowsAffected)
		}
	})
}

func TestExecCreateTable(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("CREATE TABLE users").WillReturnResult(sqlmock.NewResult(0, 0))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "CREATE TABLE users (id INT PRIMARY KEY, name TEXT)")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.RowsAffected != 0 {
			t.Errorf("RowsAffected = %d, want 0", res.RowsAffected)
		}
	})
}

func TestExecAlterTable(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("ALTER TABLE users").WillReturnResult(sqlmock.NewResult(0, 0))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "ALTER TABLE users ADD COLUMN email TEXT")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.RowsAffected != 0 {
			t.Errorf("RowsAffected = %d, want 0", res.RowsAffected)
		}
	})
}

func TestExecDropTableBlocked(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	runWithClose(t, mock, s, func() {
		_, err := s.Exec(context.Background(), "DROP TABLE users")
		if err == nil {
			t.Fatal("Exec() expected error for DROP, got nil")
		}
	})
}

func TestExecTruncateBlocked(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	runWithClose(t, mock, s, func() {
		_, err := s.Exec(context.Background(), "TRUNCATE TABLE users")
		if err == nil {
			t.Fatal("Exec() expected error for TRUNCATE, got nil")
		}
	})
}

func TestExecExecutionError(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("INSERT INTO users").WillReturnError(sql.ErrConnDone)

	runWithClose(t, mock, s, func() {
		_, err := s.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Test")
		if err == nil {
			t.Fatal("Exec() expected error for execution failure, got nil")
		}
	})
}

func TestExecContextCancelled(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runWithClose(t, mock, s, func() {
		_, err := s.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Test")
		if err == nil {
			t.Fatal("Exec() expected error for cancelled context, got nil")
		}
	})
}

func TestExecInsertReturnsZeroValues(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("INSERT INTO logs").WillReturnResult(sqlmock.NewResult(0, 0))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "INSERT INTO logs (msg) VALUES (?)", "test")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.RowsAffected != 0 {
			t.Errorf("RowsAffected = %d, want 0", res.RowsAffected)
		}
		if res.LastInsertID != 0 {
			t.Errorf("LastInsertID = %d, want 0", res.LastInsertID)
		}
	})
}

func TestExecDurationRecorded(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("INSERT INTO users").
		WillReturnResult(sqlmock.NewResult(1, 1))

	runWithClose(t, mock, s, func() {
		res, err := s.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "test")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if res.DurationMs < 0 {
			t.Errorf("DurationMs = %d, want >= 0", res.DurationMs)
		}
	})
}

func TestExecDropWithPolicyWarn(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DangerousOpPolicy = guard.PolicyWarn
	s, mock := newMockSession(t, cfg)

	mock.ExpectExec("DROP TABLE users").WillReturnResult(sqlmock.NewResult(0, 0))

	res, err := s.Exec(context.Background(), "DROP TABLE users")
	if err != nil {
		t.Fatalf("Exec() with PolicyWarn error = %v", err)
	}
	if res.RowsAffected != 0 {
		t.Errorf("RowsAffected = %d, want 0", res.RowsAffected)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestExecDropWithDefaultPromptPolicy(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	runWithClose(t, mock, s, func() {
		_, err := s.Exec(context.Background(), "DROP TABLE users")
		if err == nil {
			t.Fatal("Exec() expected error for DROP with default PolicyPrompt, got nil")
		}
	})
}

func TestExecContextDeadlineExceeded(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectExec("INSERT INTO users").WillReturnError(context.DeadlineExceeded)

	_, err := s.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "Timeout")
	if err == nil {
		t.Fatal("Exec() expected error for deadline exceeded, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
