package dsn

import "testing"

func TestDetectMySQL(t *testing.T) {
	tests := []struct {
		dsn  string
		name string
	}{
		{dsn: "user:password@tcp(127.0.0.1:3306)/dbname", name: "standard tcp"},
		{dsn: "user@tcp(127.0.0.1:3306)/dbname", name: "tcp no password"},
		{dsn: "user:password@tcp(host:3306)/dbname?charset=utf8mb4", name: "tcp with params"},
		{dsn: "user@unix(/var/run/mysqld/mysqld.sock)/dbname", name: "unix socket"},
		{dsn: "user:pass@tcp4(1.2.3.4:3306)/db", name: "tcp4"},
		{dsn: "user:pass@tcp6([::1]:3306)/db", name: "tcp6"},
		{dsn: "user@hostname/dbname", name: "mysql fallback user@host/db"},
		{dsn: "mysql://user:pass@host:3306/db", name: "mysql:// scheme"},
		{dsn: "MYSQL://user:pass@host:3306/db", name: "MYSQL:// uppercase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.dsn)
			if err != nil {
				t.Fatalf("Detect(%q) error = %v", tt.dsn, err)
			}
			if got != DriverMySQL {
				t.Errorf("Detect(%q) = %q, want %q", tt.dsn, got, DriverMySQL)
			}
		})
	}
}

func TestDetectPostgres(t *testing.T) {
	tests := []struct {
		dsn  string
		name string
	}{
		{dsn: "postgres://user:password@localhost:5432/dbname", name: "standard"},
		{dsn: "postgresql://user@localhost/dbname", name: "postgresql:// scheme"},
		{dsn: "POSTGRES://user:pass@host:5432/db", name: "POSTGRES:// uppercase"},
		{dsn: "PostgreSQL://user:pass@host:5432/db", name: "PostgreSQL:// mixed case"},
		{dsn: "postgres://localhost/dbname?sslmode=disable", name: "with query params"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.dsn)
			if err != nil {
				t.Fatalf("Detect(%q) error = %v", tt.dsn, err)
			}
			if got != DriverPostgres {
				t.Errorf("Detect(%q) = %q, want %q", tt.dsn, got, DriverPostgres)
			}
		})
	}
}

func TestDetectSQLite(t *testing.T) {
	tests := []struct {
		dsn  string
		name string
	}{
		{dsn: ":memory:", name: "in-memory"},
		{dsn: "file:/path/to/db.sqlite", name: "file: sqlite"},
		{dsn: "file:data.db", name: "file: relative"},
		{dsn: "file:///absolute/path.db", name: "file:// absolute"},
		{dsn: "/tmp/data.db", name: ".db extension absolute"},
		{dsn: "data.db", name: ".db extension relative"},
		{dsn: "mydatabase.sqlite", name: ".sqlite extension"},
		{dsn: "app.sqlite3", name: ".sqlite3 extension"},
		{dsn: "test.s3db", name: ".s3db extension"},
		{dsn: "archive.db3", name: ".db3 extension"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.dsn)
			if err != nil {
				t.Fatalf("Detect(%q) error = %v", tt.dsn, err)
			}
			if got != DriverSQLite {
				t.Errorf("Detect(%q) = %q, want %q", tt.dsn, got, DriverSQLite)
			}
		})
	}
}

func TestDetectOracle(t *testing.T) {
	tests := []struct {
		dsn  string
		name string
	}{
		{dsn: "user/password@host:1521/service", name: "standard oracle"},
		{dsn: "user/password@//host:1521/service", name: "oracle //host"},
		{dsn: "scott/tiger@localhost:1521/XEPDB1", name: "oracle with service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.dsn)
			if err != nil {
				t.Fatalf("Detect(%q) error = %v", tt.dsn, err)
			}
			if got != DriverOracle {
				t.Errorf("Detect(%q) = %q, want %q", tt.dsn, got, DriverOracle)
			}
		})
	}
}

func TestDetectErrors(t *testing.T) {
	tests := []struct {
		dsn  string
		name string
	}{
		{dsn: "", name: "empty"},
		{dsn: "   ", name: "whitespace only"},
		{dsn: "justarandomstring", name: "random string"},
		{dsn: "mongodb://localhost:27017/db", name: "mongodb"},
		{dsn: "redis://localhost:6379", name: "redis"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Detect(tt.dsn)
			if err == nil {
				t.Errorf("Detect(%q) expected error, got nil", tt.dsn)
			}
		})
	}
}

func TestDetectMySQLvsOracleAmbiguous(t *testing.T) {
	// User/pass@host:port could theoretically match both, but Oracle-convention
	// uses user/pass@host (user and pass separated by /), while MySQL uses
	// user:pass@host (user and pass separated by :).
	// Test that the format with `:` before @ is MySQL
	got, err := Detect("user:password@host:3306/dbname")
	if err != nil {
		t.Fatalf("Detect mysql-like with colon = %v", err)
	}
	if got != DriverMySQL {
		t.Errorf("Detect('user:password@host:3306/dbname') = %q, want %q", got, DriverMySQL)
	}
}

func TestDetectMySQLNetPattern(t *testing.T) {
	// MySQL with custom host:port without tcp() — uses @host:port/db format
	tests := []struct {
		dsn string
		msg string
	}{
		{"user:pass@hostname:3306/db", "standard host:port/db"},
		{"app@server.example.com/appdb", "user@host/db"},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got, err := Detect(tt.dsn)
			if err != nil {
				t.Fatalf("Detect(%q) error = %v", tt.dsn, err)
			}
			if got != DriverMySQL {
				t.Errorf("Detect(%q) = %q, want %q", tt.dsn, got, DriverMySQL)
			}
		})
	}
}
