package db

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xiaoxl/sql-cli/pkg/config"
)

func TestStreamSelectWithoutLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice"))

	sr, err := s.QueryStream(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	var rows []map[string]interface{}
	for sr.Next() {
		rows = append(rows, sr.Scan())
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0]["id"] != int64(1) || rows[0]["name"] != "Alice" {
		t.Errorf("row = %v, want {id:1 name:Alice}", rows[0])
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamSelectWithExistingLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 5").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	sr, err := s.QueryStream(context.Background(), "SELECT * FROM users LIMIT 5")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	var count int
	for sr.Next() {
		count++
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if count != 2 {
		t.Errorf("got %d rows, want 2", count)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamRejectsNonSelect(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	_, err := s.QueryStream(context.Background(), "INSERT INTO users (name) VALUES ('test')")
	if err == nil {
		t.Fatal("QueryStream() expected error for INSERT, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamEmptyResult(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users WHERE 1=0 LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	sr, err := s.QueryStream(context.Background(), "SELECT * FROM users WHERE 1=0")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	var count int
	for sr.Next() {
		count++
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if count != 0 {
		t.Errorf("got %d rows, want 0", count)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamMultipleRows(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT id, name FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice").
			AddRow(2, "Bob").
			AddRow(3, "Charlie"))

	sr, err := s.QueryStream(context.Background(), "SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	var rows []map[string]interface{}
	for sr.Next() {
		rows = append(rows, sr.Scan())
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0]["id"] != int64(1) || rows[2]["name"] != "Charlie" {
		t.Errorf("unexpected row data: %v", rows)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamColumnsAfterFirstRow(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT id, name FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice"))

	sr, err := s.QueryStream(context.Background(), "SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	// Columns should become available after first Next()
	if !sr.Next() {
		t.Fatal("Next() expected true, got false")
	}
	cols := sr.Columns()
	if len(cols) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(cols))
	}
	if cols[0] != "id" || cols[1] != "name" {
		t.Errorf("Columns = %v, want [id name]", cols)
	}

	// Drain remaining
	for sr.Next() {
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamParameterizedQuery(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users WHERE id = \\? LIMIT 100").
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(42, "Answer"))

	sr, err := s.QueryStream(context.Background(), "SELECT * FROM users WHERE id = ?", 42)
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	var count int
	for sr.Next() {
		count++
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if count != 1 {
		t.Errorf("got %d rows, want 1", count)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamContextCancelled(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	sr, err := s.QueryStream(ctx, "SELECT * FROM users")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	// The goroutine should detect cancelled context and set error, close channel
	// Without any mock expectation being consumed
	var count int
	for sr.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("got %d rows, want 0", count)
	}
	if sr.Err() == nil {
		t.Error("Err() expected non-nil for cancelled context, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamExecutionError(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	// The mock expectation for the query should not be set — the streaming
	// goroutine will try to execute but the mock will fail with no expectation.
	// Instead, set an expectation that returns an error.
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnError(simulatedErr("connection refused"))

	sr, err := s.QueryStream(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	var count int
	for sr.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("got %d rows, want 0", count)
	}
	if sr.Err() == nil {
		t.Error("Err() expected non-nil for execution error, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamEarlyClose(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(1).AddRow(2).AddRow(3).AddRow(4).AddRow(5))

	sr, err := s.QueryStream(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	// Read only 2 rows, then close early
	count := 0
	for sr.Next() {
		count++
		if count >= 2 {
			sr.Close()
		}
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if count != 2 {
		t.Errorf("got %d rows, want 2", count)
	}
	// Wait for the producer goroutine to fully finish before closing
	// the session, preventing a data race on the underlying connection.
	sr.Wait()

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStreamWithLimitCap(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxLimit = 50
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 50").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	sr, err := s.QueryStream(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	var count int
	for sr.Next() {
		count++
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if count != 1 {
		t.Errorf("got %d rows, want 1", count)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// simulatedErr returns a simple error for test use.
type simulatedErr string

func (e simulatedErr) Error() string { return string(e) }
