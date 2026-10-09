package with

import (
	"context"

	"github.com/go-redis/redis"
	"go.uber.org/multierr"
	"xorm.io/xorm"

	boot "github.com/ez4bk/ezboot/internal/gin-boot"
)

// dbSession 用于在上下文对象中记录数据库会话
type dbSession struct{}

var (
	// dbSessionKey 用于在上下文对象中记录数据库会话
	dbSessionKey = dbSession{}
)

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

	_, err := boot.MW.DefaultTransaction(ctx, func(db *xorm.Session) (interface{}, error) {
		return nil, action(KeepSession(ctx, db))
	})
	return err
}

// DefaultSession 从上下文对象中读取已存在的持久化会话
func DefaultSession(ctx context.Context, action func(db *xorm.Session) error) (err error) {
	if session, ok := ctx.Value(dbSessionKey).(*xorm.Session); ok {
		return action(session)
	}

	session := boot.MW.DefaultSession(ctx)
	defer func() { err = multierr.Append(err, session.Close()) }()
	return action(session)
}

// DefaultRedis 从上下文对象中创建一个 redis 会话
func DefaultRedis(ctx context.Context, action func(rdb *redis.Client) error) error {
	c := boot.MW.DefaultRedis().WithContext(ctx)
	// do not close the client
	return action(c)
}
