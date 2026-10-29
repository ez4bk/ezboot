package infra

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"xorm.io/xorm"

	"github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/logging"
	"github.com/ez4bk/ezboot/with"
)

func TestPluginsRequireConfiguration(t *testing.T) {
	empty := &config.Config{}

	tests := map[string]func() error{
		"mysql": func() error {
			_, err := NewDatabases(empty, logging.Default())
			return err
		},
		"redis": func() error {
			_, err := NewRedis(empty)
			return err
		},
		"kafka": func() error {
			_, err := NewKafka(empty)
			return err
		},
		"rabbitmq": func() error {
			_, err := NewRabbitMQ(empty, logging.Default())
			return err
		},
		"elasticsearch": func() error {
			_, err := NewElasticsearch(empty)
			return err
		},
		"etcd": func() error {
			_, err := NewEtcd(empty)
			return err
		},
		"nil-config": func() error {
			_, err := NewDatabases(nil, logging.Default())
			return err
		},
	}

	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("want an error for an unconfigured plugin")
			}
		})
	}
}

func TestPluginNames(t *testing.T) {
	mysqlCfg := &config.Config{}
	mysqlCfg.App.Middleware.Mysql = conf_type.Mysqls{"default": {Startup: true, Dsn: "user:pass@tcp(127.0.0.1:1)/test"}}

	databases, err := NewDatabases(mysqlCfg, logging.Default())
	if err != nil {
		t.Fatalf("NewDatabases: %v", err)
	}
	if databases.Name() != "databases" {
		t.Fatalf("Name() = %q", databases.Name())
	}
}

func TestPluginCloseIsNilSafe(t *testing.T) {
	ctx := context.Background()

	if err := (*Databases)(nil).Close(ctx); err != nil {
		t.Fatalf("nil Databases.Close() = %v", err)
	}
	if err := (*Redis)(nil).Close(ctx); err != nil {
		t.Fatalf("nil Redis.Close() = %v", err)
	}
	if err := (*Kafka)(nil).Close(ctx); err != nil {
		t.Fatalf("nil Kafka.Close() = %v", err)
	}
	if err := (*RabbitMQ)(nil).Close(ctx); err != nil {
		t.Fatalf("nil RabbitMQ.Close() = %v", err)
	}
	if err := (*Elasticsearch)(nil).Close(ctx); err != nil {
		t.Fatalf("nil Elasticsearch.Close() = %v", err)
	}
	if err := (*Etcd)(nil).Close(ctx); err != nil {
		t.Fatalf("nil Etcd.Close() = %v", err)
	}
}

func TestDatabasesExposesEngines(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {
			Startup: true,
			Dsn:     "user:pass@tcp(127.0.0.1:1)/test",
		},
	}

	p, err := NewDatabases(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewDatabases() = %v", err)
	}
	t.Cleanup(func() {
		if err := p.Close(context.Background()); err != nil {
			t.Errorf("Close() = %v", err)
		}
	})

	engine, err := p.DefaultEngine()
	if err != nil {
		t.Fatalf("DefaultEngine() = %v", err)
	}
	if engine == nil {
		t.Fatal("DefaultEngine() returned a nil engine")
	}
	if engine.DB().DB == nil {
		t.Fatal("the xorm engine pool was not exposed")
	}
}

func TestDatabasesStartRegistersTheDefaultDatabase(t *testing.T) {
	with.SetDefault(nil)
	t.Cleanup(func() { with.SetDefault(nil) })

	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {
			Startup: true,
			Dsn:     "user:pass@tcp(127.0.0.1:1)/test",
		},
	}

	p, err := NewDatabases(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewDatabases() = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := with.DefaultSession(context.Background(), func(*xorm.Session) error { return nil }); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("the default database was never registered")
		}
		time.Sleep(time.Millisecond)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start() = %v", err)
	}
}

func TestResourceErrorMatchesSentinelAndCause(t *testing.T) {
	cause := errors.New("connection refused")
	err := resourceError("redis", "cache", cause)

	if !errors.Is(err, ErrResourceUnavailable) {
		t.Fatalf("errors.Is(%v, ErrResourceUnavailable) = false", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(%v, cause) = false", err)
	}
	if err.Error() == "" {
		t.Fatal("ResourceError.Error() is empty")
	}
}

func TestDefaultProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {Startup: true, Dsn: "user:pass@tcp(127.0.0.1:1)/test"},
	}
	cfg.App.Middleware.Redis = conf_type.Redises{
		"default": {Startup: true, Addr: "127.0.0.1:6379"},
	}

	databases, err := NewDatabases(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewDatabases: %v", err)
	}
	t.Cleanup(func() { _ = databases.Close(context.Background()) })

	engine, err := ProvideDefaultEngine(databases)
	if err != nil {
		t.Fatalf("ProvideDefaultEngine: %v", err)
	}
	if engine == nil {
		t.Fatal("ProvideDefaultEngine returned a nil engine")
	}

	redisPlugin, err := NewRedis(cfg)
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	t.Cleanup(func() { _ = redisPlugin.Close(context.Background()) })

	client, err := ProvideDefaultRedis(redisPlugin)
	if err != nil {
		t.Fatalf("ProvideDefaultRedis: %v", err)
	}
	if client == nil {
		t.Fatal("ProvideDefaultRedis returned a nil client")
	}
}

func TestDatabasesRejectsMultipleDialects(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {Startup: true, Dsn: "user:pass@tcp(127.0.0.1:1)/test"},
	}
	cfg.App.Middleware.Pgsql = conf_type.Pgsqls{
		"default": {Startup: true, URL: "postgres://user:pass@127.0.0.1:1/db"},
	}

	_, err := NewDatabases(cfg, logging.Default())
	if err == nil {
		t.Fatal("expected an error when two dialects are configured")
	}
	if !strings.Contains(err.Error(), "only one database dialect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDatabasesAllowsMultipleInstancesOfOneDialect(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {Startup: true, Dsn: "user:pass@tcp(127.0.0.1:1)/main"},
		"report":  {Startup: true, Dsn: "user:pass@tcp(127.0.0.1:1)/report"},
	}

	p, err := NewDatabases(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewDatabases: %v", err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })

	if p.Dialect() != DialectMySQL {
		t.Fatalf("Dialect() = %q", p.Dialect())
	}
	if _, err := p.Engine("report"); err != nil {
		t.Fatalf("Engine(report): %v", err)
	}
}

func TestDatabasesPostgres(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Middleware.Pgsql = conf_type.Pgsqls{
		"default": {Startup: true, URL: "postgres://user:pass@127.0.0.1:1/db"},
	}

	p, err := NewDatabases(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewDatabases: %v", err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })

	if p.Dialect() != DialectPostgres {
		t.Fatalf("Dialect() = %q", p.Dialect())
	}
	if _, err := ProvideDefaultEngine(p); err != nil {
		t.Fatalf("ProvideDefaultEngine: %v", err)
	}
}
