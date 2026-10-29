package infra

import (
	"context"
	"errors"
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
			_, err := NewMySQL(empty, logging.Default())
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
			_, err := NewMySQL(nil, logging.Default())
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
	mysqlCfg.App.Middleware.Mysql = conf_type.Mysqls{"default": {}}

	mysql, err := NewMySQL(mysqlCfg, logging.Default())
	if err != nil {
		t.Fatalf("NewMySQL: %v", err)
	}
	if mysql.Name() != "mysql" {
		t.Fatalf("Name() = %q", mysql.Name())
	}
}

func TestPluginCloseIsNilSafe(t *testing.T) {
	ctx := context.Background()

	if err := (*MySQL)(nil).Close(ctx); err != nil {
		t.Fatalf("nil MySQL.Close() = %v", err)
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

func TestMySQLExposesTheEnginePool(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {
			Startup: true,
			Dsn:     "user:pass@tcp(127.0.0.1:1)/test",
		},
	}

	p, err := NewMySQL(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewMySQL() = %v", err)
	}
	t.Cleanup(func() {
		if err := p.Close(context.Background()); err != nil {
			t.Errorf("Close() = %v", err)
		}
	})

	db, err := p.DefaultDB()
	if err != nil {
		t.Fatalf("DefaultDB() = %v", err)
	}
	if db.Engine() == nil {
		t.Fatal("DB.Engine() is nil")
	}
	if db.SQL() == nil {
		t.Fatal("DB.SQL() is nil; the xorm engine pool was not exposed")
	}
	if got := db.SQL(); got != db.Engine().DB().DB {
		t.Fatal("DB.SQL() does not point at the xorm engine pool")
	}
}

func TestMySQLStartRegistersTheDefaultDatabase(t *testing.T) {
	with.SetDefault(nil)
	t.Cleanup(func() { with.SetDefault(nil) })

	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {
			Startup: true,
			Dsn:     "user:pass@tcp(127.0.0.1:1)/test",
		},
	}

	p, err := NewMySQL(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewMySQL() = %v", err)
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

	mysql, err := NewMySQL(cfg, logging.Default())
	if err != nil {
		t.Fatalf("NewMySQL: %v", err)
	}
	t.Cleanup(func() { _ = mysql.Close(context.Background()) })

	engine, err := ProvideDefaultMySQL(mysql)
	if err != nil {
		t.Fatalf("ProvideDefaultMySQL: %v", err)
	}
	if engine == nil {
		t.Fatal("ProvideDefaultMySQL returned a nil engine")
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
