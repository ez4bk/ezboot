package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	_ "github.com/jackc/pgx/v5/stdlib"
	"xorm.io/xorm"

	"github.com/ez4bk/ezboot"
	"github.com/ez4bk/ezboot/with"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware/tracing/otelxorm"
	"github.com/ez4bk/ezboot/internal/logging"
)

// Dialect 是 Databases 插件当前使用的数据库方言。
type Dialect string

const (
	DialectMySQL    Dialect = "mysql"
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// Databases 是关系型数据库插件，统一通过 xorm 管理配置里的
// MySQL / SQLite / Postgres 实例。
//
// 同一时刻只允许配置一种方言，但同一种方言可以有多个命名实例
// （主库、报表库、分片库等）。三种方言的默认实例都是 *xorm.Engine，
// 因此只有一个 ProvideDefaultEngine。
type Databases struct {
	ezboot.PluginBase

	mu      sync.RWMutex
	dialect Dialect
	engines map[string]*xorm.Engine
}

// DatabasesProviderSet 提供数据库插件，以及 default 实例的 xorm 引擎。
var DatabasesProviderSet = wire.NewSet(NewDatabases, ProvideDefaultEngine)

// NewDatabases 创建插件并建立连接池。连接池在第一次查询时才真正连接
// 数据库，所以这里不产生网络 I/O。
func NewDatabases(cfg *config.Config, logger *logging.Logger) (*Databases, error) {
	if cfg == nil {
		return nil, errors.New("infra: databases requires a config")
	}

	dialect, err := detectDialect(cfg)
	if err != nil {
		return nil, err
	}

	d := &Databases{dialect: dialect}
	if err := d.open(cfg, loggerOrDefault(logger)); err != nil {
		return nil, err
	}

	return d, nil
}

// detectDialect 统计配置里实际启用的方言。零种或多于一种都视为配置
// 错误；同一种方言的多个实例是允许的。
func detectDialect(cfg *config.Config) (Dialect, error) {
	var found []Dialect

	for _, c := range cfg.App.Middleware.Mysql {
		if c.Startup {
			found = append(found, DialectMySQL)
			break
		}
	}
	for _, c := range cfg.App.Middleware.Sqlite {
		if c.Startup {
			found = append(found, DialectSQLite)
			break
		}
	}
	for _, c := range cfg.App.Middleware.Pgsql {
		if c.Startup {
			found = append(found, DialectPostgres)
			break
		}
	}

	switch len(found) {
	case 0:
		return "", errors.New("infra: databases plugin is enabled but no database is configured")
	case 1:
		return found[0], nil
	default:
		names := make([]string, len(found))
		for i, d := range found {
			names[i] = string(d)
		}
		return "", fmt.Errorf("infra: only one database dialect may be configured, got %s", strings.Join(names, ", "))
	}
}

func (d *Databases) open(cfg *config.Config, logger *logging.Logger) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.engines = make(map[string]*xorm.Engine)

	switch d.dialect {
	case DialectMySQL:
		for name, c := range cfg.App.Middleware.Mysql {
			if !c.Startup {
				continue
			}

			engine, err := xorm.NewEngine("mysql", c.Dsn)
			if err != nil {
				return fmt.Errorf("open mysql %q: %w", name, err)
			}

			applyPool(engine, c.MaxIdleConnection, c.MaxOpenConnection, c.MaxIdleTime)
			engine.ShowSQL(!c.HiddenSQL)
			engine.SetLogger(NewXormLogger(logger, !c.HiddenSQL))

			if cfg.App.Middleware.Tracing.Database.Enabled {
				engine.AddHook(otelxorm.NewTracingHook(&cfg.App.Middleware.Tracing))
			}

			d.engines[name] = engine
		}

	case DialectSQLite:
		for name, c := range cfg.App.Middleware.Sqlite {
			if !c.Startup {
				continue
			}

			schema, rest, err := parseDSN(c.Dsn)
			if err != nil {
				return fmt.Errorf("open sqlite %q: %w", name, err)
			}

			engine, err := xorm.NewEngine(schema, rest)
			if err != nil {
				return fmt.Errorf("open sqlite %q: %w", name, err)
			}

			applyPool(engine, c.MaxIdleConnection, c.MaxOpenConnection, 0)
			engine.ShowSQL(!c.HiddenSQL)
			engine.SetLogger(NewXormLogger(logger, !c.HiddenSQL))

			d.engines[name] = engine
		}

	case DialectPostgres:
		for name, c := range cfg.App.Middleware.Pgsql {
			if !c.Startup {
				continue
			}

			engine, err := xorm.NewEngine("pgx", c.URL)
			if err != nil {
				return fmt.Errorf("open postgres %q: %w", name, err)
			}

			applyPool(engine, c.MaxIdleConnection, c.MaxOpenConnection, c.MaxIdleTime)
			engine.ShowSQL(!c.HiddenSQL)
			engine.SetLogger(NewXormLogger(logger, !c.HiddenSQL))

			d.engines[name] = engine
		}
	}

	return nil
}

