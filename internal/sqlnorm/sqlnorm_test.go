package sqlnorm

import "testing"

func TestOperation(t *testing.T) {
	tests := []struct {
		sql string
		op  string
	}{
		{"SELECT * FROM users", "SELECT"},
		{"  select id from users", "SELECT"},
		{"insert into users (name) values (?)", "INSERT"},
		{"UPDATE users SET name = ? WHERE id = ?", "UPDATE"},
		{"DELETE FROM users WHERE id = ?", "DELETE"},
		{"CREATE TABLE users (id INT)", "CREATE"},
		{"ALTER TABLE users ADD COLUMN email TEXT", "ALTER"},
		{"DROP TABLE users", "DROP"},
		{"TRUNCATE TABLE users", "TRUNCATE"},
		{"  ", ""},
		{"", ""},
		{"SELECT", "SELECT"},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := Operation(tt.sql)
			if got != tt.op {
				t.Errorf("Operation(%q) = %q, want %q", tt.sql, got, tt.op)
			}
		})
	}
}

func TestHasWHERE(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"SELECT * FROM users", false},
		{"SELECT * FROM users WHERE id = 1", true},
		{"SELECT * FROM users WHERE name = 'WHERE'", true},    // WHERE inside value should be ignored
		{"SELECT * FROM users where id = 1", true},            // lowercase
		{"SELECT * FROM users WHERE", true},                   // incomplete but has WHERE
		{"UPDATE users SET name = 'foo' WHERE id = 1", true},
		{"UPDATE users SET name = 'WHERE clause text'", false}, // WHERE only in string literal
		{"DELETE FROM users", false},
		{"DELETE FROM users WHERE id = 1", true},
		{"INSERT INTO users (name) VALUES ('WHERE')", false}, // WHERE only in value
		{"SELECT *, (SELECT 1 FROM t WHERE x=1) AS sub FROM users", true},
		{"SELECT * FROM users WHERE name = 'hello'", true},
		{"SELECT * FROM users WHERE name = \"hello\"", true}, // Double quotes
		{"WHERE", true}, // Just the keyword
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := HasWHERE(tt.sql)
			if got != tt.want {
				t.Errorf("HasWHERE(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func TestRequiresWHERE(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{
		{"UPDATE", true},
		{"DELETE", true},
		{"SELECT", false},
		{"INSERT", false},
		{"CREATE", false},
		{"ALTER", false},
		{"DROP", false},
		{"TRUNCATE", false},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got := RequiresWHERE(tt.op)
			if got != tt.want {
				t.Errorf("RequiresWHERE(%q) = %v, want %v", tt.op, got, tt.want)
			}
		})
	}
}

func TestIsSELECT(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{
		{"SELECT", true},
		{"WITH", true},
		{"INSERT", false},
		{"UPDATE", false},
		{"DELETE", false},
		{"CREATE", false},
		{"DROP", false},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got := IsSELECT(tt.op)
			if got != tt.want {
				t.Errorf("IsSELECT(%q) = %v, want %v", tt.op, got, tt.want)
			}
		})
	}
}

func TestHasLIMIT(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"SELECT * FROM users", false},
		{"SELECT * FROM users LIMIT 10", true},
		{"SELECT * FROM users limit 100", true},
		{"SELECT * FROM users LIMIT", true},
		{"INSERT INTO users VALUES (1, 'LIMIT')", false},
		{"SELECT * FROM users WHERE name = 'limit'", false},
		{"SELECT * FROM (SELECT id FROM t LIMIT 5) AS sub", true},
		{"SELECT * FROM users LIMIT 100 OFFSET 20", true},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := HasLIMIT(tt.sql)
			if got != tt.want {
				t.Errorf("HasLIMIT(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func TestAppendLIMIT(t *testing.T) {
	tests := []struct {
		sql   string
		limit int
		want  string
	}{
		{"SELECT * FROM users", 100, "SELECT * FROM users LIMIT 100"},
		{"SELECT * FROM users WHERE id = 1", 50, "SELECT * FROM users WHERE id = 1 LIMIT 50"},
		{"SELECT * FROM users;", 100, "SELECT * FROM users LIMIT 100;"},
		{"select id, name from users", 10, "select id, name from users LIMIT 10"},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := AppendLIMIT(tt.sql, tt.limit)
			if got != tt.want {
				t.Errorf("AppendLIMIT(%q, %d) = %q, want %q", tt.sql, tt.limit, got, tt.want)
			}
		})
	}
}

func TestAppendLimitPositive(t *testing.T) {
	got := AppendLIMIT("SELECT * FROM users", 42)
	want := "SELECT * FROM users LIMIT 42"
	if got != want {
		t.Errorf("AppendLIMIT(..., 42) = %q, want %q", got, want)
	}
}

func TestAppendLimitZero(t *testing.T) {
	got := AppendLIMIT("SELECT * FROM users", 0)
	want := "SELECT * FROM users LIMIT 0"
	if got != want {
		t.Errorf("AppendLIMIT(..., 0) = %q, want %q", got, want)
	}
}

func TestHasLIMITNoMatch(t *testing.T) {
	if HasLIMIT("SELECT * FROM users") {
		t.Error("HasLIMIT expected false for no LIMIT")
	}
	if HasLIMIT("INSERT INTO t VALUES ('limit test')") {
		t.Error("HasLIMIT expected false for LIMIT inside string literal")
	}
}

func TestHasLIMITExisting(t *testing.T) {
	if !HasLIMIT("SELECT * FROM users LIMIT 10") {
		t.Error("HasLIMIT expected true for existing LIMIT")
	}
	if !HasLIMIT("SELECT * FROM users LIMIT") {
		t.Error("HasLIMIT expected true for bare LIMIT keyword")
	}
}

func TestMatchKeywordMultiple(t *testing.T) {
	if !matchKeyword("WHERE x = 1 AND WHERE y = 2", "WHERE") {
		t.Error("matchKeyword expected true for multiple WHERE")
	}
}

func TestHasWHEREKeywordEmbedded(t *testing.T) {
	// "WHERE" should NOT match when it's part of another word like "SOMEWHERE"
	if HasWHERE("SELECT * FROM somewhere") {
		t.Error("HasWHERE should not match WHERE inside 'somewhere'")
	}
	// First occurrence is embedded, second is standalone — should match
	if !HasWHERE("SELECT * FROM somewhere WHERE x = 1") {
		t.Error("HasWHERE should match standalone WHERE after embedded occurrence")
	}
}

func TestHasLIMITKeywordEmbedded(t *testing.T) {
	// "LIMIT" should NOT match when inside another word like "LIMITATION"
	if HasLIMIT("SELECT * FROM limitation_test") {
		t.Error("HasLIMIT should not match LIMIT inside 'limitation'")
	}
	// First occurrence embedded, second standalone — should match
	if !HasLIMIT("SELECT * FROM limitation_test LIMIT 5") {
		t.Error("HasLIMIT should match standalone LIMIT after embedded occurrence")
	}
}

func TestItoaNegative(t *testing.T) {
	got := AppendLIMIT("SELECT * FROM users", -5)
	// itoa handles negative via AppendLIMIT called with a negative limit
	// The actual output format: "SELECT * FROM users LIMIT -5"
	if got != "SELECT * FROM users LIMIT -5" {
		t.Errorf("AppendLIMIT with negative = %q, want %q", got, "SELECT * FROM users LIMIT -5")
	}
}
