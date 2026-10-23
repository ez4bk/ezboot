package infra

import (
	"context"
	"errors"
	"testing"

	"xorm.io/xorm"

	"github.com/ez4bk/ezboot/internal/gin-boot/conf_type"
	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/logging"
)

func TestSetCloseIsIdempotentAndNilSafe(t *testing.T) {
	var nilSet *Set
	if err := nilSet.Close(context.Background()); err != nil {
		t.Fatalf("nil Close() = %v, want nil", err)
	}

	r := &Set{}
	for i := 0; i < 3; i++ {
		if err := r.Close(context.Background()); err != nil {
			t.Fatalf("Close() #%d = %v, want nil", i+1, err)
		}
	}
}

func TestSetMissingDependenciesReturnErrors(t *testing.T) {
	r := &Set{}

	tests := map[string]func() error{
		"db": func() error {
			_, err := r.DefaultDB()
			return err
		},
		"sqlite": func() error {
			_, err := r.SQLite("default")
			return err
		},
		"pgsql": func() error {
			_, err := r.DefaultPgsql()
			return err
		},
		"session": func() error {
			_, err := r.Session(context.Background(), "default")
			return err
		},
		"transaction": func() error {
			_, err := r.DefaultTransaction(context.Background(), func(_ *xorm.Session) (interface{}, error) {
				return nil, nil
			})
			return err
		},
		"redis": func() error {
			_, err := r.Redis("default")
			return err
		},
		"cache": func() error {
			_, err := r.DefaultCache()
			return err
		},
		"etcd": func() error {
			_, err := r.Etcd()
			return err
		},
		"kafka": func() error {
			_, err := r.Kafka()
			return err
		},
		"rabbitmq": func() error {
			_, err := r.RabbitMQ()
			return err
		},
		"elasticsearch": func() error {
			_, err := r.DefaultElasticsearch()
			return err
		},
	}

	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			err := call()
			if err == nil {
				t.Fatal("want an error for an unconfigured resource")
			}
			if !errors.Is(err, ErrResourceUnavailable) {
				t.Fatalf("error = %v, want errors.Is(ErrResourceUnavailable)", err)
			}
		})
	}
}

func TestOpenEmptyConfigAndClose(t *testing.T) {
	r, err := Open(&config.Config{}, logging.Default())
	if err != nil {
		t.Fatalf("Open(empty config) = %v", err)
	}
	if r == nil {
		t.Fatal("Open(empty config) returned nil Set")
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatalf("Close() = %v", err)
	}
}

func TestOpenRejectsNilConfig(t *testing.T) {
	if _, err := Open(nil, logging.Default()); err == nil {
		t.Fatal("Open(nil) = nil error, want error")
	}
}

func TestSetDefaultReplacesPointer(t *testing.T) {
	old := Instance
	t.Cleanup(func() { Instance = old })

	r := &Set{}
	SetDefault(r)
	if Default() != r {
		t.Fatal("Default() did not return the value passed to SetDefault")
	}

	SetDefault(nil)
	if Default() != r {
		t.Fatal("SetDefault(nil) changed the default")
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

func TestShutdownWithoutRabbitMQ(t *testing.T) {
	old := Instance
	t.Cleanup(func() { Instance = old })
	SetDefault(&Set{})

	if err := Shutdown(); err != nil {
		t.Fatalf("Shutdown() without RabbitMQ = %v, want nil", err)
	}
}

func TestOpenMySQLExposesTheEnginePool(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Middleware.Mysql = conf_type.Mysqls{
		"default": {
			Startup: true,
			Dsn:     "user:pass@tcp(127.0.0.1:1)/test",
		},
	}

	r, err := Open(cfg, logging.Default())
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	t.Cleanup(func() {
		if err := r.Close(context.Background()); err != nil {
			t.Errorf("Close() = %v", err)
		}
	})

	db, err := r.DefaultDB()
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

func TestSetHealthySkipsUnconfiguredDependencies(t *testing.T) {
	var nilSet *Set
	if err := nilSet.Healthy(context.Background()); err == nil {
		t.Fatal("nil Set.Healthy() = nil, want an error")
	}

	if err := (&Set{}).Healthy(context.Background()); err != nil {
		t.Fatalf("empty Set.Healthy() = %v, want nil", err)
	}
}
