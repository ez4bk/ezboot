package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-pg/pg/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	"xorm.io/xorm"

	"github.com/ez4bk/ezboot"
	"github.com/ez4bk/ezboot/with"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware/tracing/otelxorm"
	"github.com/ez4bk/ezboot/internal/logging"
)

// Databases 是关系型数据库插件，管理配置里的 MySQL / SQLite / Postgres
// 实例。
//
// 三类数据库各有独立的默认 provider，没有“当前默认数据库”这种全局
// 选择：想要 MySQL 就注入 *xorm.Engine（ProvideDefaultMySQL），想要
// Postgres 就注入 *pg.DB（ProvideDefaultPostgres）。
type Databases struct {
	ezboot.PluginBase

	mu      sync.RWMutex
	mysqls  map[string]*xorm.Engine
	sqlites map[string]*xorm.Engine
	pgsqls  map[string]*pg.DB
}

// DatabasesProviderSet 只提供数据库插件本身。
var DatabasesProviderSet = wire.NewSet(NewDatabases)

// MySQLProviderSet / SQLiteProviderSet / PostgresProviderSet 提供对应
// 数据库 default 实例。只加你实际使用的那种。
//
// 注意：MySQL 与 SQLite 的默认实例都是 *xorm.Engine，wire 无法区分，
// 所以同一个 injector 里不能同时引入这两个 set。
var (
	MySQLProviderSet    = wire.NewSet(ProvideDefaultMySQL)
	SQLiteProviderSet   = wire.NewSet(ProvideDefaultSQLite)
	PostgresProviderSet = wire.NewSet(ProvideDefaultPostgres)
)

// NewDatabases 创建插件并建立连接池。连接池在第一次查询时才真正连接
// 数据库，所以这里不产生网络 I/O。
func NewDatabases(cfg *config.Config, logger *logging.Logger) (*Databases, error) {
	if cfg == nil {
		return nil, errors.New("infra: databases requires a config")
	}
	if len(cfg.App.Middleware.Mysql)+len(cfg.App.Middleware.Sqlite)+len(cfg.App.Middleware.Pgsql) == 0 {
		return nil, errors.New("infra: databases plugin is enabled but no database is configured")
	}

	d := &Databases{}
	if err := d.open(cfg, loggerOrDefault(logger)); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Databases) open(cfg *config.Config, logger *logging.Logger) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.mysqls = make(map[string]*xorm.Engine)
	d.sqlites = make(map[string]*xorm.Engine)
	d.pgsqls = make(map[string]*pg.DB)

	for name, c := range cfg.App.Middleware.Mysql {
		if !c.Startup {
			continue
		}

		engine, err := xorm.NewEngine("mysql", c.Dsn)
		if err != nil {
			return fmt.Errorf("open mysql %q: %w", name, err)
		}

		if c.MaxIdleConnection != 0 {
			engine.SetMaxIdleConns(c.MaxIdleConnection)
		}
		if c.MaxOpenConnection != 0 {
			engine.SetMaxOpenConns(c.MaxOpenConnection)
		}
		if c.MaxIdleTime != 0 {
			engine.SetConnMaxLifetime(c.MaxIdleTime)
			engine.DB().SetConnMaxIdleTime(c.MaxIdleTime)
		}

		engine.ShowSQL(!c.HiddenSQL)
		engine.SetLogger(NewXormLogger(logger, !c.HiddenSQL))

		if cfg.App.Middleware.Tracing.Database.Enabled {
			engine.AddHook(otelxorm.NewTracingHook(&cfg.App.Middleware.Tracing))
		}

		d.mysqls[name] = engine
	}

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

		if c.MaxIdleConnection != 0 {
			engine.SetMaxIdleConns(c.MaxIdleConnection)
		}
		if c.MaxOpenConnection != 0 {
			engine.SetMaxOpenConns(c.MaxOpenConnection)
		}

		engine.ShowSQL(!c.HiddenSQL)
		engine.SetLogger(NewXormLogger(logger, !c.HiddenSQL))

		d.sqlites[name] = engine
	}

	for name, c := range cfg.App.Middleware.Pgsql {
		if !c.Startup {
			continue
		}

		opt, err := pg.ParseURL(c.URL)
		if err != nil {
			return fmt.Errorf("open pgsql %q: %w", name, err)
		}
		d.pgsqls[name] = pg.Connect(opt)
	}

	return nil
}

