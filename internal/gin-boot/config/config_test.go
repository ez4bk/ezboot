package config

import (
	"testing"

	"github.com/ez4bk/ezboot/internal/gin-boot/util"
)

func TestMysql(t *testing.T) {
}

func TestRedis(t *testing.T) {
}

func TestDecryptMysqlDsn(t *testing.T) {
	user, err := util.Encrypt("user", "config-secret")
	if err != nil {
		t.Fatal(err)
	}
	password, err := util.Encrypt("password", "config-secret")
	if err != nil {
		t.Fatal(err)
	}

	dsn := user + ":" + password + "@tcp(127.0.0.1:3306)/db"
	got, err := decryptMysqlDsn(dsn, "local.yml", "config-secret")
	if err != nil {
		t.Fatalf("decryptMysqlDsn() = %v", err)
	}
	const want = "user:password@tcp(127.0.0.1:3306)/db"
	if got != want {
		t.Fatalf("decryptMysqlDsn() = %q, want %q", got, want)
	}
}
