package sqlnorm

import (
	"reflect"
	"testing"
)

func TestSplitStatementsEmpty(t *testing.T) {
	if stmts := SplitStatements(""); len(stmts) != 0 {
		t.Errorf("SplitStatements('') = %v, want empty", stmts)
	}
}

func TestSplitStatementsWhitespace(t *testing.T) {
	if stmts := SplitStatements("  \n\t  "); len(stmts) != 0 {
		t.Errorf("SplitStatements(whitespace) = %v, want empty", stmts)
	}
}

func TestSplitStatementsSemicolonsOnly(t *testing.T) {
	if stmts := SplitStatements(";;;"); len(stmts) != 0 {
		t.Errorf("SplitStatements(';;;') = %v, want empty", stmts)
	}
}

func TestSplitStatementsSingle(t *testing.T) {
	stmts := SplitStatements("SELECT 1")
	want := []string{"SELECT 1"}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsMultiple(t *testing.T) {
	sql := "CREATE TABLE t (id INT); INSERT INTO t VALUES (1); SELECT * FROM t"
	stmts := SplitStatements(sql)
	want := []string{
		"CREATE TABLE t (id INT)",
		"INSERT INTO t VALUES (1)",
		"SELECT * FROM t",
	}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsTrailingSemicolon(t *testing.T) {
	stmts := SplitStatements("SELECT 1;")
	want := []string{"SELECT 1"}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsSemicolonsInStrings(t *testing.T) {
	sql := `SELECT 'hello;world'; INSERT INTO t VALUES ('a;b')`
	stmts := SplitStatements(sql)
	want := []string{
		`SELECT 'hello;world'`,
		`INSERT INTO t VALUES ('a;b')`,
	}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsSemicolonsInDoubleQuotes(t *testing.T) {
	sql := `SELECT "hello;world" AS msg`
	stmts := SplitStatements(sql)
	want := []string{`SELECT "hello;world" AS msg`}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsSemicolonsInBackticks(t *testing.T) {
	sql := "CREATE TABLE `t;blah` (id INT); SELECT 1"
	stmts := SplitStatements(sql)
	want := []string{
		"CREATE TABLE `t;blah` (id INT)",
		"SELECT 1",
	}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsCommentLine(t *testing.T) {
	sql := "SELECT 1; -- comment; here\nSELECT 2"
	stmts := SplitStatements(sql)
	want := []string{"SELECT 1", "SELECT 2"}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsBlockComment(t *testing.T) {
	sql := "SELECT 1; /* block; comment */ SELECT 2"
	stmts := SplitStatements(sql)
	want := []string{"SELECT 1", "SELECT 2"}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsEscapedQuote(t *testing.T) {
	sql := `SELECT 'it''s; test' AS msg; SELECT 2`
	stmts := SplitStatements(sql)
	want := []string{`SELECT 'it''s; test' AS msg`, "SELECT 2"}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsLeadingAndTrailing(t *testing.T) {
	sql := "  \nSELECT 1;\n\nSELECT 2;\n  "
	stmts := SplitStatements(sql)
	want := []string{"SELECT 1", "SELECT 2"}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %v, want %v", stmts, want)
	}
}

func TestSplitStatementsRealisticMigration(t *testing.T) {
	sql := `CREATE TABLE users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(255) NOT NULL
);

INSERT INTO users (name) VALUES ('Alice');
INSERT INTO users (name) VALUES ('Bob; the builder');

CREATE INDEX idx_name ON users(name);`
	stmts := SplitStatements(sql)
	want := []string{
		"CREATE TABLE users (\n  id INT AUTO_INCREMENT PRIMARY KEY,\n  name VARCHAR(255) NOT NULL\n)",
		"INSERT INTO users (name) VALUES ('Alice')",
		"INSERT INTO users (name) VALUES ('Bob; the builder')",
		"CREATE INDEX idx_name ON users(name)",
	}
	if !reflect.DeepEqual(stmts, want) {
		t.Errorf("SplitStatements() = %d statements, want %d", len(stmts), len(want))
		for i := 0; i < len(stmts) && i < len(want); i++ {
			if stmts[i] != want[i] {
				t.Errorf("  stmt[%d] = %q, want %q", i, stmts[i], want[i])
			}
		}
	}
}
