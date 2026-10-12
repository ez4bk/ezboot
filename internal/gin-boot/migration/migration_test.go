package migration

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestCompletionDSNUsesAppMigrationTable(t *testing.T) {
	dsn, err := completionDSN(
		"user:password@tcp(127.0.0.1:3306)/demo?charset=utf8mb4",
		"things",
	)
	if err != nil {
		t.Fatalf("completionDSN() error = %v", err)
	}

	if !strings.HasPrefix(dsn, "mysql://") {
		t.Fatalf("completionDSN() = %q, want mysql:// prefix", dsn)
	}

	config, err := mysql.ParseDSN(strings.TrimPrefix(dsn, "mysql://"))
	if err != nil {
		t.Fatalf("mysql.ParseDSN() error = %v", err)
	}

	if got := config.Params["x-migrations-table"]; got != "things_schema_version" {
		t.Fatalf("x-migrations-table = %q, want %q", got, "things_schema_version")
	}
}
