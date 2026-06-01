package config

import (
	"testing"
	"time"

	"github.com/xiaoxl/sql-cli/pkg/guard"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxOpenConns != DefaultMaxOpenConns {
		t.Errorf("MaxOpenConns = %d, want %d", cfg.MaxOpenConns, DefaultMaxOpenConns)
	}
	if cfg.MaxIdleConns != DefaultMaxIdleConns {
		t.Errorf("MaxIdleConns = %d, want %d", cfg.MaxIdleConns, DefaultMaxIdleConns)
	}
	if cfg.ConnMaxLifetime != DefaultConnMaxLifetime {
		t.Errorf("ConnMaxLifetime = %v, want %v", cfg.ConnMaxLifetime, DefaultConnMaxLifetime)
	}
	if cfg.MaxRows != DefaultMaxRows {
		t.Errorf("MaxRows = %d, want %d", cfg.MaxRows, DefaultMaxRows)
	}
	if cfg.DefaultLimit != DefaultDefaultLimit {
		t.Errorf("DefaultLimit = %d, want %d", cfg.DefaultLimit, DefaultDefaultLimit)
	}
	if cfg.MaxLimit != DefaultMaxLimit {
		t.Errorf("MaxLimit = %d, want %d", cfg.MaxLimit, DefaultMaxLimit)
	}
	if cfg.QueryTimeout != DefaultQueryTimeout {
		t.Errorf("QueryTimeout = %v, want %v", cfg.QueryTimeout, DefaultQueryTimeout)
	}
	if cfg.StreamBatchSize != DefaultStreamBatchSize {
		t.Errorf("StreamBatchSize = %d, want %d", cfg.StreamBatchSize, DefaultStreamBatchSize)
	}
	if cfg.DangerousOpPolicy != guard.PolicyPrompt {
		t.Errorf("DangerousOpPolicy = %v, want %v", cfg.DangerousOpPolicy, guard.PolicyPrompt)
	}
	if !cfg.RejectNoWhere {
		t.Errorf("RejectNoWhere = %v, want true", cfg.RejectNoWhere)
	}
	if cfg.LogSanitizeParams != false {
		t.Errorf("LogSanitizeParams = %v, want false", cfg.LogSanitizeParams)
	}
	if cfg.MaxConcurrentQueries != 0 {
		t.Errorf("MaxConcurrentQueries = %d, want 0", cfg.MaxConcurrentQueries)
	}
}

func TestWithMaxOpenConns(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxOpenConns(50)(cfg)
	if cfg.MaxOpenConns != 50 {
		t.Errorf("MaxOpenConns = %d, want 50", cfg.MaxOpenConns)
	}
}

func TestWithMaxIdleConns(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxIdleConns(10)(cfg)
	if cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", cfg.MaxIdleConns)
	}
}

func TestWithConnMaxLifetime(t *testing.T) {
	cfg := DefaultConfig()
	d := 10 * time.Minute
	WithConnMaxLifetime(d)(cfg)
	if cfg.ConnMaxLifetime != d {
		t.Errorf("ConnMaxLifetime = %v, want %v", cfg.ConnMaxLifetime, d)
	}
}

func TestWithRejectNoWhere(t *testing.T) {
	cfg := DefaultConfig()
	WithRejectNoWhere(false)(cfg)
	if cfg.RejectNoWhere != false {
		t.Errorf("RejectNoWhere = %v, want false", cfg.RejectNoWhere)
	}
}

func TestWithMaxRows(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxRows(500)(cfg)
	if cfg.MaxRows != 500 {
		t.Errorf("MaxRows = %d, want 500", cfg.MaxRows)
	}
}

func TestWithDefaultLimit(t *testing.T) {
	cfg := DefaultConfig()
	WithDefaultLimit(50)(cfg)
	if cfg.DefaultLimit != 50 {
		t.Errorf("DefaultLimit = %d, want 50", cfg.DefaultLimit)
	}
}

func TestWithMaxLimit(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxLimit(2000)(cfg)
	if cfg.MaxLimit != 2000 {
		t.Errorf("MaxLimit = %d, want 2000", cfg.MaxLimit)
	}
}

func TestWithQueryTimeout(t *testing.T) {
	cfg := DefaultConfig()
	d := 10 * time.Second
	WithQueryTimeout(d)(cfg)
	if cfg.QueryTimeout != d {
		t.Errorf("QueryTimeout = %v, want %v", cfg.QueryTimeout, d)
	}
}

func TestWithStreamBatchSize(t *testing.T) {
	cfg := DefaultConfig()
	WithStreamBatchSize(100)(cfg)
	if cfg.StreamBatchSize != 100 {
		t.Errorf("StreamBatchSize = %d, want 100", cfg.StreamBatchSize)
	}
}

func TestWithDangerousOpPolicy(t *testing.T) {
	cfg := DefaultConfig()
	WithDangerousOpPolicy(guard.PolicyAllow)(cfg)
	if cfg.DangerousOpPolicy != guard.PolicyAllow {
		t.Errorf("DangerousOpPolicy = %v, want %v", cfg.DangerousOpPolicy, guard.PolicyAllow)
	}
}

func TestWithLogSanitizeParams(t *testing.T) {
	cfg := DefaultConfig()
	WithLogSanitizeParams(true)(cfg)
	if cfg.LogSanitizeParams != true {
		t.Errorf("LogSanitizeParams = %v, want true", cfg.LogSanitizeParams)
	}
}

func TestWithMaxConcurrentQueries(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxConcurrentQueries(5)(cfg)
	if cfg.MaxConcurrentQueries != 5 {
		t.Errorf("MaxConcurrentQueries = %d, want 5", cfg.MaxConcurrentQueries)
	}
}

func TestMultipleOptions(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxOpenConns(10)(cfg)
	WithDefaultLimit(200)(cfg)
	WithDangerousOpPolicy(guard.PolicyWarn)(cfg)

	if cfg.MaxOpenConns != 10 || cfg.DefaultLimit != 200 || cfg.DangerousOpPolicy != guard.PolicyWarn {
		t.Errorf("Multiple options not applied correctly: %+v", cfg)
	}
	// Verify defaults for unset fields
	if cfg.MaxIdleConns != DefaultMaxIdleConns {
		t.Errorf("MaxIdleConns changed = %d, want %d", cfg.MaxIdleConns, DefaultMaxIdleConns)
	}
}
