package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-pg/pg/v10"
	_ "github.com/go-sql-driver/mysql"
	"xorm.io/xorm"

	"github.com/google/wire"

	"github.com/ez4bk/ezboot"
	"github.com/ez4bk/ezboot/with"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware/tracing/otelxorm"
	"github.com/ez4bk/ezboot/internal/logging"
)

// DatabaseType selects which database family DefaultSession/DB use.
type DatabaseType int

const (
	DatabaseMySQL DatabaseType = iota
	DatabasePostgres
	DatabaseSQLite
)

// DB is a typed handle to a configured database engine.
//
// The handle owns exactly one connection pool (the xorm engine's pool). Use
// Engine for xorm operations and SQL for the underlying *sql.DB when raw SQL
// is needed; both refer to the same pool.
type DB struct {
	name    string
	dialect DatabaseType
	engine  *xorm.Engine
}

func (db *DB) Name() string {
	if db == nil {
		return ""
	}
	return db.name
}

func (db *DB) Dialect() DatabaseType {
	if db == nil {
		return DatabaseMySQL
	}
	return db.dialect
}

func (db *DB) Engine() *xorm.Engine {
	if db == nil {
		return nil
	}
	return db.engine
}

// SQL returns the pool owned by the xorm engine. It is nil only when the
// handle is nil or the engine was closed.
func (db *DB) SQL() *sql.DB {
	if db == nil || db.engine == nil {
		return nil
	}
	return db.engine.DB().DB
}

func (db *DB) Close() error {
	if db == nil || db.engine == nil {
		return nil
	}
	return db.engine.Close()
}

type MySQL struct {
	ezboot.PluginBase

	mu sync.RWMutex

	defaultType DatabaseType
	mysqls      map[string]*DB
	sqlites     map[string]*DB
	pgsqls      map[string]*pg.DB
}

func (d *MySQL) open(cfg *config.Config, logger *logging.Logger) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.mysqls = make(map[string]*DB)
	d.sqlites = make(map[string]*DB)
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

		d.mysqls[name] = &DB{name: name, dialect: DatabaseMySQL, engine: engine}
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

		d.sqlites[name] = &DB{name: name, dialect: DatabaseSQLite, engine: engine}
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

func (d *MySQL) setDefault(t DatabaseType) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.defaultType = t
}

func (d *MySQL) get(name string, dialect DatabaseType) (*DB, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var (
		db  *DB
		ok  bool
		all map[string]*DB
	)
	switch dialect {
	case DatabaseMySQL:
		all = d.mysqls
	case DatabaseSQLite:
		all = d.sqlites
	default:
		return nil, resourceError("db", name, fmt.Errorf("unsupported database type %d", dialect))
	}
	db, ok = all[name]
	if !ok {
		return nil, resourceError("db", name, nil)
	}
	return db, nil
}

func (d *MySQL) db(name string) (*DB, error) {
	d.mu.RLock()
	dialect := d.defaultType
	d.mu.RUnlock()

	return d.get(name, dialect)
}

func (d *MySQL) pgsql(name string) (*pg.DB, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	db, ok := d.pgsqls[name]
	if !ok {
		return nil, resourceError("pgsql", name, nil)
	}
	return db, nil
}

func (d *MySQL) engine(name string) (*xorm.Engine, error) {
	db, err := d.db(name)
	if err != nil {
		return nil, err
	}
	return db.engine, nil
}

func (d *MySQL) session(ctx context.Context, name string) (*xorm.Session, error) {
	engine, err := d.engine(name)
	if err != nil {
		return nil, err
	}
	return engine.Context(ctx), nil
}

func (d *MySQL) transaction(ctx context.Context, name string, f func(*xorm.Session) (any, error)) (any, error) {
	engine, err := d.engine(name)
	if err != nil {
		return nil, err
	}
	return engine.Transaction(func(s *xorm.Session) (any, error) {
		return f(s.Context(ctx))
	})
}

