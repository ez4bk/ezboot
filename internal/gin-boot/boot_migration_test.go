package boot

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
)

func TestDoMigrationDisabled(t *testing.T) {
	original := config.Conf
	t.Cleanup(func() {
		config.Conf = original
	})

	config.Conf = &config.Config{}

	if err := doMigration(nil); err != nil {
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

	err := doMigration(nil)
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

	err := doMigration(nil)
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

	err := doMigration(nil)
	if err == nil || !strings.Contains(err.Error(), "mysql.default.dsn") {
		t.Fatalf("doMigration() error = %v, want missing default mysql dsn error", err)
	}
}

// TestDoMigrationRequiresMigrationFS 覆盖 embeds 成为唯一迁移来源后的行为：
// 没有注入文件系统时必须立刻报错，而不是静默跳过或去读磁盘。
func TestDoMigrationRequiresMigrationFS(t *testing.T) {
	original := config.Conf
	t.Cleanup(func() {
		config.Conf = original
	})

	config.Conf = &config.Config{}
	config.Conf.App.Name = "demo"
	config.Conf.App.DBMigration.AutoMigrate = true
	config.Conf.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {Dsn: "user:password@tcp(127.0.0.1:3306)/demo"},
	}

	err := doMigration(nil)
	if err == nil || !strings.Contains(err.Error(), "migration fs cannot be nil") {
		t.Fatalf("doMigration() error = %v, want missing migration fs error", err)
	}
}

// TestDoMigrationUsesInjectedFS 确认注入的文件系统取代了工作目录下的
// schemas/：即便当前目录没有 schemas/，也应当继续走到连接数据库阶段。
func TestDoMigrationUsesInjectedFS(t *testing.T) {
	original := config.Conf
	t.Cleanup(func() {
		config.Conf = original
	})

	t.Chdir(t.TempDir())

	config.Conf = &config.Config{}
	config.Conf.App.Name = "demo"
	config.Conf.App.DBMigration.AutoMigrate = true
	config.Conf.App.Middleware.Mysql = conf_type.Mysqls{
		// 端口 1 必定连不上，用 timeout 兜住极端环境下的拨号等待。
		"default": {Dsn: "user:password@tcp(127.0.0.1:1)/demo?timeout=1s"},
	}

	fsys := fstest.MapFS{
		"000001_demo.up.sql":   &fstest.MapFile{Data: []byte("SELECT 1;")},
		"000001_demo.down.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}

	err := doMigration(fsys)
	if err == nil {
		t.Fatal("doMigration() error = nil, want connection failure")
	}
	if strings.Contains(err.Error(), "load migration schemas") {
		t.Fatalf("doMigration() error = %v, injected fs must replace the disk schemas directory", err)
	}
	if !strings.Contains(err.Error(), "create migration failed") {
		t.Fatalf("doMigration() error = %v, want failure after the source driver was loaded", err)
	}
}
