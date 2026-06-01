package db

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xiaoxl/sql-cli/pkg/config"
)

func TestQuerySelectWithoutLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice").
			AddRow(2, "Bob"))

	res, err := s.Query(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", res.RowCount)
	}
	if res.Warning == "" {
		t.Error("Warning expected for auto-appended LIMIT, got empty")
	}
	if res.DurationMs < 0 {
		t.Errorf("DurationMs = %d, want >= 0", res.DurationMs)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQuerySelectWithLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 5").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.Query(context.Background(), "SELECT * FROM users LIMIT 5")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning != "" {
		t.Errorf("Warning = %q, want empty (no auto-append)", res.Warning)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryRejectsNonSelect(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	_, err := s.Query(context.Background(), "INSERT INTO users (name) VALUES ('test')")
	if err == nil {
		t.Fatal("Query() expected error for INSERT, got nil")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithLimitExplicit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 10").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice"))

	res, err := s.QueryWithLimit(context.Background(), "SELECT * FROM users", 10)
	if err != nil {
		t.Fatalf("QueryWithLimit() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning == "" {
		t.Error("Warning expected for auto-appended LIMIT, got empty")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithLimitCapsAtMax(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxLimit = 500
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithLimit(context.Background(), "SELECT * FROM users", 9999)
	if err != nil {
		t.Fatalf("QueryWithLimit() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithLimitCapInWarning(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxLimit = 50
	cfg.DefaultLimit = 20
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 50").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithLimit(context.Background(), "SELECT * FROM users", 9999)
	if err != nil {
		t.Fatalf("QueryWithLimit() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning == "" {
		t.Errorf("Warning expected for capped LIMIT, got empty")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryReturnedRowsAndColumns(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT id, name FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice").
			AddRow(2, "Bob").
			AddRow(3, "Charlie"))

	res, err := s.Query(context.Background(), "SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(res.Columns) != 2 {
		t.Errorf("len(Columns) = %d, want 2", len(res.Columns))
	}
	if res.Columns[0] != "id" || res.Columns[1] != "name" {
		t.Errorf("Columns = %v, want [id name]", res.Columns)
	}
	if res.RowCount != 3 {
		t.Errorf("RowCount = %d, want 3", res.RowCount)
	}
	if len(res.Rows) != 3 {
		t.Errorf("len(Rows) = %d, want 3", len(res.Rows))
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryEmptyResult(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users WHERE id = -1 LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	res, err := s.Query(context.Background(), "SELECT * FROM users WHERE id = -1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0", res.RowCount)
	}
	if len(res.Rows) != 0 {
		t.Errorf("len(Rows) = %d, want 0", len(res.Rows))
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryParameterized(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users WHERE id = \\? LIMIT 100").
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(42, "Answer"))

	res, err := s.Query(context.Background(), "SELECT * FROM users WHERE id = ?", 42)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithClause(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("WITH cte AS \\(SELECT \\* FROM users\\) SELECT \\* FROM cte LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.Query(context.Background(), "WITH cte AS (SELECT * FROM users) SELECT * FROM cte")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryHasMoreTrue(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DefaultLimit = 2
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	res, err := s.Query(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if !res.HasMore {
		t.Error("HasMore expected true when rowCount == defaultLimit, got false")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryHasMoreFalse(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DefaultLimit = 100
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.Query(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.HasMore {
		t.Error("HasMore expected false when rowCount < appliedLimit, got true")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithLimitCapsAtMaxHasMore(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxLimit = 2
	cfg.DefaultLimit = 5
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	res, err := s.Query(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if !res.HasMore {
		t.Error("HasMore expected true when rowCount == capped limit, got false")
	}
	if res.Warning == "" {
		t.Error("Warning expected for capped LIMIT, got empty")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryExistingLimitNoHasMore(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	res, err := s.Query(context.Background(), "SELECT * FROM users LIMIT 2")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.HasMore {
		t.Error("HasMore expected false with existing LIMIT, got true")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithLimitIgnoreWarning(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 5").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithLimit(context.Background(), "SELECT * FROM users LIMIT 5", 100)
	if err != nil {
		t.Fatalf("QueryWithLimit() error = %v", err)
	}
	if res.Warning != "" {
		t.Errorf("Warning = %q, want empty (existing LIMIT ignores explicit limit)", res.Warning)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