func (d *MySQL) close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var errs []error
	for name, db := range d.mysqls {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close mysql %q: %w", name, err))
		}
	}
	for name, db := range d.sqlites {
		if err := db.Close(); err != nil {
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

func (d *MySQL) healthy(ctx context.Context) []error {
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
	for name, db := range d.mysqls {
		mysqls = append(mysqls, namedEngine{name: name, engine: db.engine})
	}
	sqlites := make([]namedEngine, 0, len(d.sqlites))
	for name, db := range d.sqlites {
		sqlites = append(sqlites, namedEngine{name: name, engine: db.engine})
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

// DB returns the configured database handle selected by UseDefaultDatabase.
func (s *MySQL) DB(name string) (*DB, error) {
	if s == nil {
		return nil, resourceError("db", name, nil)
	}
	return s.db(name)
}

// DefaultDB returns the default database handle.
func (s *MySQL) DefaultDB() (*DB, error) {
	return s.DB(defaultName)
}

// MySQL returns a configured MySQL handle by name.
func (s *MySQL) MySQL(name string) (*DB, error) {
	if s == nil {
		return nil, resourceError("mysql", name, nil)
	}
	return s.get(name, DatabaseMySQL)
}

// SQLite returns a configured SQLite handle by name.
func (s *MySQL) SQLite(name string) (*DB, error) {
	if s == nil {
		return nil, resourceError("sqlite", name, nil)
	}
	return s.get(name, DatabaseSQLite)
}

// Pgsql returns a configured PostgreSQL client by name.
func (s *MySQL) Pgsql(name string) (*pg.DB, error) {
	if s == nil {
		return nil, resourceError("pgsql", name, nil)
	}
	return s.pgsql(name)
}

// DefaultPgsql returns the default PostgreSQL client.
func (s *MySQL) DefaultPgsql() (*pg.DB, error) {
	return s.Pgsql(defaultName)
}

// ORMEngine returns the xorm engine selected by UseDefaultDatabase.
func (s *MySQL) ORMEngine(name string) (*xorm.Engine, error) {
	if s == nil {
		return nil, resourceError("db", name, nil)
	}
	return s.engine(name)
}

// DefaultORMEngine returns the default xorm engine.
func (s *MySQL) DefaultORMEngine() (*xorm.Engine, error) {
	return s.ORMEngine(defaultName)
}

// SqliteORMEngine returns the default SQLite xorm engine.
func (s *MySQL) SqliteORMEngine() (*xorm.Engine, error) {
	db, err := s.SQLite(defaultName)
	if err != nil {
		return nil, err
	}
	return db.Engine(), nil
}

// Session starts a session for the selected database.
func (s *MySQL) Session(ctx context.Context, name string) (*xorm.Session, error) {
	if s == nil {
		return nil, resourceError("db", name, nil)
	}
	return s.session(ctx, name)
}

// DefaultSession starts a session for the default database.
//
// It returns nil when the default database is not configured. The
// error-returning variant is Session(ctx, defaultName).
func (s *MySQL) DefaultSession(ctx context.Context) *xorm.Session {
	session, err := s.Session(ctx, defaultName)
	if err != nil {
		return nil
	}
	return session
}

// Transaction runs f in a transaction on the selected database.
func (s *MySQL) Transaction(
	ctx context.Context,
	name string,
	f func(*xorm.Session) (any, error),
) (any, error) {
	if s == nil {
		return nil, resourceError("db", name, nil)
	}
	return s.transaction(ctx, name, f)
}

// DefaultTransaction runs f in a transaction on the default database.
func (s *MySQL) DefaultTransaction(
	ctx context.Context,
	f func(*xorm.Session) (any, error),
) (any, error) {
	return s.Transaction(ctx, defaultName, f)
}

// UseDefaultDatabase selects the database family used by DB/Session/Transaction.
func (s *MySQL) UseDefaultDatabase(dbt DatabaseType) {
	if s != nil {
		s.setDefault(dbt)
	}
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

// MySQL 是数据库插件。一个插件管理配置里所有 MySQL / SQLite / Pgsql
// 实例，与 app.middleware.mysql 等配置段一一对应。
//
// 连接池在构造期建立（不产生网络 I/O），Start 只注册默认引擎并阻塞。
var MySQLProviderSet = wire.NewSet(NewMySQL, ProvideDefaultMySQL)

// NewMySQL 创建插件并建立连接池。
func NewMySQL(cfg *config.Config, logger *logging.Logger) (*MySQL, error) {
	if cfg == nil {
		return nil, errors.New("infra: mysql requires a config")
	}
	if len(cfg.App.Middleware.Mysql)+len(cfg.App.Middleware.Sqlite)+len(cfg.App.Middleware.Pgsql) == 0 {
		return nil, errors.New("infra: mysql plugin is enabled but no database is configured")
	}

	m := &MySQL{}
	if err := m.open(cfg, loggerOrDefault(logger)); err != nil {
		return nil, err
	}

	return m, nil
}

// Name 返回插件名。
func (m *MySQL) Name() string { return "mysql" }

// Start 注册默认数据库并阻塞到 ctx 结束。
func (m *MySQL) Start(ctx context.Context) error {
	if engine, err := m.engine(defaultName); err == nil {
		with.SetDefault(engine)
	}

	<-ctx.Done()
	return nil
}

// Ready 对已打开的数据库做一次 Ping。
func (m *MySQL) Ready(ctx context.Context) error {
	return errors.Join(m.healthy(ctx)...)
}

// Close 释放所有连接池。
func (m *MySQL) Close(context.Context) error {
	if m == nil {
		return nil
	}
	return m.close()
}

// ProvideDefaultMySQL 提供 default 实例的 xorm 引擎。
func ProvideDefaultMySQL(m *MySQL) (*xorm.Engine, error) {
	return m.DefaultORMEngine()
}
