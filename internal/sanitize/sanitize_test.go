package sanitize

import "testing"

func TestParamsNonString(t *testing.T) {
	args := []interface{}{42, 3.14, true, nil}
	got := Params(args)
	for i, v := range got {
		if v != args[i] {
			t.Errorf("Params[%d] = %v, want %v", i, v, args[i])
		}
	}
}

func TestParamsSensitivePrefixes(t *testing.T) {
	tests := []struct {
		value string
	}{
		{"sk-abc123def456"},
		{"sk_abc123def456"},
		{"pk-abc123def456"},
		{"pk_abc123def456"},
		{"AKIA1234567890ABC"},
		{"ASIA1234567890DEF"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := Params([]interface{}{tt.value})
			if got[0] != "[REDACTED]" {
				t.Errorf("Params(%q) = %v, want [REDACTED]", tt.value, got[0])
			}
		})
	}
}

func TestParamsSensitiveKeywords(t *testing.T) {
	tests := []string{
		"my_password_123",
		"the_secret_key",
		"auth_token_xyz",
		"credential_data",
		"passwd123",
		"pwd_abc",
		"MyPassword",
		"my-token-value",
	}
	for _, val := range tests {
		t.Run(val, func(t *testing.T) {
			got := Params([]interface{}{val})
			if got[0] != "[REDACTED]" {
				t.Errorf("Params(%q) = %v, want [REDACTED]", val, got[0])
			}
		})
	}
}

func TestParamsLongRandomString(t *testing.T) {
	// string longer than 20 chars that is >80% alphanumeric
	longRand := "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
	got := Params([]interface{}{longRand})
	if got[0] != "[REDACTED]" {
		t.Errorf("Params(long random string) = %v, want [REDACTED]", got[0])
	}

	// long string with many spaces should NOT be redacted
	longWithSpaces := "this is a normal sentence with lots of words in it"
	got2 := Params([]interface{}{longWithSpaces})
	if got2[0] == "[REDACTED]" {
		t.Errorf("Params(long sentence) should not be redacted, got [REDACTED]")
	}
}

func TestParamsShortString(t *testing.T) {
	short := "hello"
	got := Params([]interface{}{short})
	if got[0] != "hello" {
		t.Errorf("Params(%q) = %v, want %q", short, got[0], short)
	}
}

func TestParamsEmptyString(t *testing.T) {
	got := Params([]interface{}{""})
	if got[0] != "" {
		t.Errorf("Params('') = %v, want ''", got[0])
	}
}

func TestParamsMixed(t *testing.T) {
	args := []interface{}{"Alice", "sk-secret-key", 42, "password123", "hello"}
	got := Params(args)
	expected := []interface{}{"Alice", "[REDACTED]", 42, "[REDACTED]", "hello"}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("Params[%d] = %v, want %v", i, got[i], expected[i])
		}
	}
}

func TestSQLNoQuotes(t *testing.T) {
	sql := "SELECT * FROM users WHERE id = 1"
	got := SQL(sql)
	if got != sql {
		t.Errorf("SQL(%q) = %q, want %q", sql, got, sql)
	}
}

func TestSQLSingleQuotes(t *testing.T) {
	sql := "SELECT * FROM users WHERE name = 'Alice'"
	got := SQL(sql)
	want := "SELECT * FROM users WHERE name = '*****'"
	if got != want {
		t.Errorf("SQL(%q) = %q, want %q", sql, got, want)
	}
}

func TestSQLDoubleQuotes(t *testing.T) {
	sql := `SELECT * FROM users WHERE name = "Alice"`
	got := SQL(sql)
	want := `SELECT * FROM users WHERE name = "*****"`
	if got != want {
		t.Errorf("SQL(%q) = %q, want %q", sql, got, want)
	}
}

func TestSQLMultipleQuotedValues(t *testing.T) {
	sql := "INSERT INTO users (name, email) VALUES ('Alice', 'alice@test.com')"
	got := SQL(sql)
	want := "INSERT INTO users (name, email) VALUES ('*****', '**************')"
	if got != want {
		t.Errorf("SQL(%q) = %q, want %q", sql, got, want)
	}
}

func TestSQLEmpty(t *testing.T) {
	if got := SQL(""); got != "" {
		t.Errorf("SQL('') = %q, want ''", got)
	}
}

func TestSQLUnmatchedQuote(t *testing.T) {
	sql := "SELECT * FROM users WHERE name = 'incomplete"
	got := SQL(sql)
	// All chars after the opening quote become '*' — the mismatched quote handling
	if got == sql {
		t.Errorf("SQL should have masked the quoted content")
	}
}