func (d *Databases) close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var errs []error
	for name, engine := range d.mysqls {
		if err := engine.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close mysql %q: %w", name, err))
		}
	}
	for name, engine := range d.sqlites {
		if err := engine.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close sqlite %q: %w", name, err))
		}
	}
	for name, db := range d.pgsqls {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close pgsql %q: %w", name, err))
		}
	}

	d.mysqls = nil
	d.sqlites = nil
	d.pgsqls = nil

	return errors.Join(errs...)
}

func (d *Databases) healthy(ctx context.Context) []error {
	type namedEngine struct {
		name   string
		engine *xorm.Engine
	}
	type namedPgsql struct {
		name string
		db   *pg.DB
	}

	d.mu.RLock()
	mysqls := make([]namedEngine, 0, len(d.mysqls))
	for name, engine := range d.mysqls {
		mysqls = append(mysqls, namedEngine{name: name, engine: engine})
	}
	sqlites := make([]namedEngine, 0, len(d.sqlites))
	for name, engine := range d.sqlites {
		sqlites = append(sqlites, namedEngine{name: name, engine: engine})
	}
	pgsqls := make([]namedPgsql, 0, len(d.pgsqls))
	for name, db := range d.pgsqls {
		pgsqls = append(pgsqls, namedPgsql{name: name, db: db})
	}
	d.mu.RUnlock()

	var errs []error
	for _, item := range mysqls {
		if err := item.engine.PingContext(ctx); err != nil {
			errs = append(errs, fmt.Errorf("mysql %q: %w", item.name, err))
		}
	}
	for _, item := range sqlites {
		if err := item.engine.PingContext(ctx); err != nil {
			errs = append(errs, fmt.Errorf("sqlite %q: %w", item.name, err))
		}
	}
	for _, item := range pgsqls {
		if err := item.db.Ping(ctx); err != nil {
			errs = append(errs, fmt.Errorf("pgsql %q: %w", item.name, err))
		}
	}
	return errs
}

// MySQL 返回指定名称的 MySQL xorm 引擎。
func (d *Databases) MySQL(name string) (*xorm.Engine, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	engine, ok := d.mysqls[name]
	if !ok {
		return nil, resourceError("mysql", name, nil)
	}
	return engine, nil
}

// DefaultMySQL 返回 default 名称的 MySQL xorm 引擎。
func (d *Databases) DefaultMySQL() (*xorm.Engine, error) {
	return d.MySQL(defaultName)
}

// SQLite 返回指定名称的 SQLite xorm 引擎。
func (d *Databases) SQLite(name string) (*xorm.Engine, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	engine, ok := d.sqlites[name]
	if !ok {
		return nil, resourceError("sqlite", name, nil)
	}
	return engine, nil
}

// DefaultSQLite 返回 default 名称的 SQLite xorm 引擎。
func (d *Databases) DefaultSQLite() (*xorm.Engine, error) {
	return d.SQLite(defaultName)
}

// Postgres 返回指定名称的 go-pg 客户端。
func (d *Databases) Postgres(name string) (*pg.DB, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	db, ok := d.pgsqls[name]
	if !ok {
		return nil, resourceError("pgsql", name, nil)
	}
	return db, nil
}

// DefaultPostgres 返回 default 名称的 go-pg 客户端。
func (d *Databases) DefaultPostgres() (*pg.DB, error) {
	return d.Postgres(defaultName)
}

// Name 返回插件名。
func (d *Databases) Name() string { return "databases" }

// Start 把默认的 xorm 引擎注册到 with 包（优先 MySQL，其次 SQLite），
// 然后阻塞到 ctx 结束。
func (d *Databases) Start(ctx context.Context) error {
	if engine, err := d.DefaultMySQL(); err == nil {
		with.SetDefault(engine)
	} else if engine, err := d.DefaultSQLite(); err == nil {
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

// ProvideDefaultMySQL 提供 default 名称的 MySQL xorm 引擎。
func ProvideDefaultMySQL(d *Databases) (*xorm.Engine, error) {
	return d.DefaultMySQL()
}

// ProvideDefaultSQLite 提供 default 名称的 SQLite xorm 引擎。
func ProvideDefaultSQLite(d *Databases) (*xorm.Engine, error) {
	return d.DefaultSQLite()
}

// ProvideDefaultPostgres 提供 default 名称的 Postgres 客户端。
func ProvideDefaultPostgres(d *Databases) (*pg.DB, error) {
	return d.DefaultPostgres()
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
