package guard

import "testing"

func TestPolicyString(t *testing.T) {
	tests := []struct {
		p    Policy
		want string
	}{
		{PolicyBlock, "block"},
		{PolicyWarn, "warn"},
		{PolicyAllow, "allow"},
		{Policy(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsDangerousOp(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"DROP TABLE users", true},
		{"drop table users", true},
		{"DROP   TABLE users", true},
		{"DROP\tTABLE", true},
		{"TRUNCATE TABLE users", true},
		{"truncate table users", true},
		{"SELECT * FROM users", false},
		{"INSERT INTO users VALUES (1)", false},
		{"UPDATE users SET name = 'test'", false},
		{"DELETE FROM users", false},
		{"DROPPED_VIEW", false},
		{"", false},
		{"DROP", true},
		{"TRUNCATE", true},
		{"SELECT DROP", false},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := IsDangerousOp(tt.sql)
			if got != tt.want {
				t.Errorf("IsDangerousOp(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func TestCheckPolicyAllow(t *testing.T) {
	if err := Check(PolicyAllow, "DROP TABLE users"); err != nil {
		t.Errorf("Check(Allow, DROP) error = %v, want nil", err)
	}
}

func TestCheckPolicyBlock(t *testing.T) {
	tests := []struct {
		sql string
		err bool
	}{
		{"DROP TABLE users", true},
		{"TRUNCATE TABLE users", true},
		{"SELECT * FROM users", false},
		{"INSERT INTO users VALUES (1)", false},
		{"UPDATE users SET name = 'test' WHERE id = 1", false},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			err := Check(PolicyBlock, tt.sql)
			if tt.err && err == nil {
				t.Errorf("Check(Block, %q) expected error, got nil", tt.sql)
			}
			if !tt.err && err != nil {
				t.Errorf("Check(Block, %q) unexpected error = %v", tt.sql, err)
			}
		})
	}
}

func TestCheckPolicyWarn(t *testing.T) {
	if err := Check(PolicyWarn, "DROP TABLE users"); err != nil {
		t.Errorf("Check(Warn, DROP) error = %v, want nil", err)
	}
	if err := Check(PolicyWarn, "SELECT * FROM users"); err != nil {
		t.Errorf("Check(Warn, SELECT) error = %v, want nil", err)
	}
}

func TestCheckReturnsErrDangerousOp(t *testing.T) {
	err := Check(PolicyBlock, "DROP TABLE users")
	if err != ErrDangerousOp {
		t.Errorf("Check(Block, DROP) = %v, want %v", err, ErrDangerousOp)
	}
}

func TestDangerousOpsList(t *testing.T) {
	if len(DangerousOps) != 2 {
		t.Errorf("len(DangerousOps) = %d, want 2", len(DangerousOps))
	}
	if DangerousOps[0] != "DROP" {
		t.Errorf("DangerousOps[0] = %q, want %q", DangerousOps[0], "DROP")
	}
	if DangerousOps[1] != "TRUNCATE" {
		t.Errorf("DangerousOps[1] = %q, want %q", DangerousOps[1], "TRUNCATE")
	}
}
