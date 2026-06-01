package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
)

// newMockSession creates a *db.Session backed by sqlmock.
func newMockSession(t *testing.T, name string, cfg *config.Config) (*db.Session, sqlmock.Sqlmock) {
	t.Helper()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	sdb := sqlx.NewDb(mockDB, "sqlmock")
	s := db.NewTestSession(name, "mock://", cfg, sdb)
	return s, mock
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0", r.Count())
	}
	if names := r.List(); len(names) != 0 {
		t.Errorf("List() = %v, want empty", names)
	}
}

func TestRegistryAddGet(t *testing.T) {
	r := NewRegistry()
	s, mock := newMockSession(t, "test-session", nil)
	mock.ExpectClose()

	if err := r.Add("sessions", s); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if r.Count() != 1 {
		t.Errorf("Count() = %d, want 1", r.Count())
	}

	got, err := r.Get("sessions")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name() != s.Name() {
		t.Errorf("Get().Name() = %q, want %q", got.Name(), s.Name())
	}
}

func TestRegistryAddDuplicate(t *testing.T) {
	r := NewRegistry()
	s1, mock1 := newMockSession(t, "s1", nil)
	mock1.ExpectClose()
	s2, mock2 := newMockSession(t, "s2", nil)
	mock2.ExpectClose()

	if err := r.Add("same-name", s1); err != nil {
		t.Fatalf("first Add() error = %v", err)
	}
	if err := r.Add("same-name", s2); err == nil {
		t.Error("second Add() expected error, got nil")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("nonexistent"); err == nil {
		t.Error("Get() expected error for nonexistent session, got nil")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	s1, mock1 := newMockSession(t, "alpha", nil)
	mock1.ExpectClose()
	s2, mock2 := newMockSession(t, "beta", nil)
	mock2.ExpectClose()
	s3, mock3 := newMockSession(t, "gamma", nil)
	mock3.ExpectClose()

	r.Add("alpha", s1)
	r.Add("beta", s2)
	r.Add("gamma", s3)

	names := r.List()
	if len(names) != 3 {
		t.Errorf("List() returned %d names, want 3", len(names))
	}

	seen := make(map[string]bool)
	for _, n := range names {
		seen[n] = true
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !seen[want] {
			t.Errorf("List() missing %q", want)
		}
	}
}

func TestRegistryClose(t *testing.T) {
	r := NewRegistry()
	s, mock := newMockSession(t, "to-close", nil)
	mock.ExpectClose()

	r.Add("to-close", s)

	if err := r.Close("to-close"); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after close", r.Count())
	}
	if _, err := r.Get("to-close"); err == nil {
		t.Error("Get() expected error after close, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestRegistryCloseNotFound(t *testing.T) {
	r := NewRegistry()
	if err := r.Close("nonexistent"); err == nil {
		t.Error("Close() expected error for nonexistent session, got nil")
	}
}

func TestRegistryCloseAll(t *testing.T) {
	r := NewRegistry()
	s1, mock1 := newMockSession(t, "a", nil)
	mock1.ExpectClose()
	s2, mock2 := newMockSession(t, "b", nil)
	mock2.ExpectClose()
	s3, mock3 := newMockSession(t, "c", nil)
	mock3.ExpectClose()

	r.Add("a", s1)
	r.Add("b", s2)
	r.Add("c", s3)

	if err := r.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after CloseAll", r.Count())
	}
	for _, m := range []sqlmock.Sqlmock{mock1, mock2, mock3} {
		if err := m.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet mock expectations: %v", err)
		}
	}
}

func TestRegistryCloseAllEmpty(t *testing.T) {
	r := NewRegistry()
	if err := r.CloseAll(); err != nil {
		t.Errorf("CloseAll() on empty registry error = %v, want nil", err)
	}
}

func TestRegistryOpenDuplicateName(t *testing.T) {
	r := NewRegistry()
	s, mock := newMockSession(t, "dup", nil)
	mock.ExpectClose()

	r.Add("dup", s)

	// Try to Open with same name — should fail
	_, err := r.Open("dup", "mysql", "mock://")
	if err == nil {
		t.Error("Open() with duplicate name expected error, got nil")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	const n = 20

	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("sess-%d", i)
			s, mock := newMockSession(t, name, nil)
			mock.ExpectClose()

			if err := r.Add(name, s); err != nil {
				t.Errorf("concurrent Add(%q) error = %v", name, err)
			}
		}()
	}

	wg.Wait()

	if r.Count() != n {
		t.Errorf("Count() = %d, want %d", r.Count(), n)
	}

	// List should return all names
	names := r.List()
	if len(names) != n {
		t.Errorf("List() returned %d names, want %d", len(names), n)
	}

	// CloseAll should close all without error
	if err := r.CloseAll(); err != nil {
		t.Errorf("CloseAll() error = %v", err)
	}
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after CloseAll", r.Count())
	}
}

func TestRegistryUseSessionAfterGet(t *testing.T) {
	r := NewRegistry()
	s, mock := newMockSession(t, "usable", nil)

	mock.ExpectQuery("SELECT 1 LIMIT 100").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow("1"))

	r.Add("usable", s)

	got, err := r.Get("usable")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// The session should be usable for queries
	res, err := got.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}

	mock.ExpectClose()
	r.Close("usable")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
