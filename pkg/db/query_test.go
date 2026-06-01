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

func TestQueryWithOffset(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 10 OFFSET 20").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice").
			AddRow(2, "Bob"))

	res, err := s.QueryWithOffset(context.Background(), "SELECT * FROM users", 10, 20)
	if err != nil {
		t.Fatalf("QueryWithOffset() error = %v", err)
	}
	if res.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", res.RowCount)
	}
	if res.Warning != "LIMIT 10 applied automatically" {
		t.Errorf("Warning = %q, want %q", res.Warning, "LIMIT 10 applied automatically")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOffsetRejectsNonSelect(t *testing.T) {
	s, _ := newMockSession(t, config.DefaultConfig())

	_, err := s.QueryWithOffset(context.Background(), "DELETE FROM users", 10, 0)
	if err == nil {
		t.Error("QueryWithOffset expected error for non-SELECT, got nil")
	}

	s.Close()
}

func TestQueryWithOffsetWithExistingLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 5 OFFSET 10").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOffset(context.Background(), "SELECT * FROM users LIMIT 5", 100, 10)
	if err != nil {
		t.Fatalf("QueryWithOffset() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning != "" {
		t.Errorf("Warning = %q, want empty (existing LIMIT)", res.Warning)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOffsetWithExistingOffset(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	// SQL already has OFFSET — should not append another
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100 OFFSET 5").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOffset(context.Background(), "SELECT * FROM users LIMIT 100 OFFSET 5", 50, 10)
	if err != nil {
		t.Fatalf("QueryWithOffset() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning != "" {
		t.Errorf("Warning = %q, want empty (existing LIMIT+OFFSET)", res.Warning)
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOffsetZeroOffset(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	// offset=0 should NOT append OFFSET
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOffset(context.Background(), "SELECT * FROM users", 100, 0)
	if err != nil {
		t.Fatalf("QueryWithOffset() error = %v", err)
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

func TestQueryWithOffsetLimitCapping(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxLimit = 50
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 50 OFFSET 100").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOffset(context.Background(), "SELECT * FROM users", 200, 100)
	if err != nil {
		t.Fatalf("QueryWithOffset() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning != "LIMIT capped to 50 (max allowed)" {
		t.Errorf("Warning = %q, want %q", res.Warning, "LIMIT capped to 50 (max allowed)")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOffsetHasMore(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 5 OFFSET 10").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(1).AddRow(2).AddRow(3).AddRow(4).AddRow(5))

	res, err := s.QueryWithOffset(context.Background(), "SELECT * FROM users", 5, 10)
	if err != nil {
		t.Fatalf("QueryWithOffset() error = %v", err)
	}
	if res.RowCount != 5 {
		t.Errorf("RowCount = %d, want 5", res.RowCount)
	}
	if !res.HasMore {
		t.Error("HasMore = false, want true (rowCount >= appliedLimit)")
	}

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOptionsDefaultLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	// Zero-value options should apply default limit (100)
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOptions(context.Background(), "SELECT * FROM users", QueryOptions{})
	if err != nil {
		t.Fatalf("QueryWithOptions() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	if res.Warning != "LIMIT 100 applied automatically" {
		t.Errorf("Warning = %q, want LIMIT 100 warning", res.Warning)
	}

	mock.ExpectClose()
	s.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOptionsWithOffset(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 10 OFFSET 20").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOptions(context.Background(), "SELECT * FROM users", QueryOptions{Limit: 10, Offset: 20})
	if err != nil {
		t.Fatalf("QueryWithOptions() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}

	mock.ExpectClose()
	s.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOptionsLimitCapping(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxLimit = 50
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 50 OFFSET 100").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOptions(context.Background(), "SELECT * FROM users", QueryOptions{Limit: 200, Offset: 100})
	if err != nil {
		t.Fatalf("QueryWithOptions() error = %v", err)
	}
	if res.Warning != "LIMIT capped to 50 (max allowed)" {
		t.Errorf("Warning = %q, want LIMIT capped warning", res.Warning)
	}

	mock.ExpectClose()
	s.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOptionsExistingLimit(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	// SQL has existing LIMIT — should pass through
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 5").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOptions(context.Background(), "SELECT * FROM users LIMIT 5", QueryOptions{Limit: 100})
	if err != nil {
		t.Fatalf("QueryWithOptions() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
	// Warning should be empty since SQL has existing LIMIT
	if res.Warning != "" {
		t.Errorf("Warning = %q, want empty for existing LIMIT", res.Warning)
	}

	mock.ExpectClose()
	s.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOptionsRejectsNonSelect(t *testing.T) {
	s, _ := newMockSession(t, config.DefaultConfig())

	_, err := s.QueryWithOptions(context.Background(), "DELETE FROM users", QueryOptions{})
	if err == nil {
		t.Error("QueryWithOptions expected error for non-SELECT, got nil")
	}

	s.Close()
}

func TestQueryWithOptionsZeroOffset(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	// offset=0 should not append OFFSET at all
	mock.ExpectQuery("SELECT \\* FROM users LIMIT 50").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	res, err := s.QueryWithOptions(context.Background(), "SELECT * FROM users", QueryOptions{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("QueryWithOptions() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}

	mock.ExpectClose()
	s.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestQueryWithOptionsHasMore(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectQuery("SELECT \\* FROM users LIMIT 5 OFFSET 10").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(1).AddRow(2).AddRow(3).AddRow(4).AddRow(5))

	res, err := s.QueryWithOptions(context.Background(), "SELECT * FROM users", QueryOptions{Limit: 5, Offset: 10})
	if err != nil {
		t.Fatalf("QueryWithOptions() error = %v", err)
	}
	if !res.HasMore {
		t.Error("HasMore = false, want true (rowCount >= limit)")
	}

	mock.ExpectClose()
	s.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

