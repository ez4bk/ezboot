package boot

import (
	"os"
	"strings"
	"testing"

	conf_type "github.com/ez4bk/ezboot/internal/gin-boot/conf-type"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
)

func TestDoMigrationDisabled(t *testing.T) {
	original := config.Conf
	t.Cleanup(func() {
		config.Conf = original
	})

	config.Conf = &config.Config{}

	if err := doMigration(); err != nil {
		t.Fatalf("doMigration() error = %v", err)
	}
}

func TestDoMigrationRequiresDefaultMysql(t *testing.T) {
	original := config.Conf
	t.Cleanup(func() {
		config.Conf = original
	})

	config.Conf = &config.Config{}
	config.Conf.App.Name = "demo"
	config.Conf.App.DBMigration.AutoMigrate = true

	err := doMigration()
	if err == nil || !strings.Contains(err.Error(), "middleware.mysql.default") {
		t.Fatalf("doMigration() error = %v, want missing default mysql error", err)
	}
}

func TestDoMigrationRequiresAppName(t *testing.T) {
	original := config.Conf
	t.Cleanup(func() {
		config.Conf = original
	})

	config.Conf = &config.Config{}
	config.Conf.App.DBMigration.AutoMigrate = true
	config.Conf.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {Dsn: "user:password@tcp(127.0.0.1:3306)/demo"},
	}

	err := doMigration()
	if err == nil || !strings.Contains(err.Error(), "app.name") {
		t.Fatalf("doMigration() error = %v, want missing app name error", err)
	}
}

func TestDoMigrationRequiresDefaultMysqlDSN(t *testing.T) {
	original := config.Conf
	t.Cleanup(func() {
		config.Conf = original
	})

	config.Conf = &config.Config{}
	config.Conf.App.Name = "demo"
	config.Conf.App.DBMigration.AutoMigrate = true
	config.Conf.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {},
	}

	err := doMigration()
	if err == nil || !strings.Contains(err.Error(), "mysql.default.dsn") {
		t.Fatalf("doMigration() error = %v, want missing default mysql dsn error", err)
	}
}

func TestDoMigrationRequiresSchemasDirectory(t *testing.T) {
	originalConfig := config.Conf
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	t.Cleanup(func() {
		config.Conf = originalConfig
		if err := os.Chdir(originalWorkingDirectory); err != nil {
			t.Errorf("restore working directory error = %v", err)
		}
	})

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	config.Conf = &config.Config{}
	config.Conf.App.Name = "demo"
	config.Conf.App.DBMigration.AutoMigrate = true
	config.Conf.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {Dsn: "user:password@tcp(127.0.0.1:3306)/demo"},
	}

	err = doMigration()
	if err == nil || !strings.Contains(err.Error(), "load migration schemas") {
		t.Fatalf("doMigration() error = %v, want missing schemas directory error", err)
	}
}
