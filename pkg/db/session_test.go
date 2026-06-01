package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/guard"
)

// newMockSession creates a Session backed by a sqlmock database for testing.
func newMockSession(t *testing.T, cfg *config.Config) (*Session, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlmock")
	s := newSessionWithDB("test", "mock://", cfg, sdb)
	return s, mock
}

func TestOpenSuccess(t *testing.T) {
	sess, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/testdb")
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	if sess == nil {
		t.Fatal("Open() returned nil session")
	}
	defer sess.Close()
}

func TestOpenInvalidDSN(t *testing.T) {
	_, err := Open("mysql", "invalid-dsn-with-no-at-sign")
	if err == nil {
		t.Fatal("Open() expected error for invalid DSN, got nil")
	}
}

func TestOpenWithOptions(t *testing.T) {
	sess, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/testdb",
		config.WithMaxOpenConns(10),
		config.WithMaxIdleConns(3),
		config.WithConnMaxLifetime(30*time.Second),
		config.WithQueryTimeout(5*time.Second),
		config.WithDangerousOpPolicy(guard.PolicyWarn),
		config.WithName("test-session"),
	)
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	defer sess.Close()

	if sess.Config().MaxOpenConns != 10 {
		t.Errorf("MaxOpenConns = %d, want 10", sess.Config().MaxOpenConns)
	}
	if sess.Config().MaxIdleConns != 3 {
		t.Errorf("MaxIdleConns = %d, want 3", sess.Config().MaxIdleConns)
	}
	if sess.Config().ConnMaxLifetime != 30*time.Second {
		t.Errorf("ConnMaxLifetime = %v, want 30s", sess.Config().ConnMaxLifetime)
	}
	if sess.Config().QueryTimeout != 5*time.Second {
		t.Errorf("QueryTimeout = %v, want 5s", sess.Config().QueryTimeout)
	}
	if sess.Config().DangerousOpPolicy != guard.PolicyWarn {
		t.Errorf("DangerousOpPolicy = %v, want Warn", sess.Config().DangerousOpPolicy)
	}
	if sess.Name() != "test-session" {
		t.Errorf("Name() = %q, want %q", sess.Name(), "test-session")
	}
}

func TestSessionNameDefaultToDSN(t *testing.T) {
	sess, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/mydb")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer sess.Close()

	if sess.Name() != sess.DSN() {
		t.Errorf("Name() = %q, want DSN %q", sess.Name(), sess.DSN())
	}
}

