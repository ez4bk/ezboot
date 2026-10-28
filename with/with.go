package with

import (
	"context"
	"errors"
	"sync"

	"github.com/go-redis/redis"
	"go.uber.org/multierr"
	"xorm.io/xorm"
)

var (
	// ErrDatabaseUnavailable 表示默认数据库尚未注册，通常是 MySQL 插件
	// 还没启动完成。
	ErrDatabaseUnavailable = errors.New("with: database is not available")
	// ErrRedisUnavailable 表示默认 Redis 尚未注册，通常是 Redis 插件还没
	// 启动完成。
	ErrRedisUnavailable = errors.New("with: redis is not available")
)

// dbSession 用于在上下文对象中记录数据库会话
type dbSession struct{}

var (
	// dbSessionKey 用于在上下文对象中记录数据库会话
	dbSessionKey = dbSession{}

	mu            sync.RWMutex
	defaultEngine *xorm.Engine
	defaultRedis  *redis.Client
)

// SetDefault 注册默认数据库引擎。由 infra.MySQL 插件在 Start 阶段调用。
func SetDefault(engine *xorm.Engine) {
	mu.Lock()
	defaultEngine = engine
	mu.Unlock()
}

// SetDefaultRedis 注册默认 Redis 客户端。由 infra.Redis 插件在 Start
// 阶段调用。
func SetDefaultRedis(client *redis.Client) {
	mu.Lock()
	defaultRedis = client
	mu.Unlock()
}

func engine() *xorm.Engine {
	mu.RLock()
	defer mu.RUnlock()
	return defaultEngine
}

func redisClient() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()
	return defaultRedis
}

// KeepSession 将数据库会话持久化到上下文对象中
func KeepSession(ctx context.Context, db *xorm.Session) context.Context {
	return context.WithValue(ctx, dbSessionKey, db)
}

// DefaultTransaction 从上下文对象中读取已存在的持久化会话
func DefaultTransaction(ctx context.Context, action func(context.Context) error) error {
	if v := ctx.Value(dbSessionKey); v != nil {
		if sess, ok := v.(*xorm.Session); ok && sess != nil {
			return action(ctx)
		}
	}

	e := engine()
	if e == nil {
		return ErrDatabaseUnavailable
	}

	_, err := e.Transaction(func(db *xorm.Session) (interface{}, error) {
		return nil, action(KeepSession(ctx, db.Context(ctx)))
	})
	return err
}

// DefaultSession 从上下文对象中读取已存在的持久化会话
func DefaultSession(ctx context.Context, action func(db *xorm.Session) error) (err error) {
	if session, ok := ctx.Value(dbSessionKey).(*xorm.Session); ok {
		return action(session)
	}

	e := engine()
	if e == nil {
		return ErrDatabaseUnavailable
	}

	session := e.Context(ctx)
	defer func() { err = multierr.Append(err, session.Close()) }()
	return action(session)
}

// DefaultRedis 从上下文对象中创建一个 redis 会话
func DefaultRedis(ctx context.Context, action func(rdb *redis.Client) error) error {
	c := redisClient()
	if c == nil {
		return ErrRedisUnavailable
	}

	return action(c.WithContext(ctx))
}
