package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
)

// newMockSession creates a *db.Session backed by sqlmock.
func newMockSession(t *testing.T) (*db.Session, sqlmock.Sqlmock) {
	t.Helper()
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	sdb := sqlx.NewDb(mockDB, "sqlmock")
	cfg := config.DefaultConfig()
	s := db.NewTestSession("test", "mock://", cfg, sdb)
	return s, mock
}

func TestBatchExecDirect_Success(t *testing.T) {
	s, mock := newMockSession(t)

	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(1, 1))

	stmts := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
	}
	results := batchExec(context.Background(), s, stmts, false, false)

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Statement != stmts[0] {
		t.Errorf("results[0].Statement = %q, want %q", results[0].Statement, stmts[0])
	}
	if results[0].Error != "" {
		t.Errorf("results[0].Error = %q, want empty", results[0].Error)
	}
	if results[1].Statement != stmts[1] {
		t.Errorf("results[1].Statement = %q, want %q", results[1].Statement, stmts[1])
	}
	if results[1].Error != "" {
		t.Errorf("results[1].Error = %q, want empty", results[1].Error)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestBatchExecDirect_ErrorStops(t *testing.T) {
	s, mock := newMockSession(t)

	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("duplicate entry"))

	stmts := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
		"SELECT 1",
	}
	results := batchExec(context.Background(), s, stmts, false, false)

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2 (success + error, stopped)", len(results))
	}
	if results[0].Error != "" {
		t.Errorf("results[0].Error = %q, want empty", results[0].Error)
	}
	if results[1].Error == "" {
		t.Error("results[1].Error = empty, want error message")
	}
	if results[1].Statement != stmts[1] {
		t.Errorf("results[1].Statement = %q, want %q", results[1].Statement, stmts[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestBatchExecDirect_ContinueOnError(t *testing.T) {
	s, mock := newMockSession(t)

	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("duplicate entry"))
	mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))

	stmts := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
		"SELECT 1",
	}
	results := batchExec(context.Background(), s, stmts, false, true)

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].Error != "" {
		t.Errorf("results[0].Error = %q, want empty", results[0].Error)
	}
	if results[1].Error == "" {
		t.Error("results[1].Error = empty, want error message")
	}
	if results[2].Error != "" {
		t.Errorf("results[2].Error = %q, want empty", results[2].Error)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestBatchExecInTx_Success(t *testing.T) {
	s, mock := newMockSession(t)

	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	stmts := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
	}
	results := batchExec(context.Background(), s, stmts, true, false)

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Error != "" {
		t.Errorf("results[0].Error = %q, want empty", results[0].Error)
	}
	if results[1].Error != "" {
		t.Errorf("results[1].Error = %q, want empty", results[1].Error)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestBatchExecInTx_ExecFailRollback(t *testing.T) {
	s, mock := newMockSession(t)

	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("duplicate entry"))
	mock.ExpectRollback()

	stmts := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
		"SELECT 1",
	}
	results := batchExec(context.Background(), s, stmts, true, false)

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2 (success + error, stopped)", len(results))
	}
	if results[0].Error != "" {
		t.Errorf("results[0].Error = %q, want empty", results[0].Error)
	}
	if results[1].Error == "" {
		t.Error("results[1].Error = empty, want error message")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestBatchExecInTx_ContinueOnError(t *testing.T) {
	s, mock := newMockSession(t)

	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("duplicate entry"))
	mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	stmts := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
		"SELECT 1",
	}
	results := batchExec(context.Background(), s, stmts, true, true)

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].Error != "" {
		t.Errorf("results[0].Error = %q, want empty", results[0].Error)
	}
	if results[1].Error == "" {
		t.Error("results[1].Error = empty, want error message")
	}
	if results[2].Error != "" {
		t.Errorf("results[2].Error = %q, want empty", results[2].Error)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestBatchExecEmpty(t *testing.T) {
	s, _ := newMockSession(t)

	results := batchExec(context.Background(), s, []string{}, false, false)
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestStmtResult_JSONFormat(t *testing.T) {
	results := []*StmtResult{
		{Statement: "CREATE TABLE t (id INT)", LastInsertID: 0, RowsAffected: 0, DurationMs: 5},
		{Statement: "INSERT INTO t VALUES (1)", LastInsertID: 1, RowsAffected: 1, DurationMs: 3},
		{Statement: "INSERT INTO t VALUES (2)", Error: "duplicate key"},
	}

	data, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	jsonStr := string(data)
	for _, field := range []string{"statement", "last_insert_id", "rows_affected", "duration_ms", "error"} {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON missing field %q: %s", field, jsonStr)
		}
	}
}

func TestBatchExecFromFile(t *testing.T) {
	content := "CREATE TABLE t (id INT);\nINSERT INTO t VALUES (1);\n"
	f, err := os.CreateTemp("", "batch-test-*.sql")
	if err != nil {
		t.Fatalf("CreateTemp error = %v", err)
	}
	tmpPath := f.Name()
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(tmpPath)
		t.Fatalf("WriteString error = %v", err)
	}
	f.Close()
	defer os.Remove(tmpPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	statements := sqlnorm.SplitStatements(string(data))

	want := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
	}
	if len(statements) != len(want) {
		t.Fatalf("SplitStatements returned %d statements, want %d", len(statements), len(want))
	}
	for i, stmt := range statements {
		if stmt != want[i] {
			t.Errorf("statement[%d] = %q, want %q", i, stmt, want[i])
		}
	}
}

func TestBatchExecCommentsOnlyFile(t *testing.T) {
	content := "-- This is a comment\n-- Another comment\n"
	f, err := os.CreateTemp("", "batch-comments-*.sql")
	if err != nil {
		t.Fatalf("CreateTemp error = %v", err)
	}
	tmpPath := f.Name()
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(tmpPath)
		t.Fatalf("WriteString error = %v", err)
	}
	f.Close()
	defer os.Remove(tmpPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	statements := sqlnorm.SplitStatements(string(data))

	if len(statements) != 0 {
		t.Errorf("SplitStatements returned %d for comment-only file, want 0", len(statements))
	}
}