func TestSessionDSN(t *testing.T) {
	dsn := "user:pass@tcp(127.0.0.1:3306)/testdb"
	sess, err := Open("mysql", dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer sess.Close()

	if sess.DSN() != dsn {
		t.Errorf("DSN() = %q, want %q", sess.DSN(), dsn)
	}
}

func TestSessionConfig(t *testing.T) {
	sess, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/testdb")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer sess.Close()

	cfg := sess.Config()
	if cfg == nil {
		t.Fatal("Config() returned nil")
	}
	if cfg.DefaultLimit != config.DefaultDefaultLimit {
		t.Errorf("DefaultLimit = %d, want %d", cfg.DefaultLimit, config.DefaultDefaultLimit)
	}
	if cfg.MaxLimit != config.DefaultMaxLimit {
		t.Errorf("MaxLimit = %d, want %d", cfg.MaxLimit, config.DefaultMaxLimit)
	}
}

func TestClose(t *testing.T) {
	sess, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/testdb")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := sess.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// Close should be idempotent
	if err := sess.Close(); err != nil {
		t.Errorf("Close() (second call) error = %v, want nil", err)
	}
}

func TestPingSuccess(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())
	defer s.Close()

	mock.ExpectPing()

	ctx := context.Background()
	if err := s.Ping(ctx); err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestPingFailure(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())
	defer s.Close()

	mock.ExpectPing().WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := s.Ping(ctx)
	if err == nil {
		t.Fatal("Ping() expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestStats(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxOpenConns = 10
	s, mock := newMockSession(t, cfg)

	stats := s.Stats()

	// InUse should be 0 — no queries in flight.
	if stats.InUse != 0 {
		t.Errorf("InUse = %d, want 0", stats.InUse)
	}

	// Close and verify
	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCloseReleasesResources(t *testing.T) {
	s, mock := newMockSession(t, config.DefaultConfig())

	mock.ExpectClose()

	// Close the session
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// After close, Ping should fail because the underlying DB is closed
	ctx := context.Background()
	err := s.Ping(ctx)
	if err == nil {
		t.Error("Ping() expected error after close, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestOpenWithAllOptions(t *testing.T) {
	_, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/testdb",
		config.WithMaxOpenConns(50),
		config.WithMaxIdleConns(10),
		config.WithConnMaxLifetime(10*time.Minute),
		config.WithMaxRows(5000),
		config.WithDefaultLimit(200),
		config.WithMaxLimit(500),
		config.WithQueryTimeout(60*time.Second),
		config.WithStreamBatchSize(100),
		config.WithDangerousOpPolicy(guard.PolicyAllow),
		config.WithLogSanitizeParams(true),
		config.WithMaxConcurrentQueries(10),
	)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
}

func TestMultipleSessionsIndependent(t *testing.T) {
	sess1, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/db1", config.WithName("sess1"))
	if err != nil {
		t.Fatalf("Open(sess1) error = %v", err)
	}
	defer sess1.Close()

	sess2, err := Open("mysql", "user:pass@tcp(127.0.0.1:3306)/db2", config.WithName("sess2"))
	if err != nil {
		t.Fatalf("Open(sess2) error = %v", err)
	}
	defer sess2.Close()

	if sess1.Name() != "sess1" {
		t.Errorf("sess1.Name() = %q, want %q", sess1.Name(), "sess1")
	}
	if sess2.Name() != "sess2" {
		t.Errorf("sess2.Name() = %q, want %q", sess2.Name(), "sess2")
	}
	if sess1.DSN() == sess2.DSN() {
		t.Errorf("expected different DSNs, got both %q", sess1.DSN())
	}
	if sess1.Config() == sess2.Config() {
		t.Errorf("sessions should have distinct Config pointers")
	}
}

func TestAcquireConcurrencySlotUnderLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentQueries = 5
	s, mock := newMockSession(t, cfg)

	if err := s.acquireConcurrencySlot(context.Background()); err != nil {
		t.Errorf("acquireConcurrencySlot() error = %v, want nil", err)
	}

	s.releaseConcurrencySlot()

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestAcquireConcurrencySlotExhausted(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentQueries = 2
	s, mock := newMockSession(t, cfg)

	if err := s.acquireConcurrencySlot(context.Background()); err != nil {
		t.Fatalf("first acquireConcurrencySlot() error = %v", err)
	}
	if err := s.acquireConcurrencySlot(context.Background()); err != nil {
		t.Fatalf("second acquireConcurrencySlot() error = %v", err)
	}

	// Third should fail immediately (non-blocking)
	if err := s.acquireConcurrencySlot(context.Background()); err == nil {
		t.Error("third acquireConcurrencySlot() expected error, got nil")
	}

	// Release one slot
	s.releaseConcurrencySlot()

	if err := s.acquireConcurrencySlot(context.Background()); err != nil {
		t.Errorf("acquireConcurrencySlot() after release error = %v", err)
	}

	s.releaseConcurrencySlot()
	s.releaseConcurrencySlot()

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestConcurrencySlotReleasedOnQuery(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentQueries = 2
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT 1 LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"?"}).AddRow(int64(1)))

	_, err := s.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	// After query, slot should be released
	if err := s.acquireConcurrencySlot(context.Background()); err != nil {
		t.Errorf("acquireConcurrencySlot() after Query error = %v", err)
	}
	s.releaseConcurrencySlot()

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestConcurrencySlotReleasedOnStreamCompletion(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentQueries = 1
	s, mock := newMockSession(t, cfg)

	mock.ExpectQuery("SELECT 1 LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"?"}).AddRow(int64(1)))

	sr, err := s.QueryStream(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	for sr.Next() {
	}
	if err := sr.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}

	// After stream completes, slot should be released
	if err := s.acquireConcurrencySlot(context.Background()); err != nil {
		t.Errorf("acquireConcurrencySlot() after stream error = %v", err)
	}
	s.releaseConcurrencySlot()

	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