func applyPool(engine *xorm.Engine, maxIdle, maxOpen int, maxIdleTime time.Duration) {
	if maxIdle != 0 {
		engine.SetMaxIdleConns(maxIdle)
	}
	if maxOpen != 0 {
		engine.SetMaxOpenConns(maxOpen)
	}
	if maxIdleTime != 0 {
		engine.SetConnMaxLifetime(maxIdleTime)
		engine.DB().SetConnMaxIdleTime(maxIdleTime)
	}
}

func (d *Databases) close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var errs []error
	for name, engine := range d.engines {
		if err := engine.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s %q: %w", d.dialect, name, err))
		}
	}

	d.engines = nil

	return errors.Join(errs...)
}

func (d *Databases) healthy(ctx context.Context) []error {
	d.mu.RLock()
	engines := make(map[string]*xorm.Engine, len(d.engines))
	for name, engine := range d.engines {
		engines[name] = engine
	}
	d.mu.RUnlock()

	var errs []error
	for name, engine := range engines {
		if err := engine.PingContext(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s %q: %w", d.dialect, name, err))
		}
	}
	return errs
}

// Engine 返回指定名称的 xorm 引擎。
func (d *Databases) Engine(name string) (*xorm.Engine, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	engine, ok := d.engines[name]
	if !ok {
		return nil, resourceError(string(d.dialect), name, nil)
	}
	return engine, nil
}

// DefaultEngine 返回 default 名称的 xorm 引擎。
func (d *Databases) DefaultEngine() (*xorm.Engine, error) {
	return d.Engine(defaultName)
}

// Dialect 返回当前插件使用的数据库方言。
func (d *Databases) Dialect() Dialect {
	if d == nil {
		return ""
	}
	return d.dialect
}

// Name 返回插件名。
func (d *Databases) Name() string { return "databases" }

// Start 把默认引擎注册到 with 包，然后阻塞到 ctx 结束。
func (d *Databases) Start(ctx context.Context) error {
	if engine, err := d.DefaultEngine(); err == nil {
		with.SetDefault(engine)
	}

	<-ctx.Done()
	return nil
}

// Ready 对所有已打开的数据库做一次 Ping。
func (d *Databases) Ready(ctx context.Context) error {
	return errors.Join(d.healthy(ctx)...)
}

// Close 释放所有连接池。
func (d *Databases) Close(context.Context) error {
	if d == nil {
		return nil
	}
	return d.close()
}

// ProvideDefaultEngine 提供 default 实例的 xorm 引擎。
func ProvideDefaultEngine(d *Databases) (*xorm.Engine, error) {
	return d.DefaultEngine()
}

// parseDSN splits "<dialect>:<dsn>" into schema and the remaining DSN.
func parseDSN(raw string) (schema string, rest string, err error) {
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
		case '0' <= c && c <= '9' || c == '+' || c == '-' || c == '.':
			if i == 0 {
				return "", raw, nil
			}
		case c == ':':
			if i == 0 {
				return "", "", errors.New("missing protocol scheme")
			}
			if strings.HasPrefix(raw[i+1:], "//") {
				return raw[:i], raw[i+3:], nil
			}
			return raw[:i], raw[i+1:], nil
		default:
			return "", raw, nil
		}
	}
	return "", raw, nil
}
