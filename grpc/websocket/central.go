package websocket

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/metadata"
)

type reactor struct {
	seq   uint64
	conns sync.Map
}

// GetConn 从当前对象中获取连接对象
func (r *reactor) GetConn(id uint64) (*Conn, bool) {
	if conn, ok := r.conns.Load(id); ok {
		return conn.(*Conn), true
	}
	return nil, false
}

// GetConnFromContext 从上下文对象中获取连接对象
func (r *reactor) GetConnFromContext(ctx context.Context) (*Conn, bool) {
	if id, ok := GetConnId(ctx); ok {
		return r.GetConn(id)
	}
	return nil, false
}

// Detach 从当前对象中删除连接对象
func (r *reactor) Detach(conn *Conn) bool {
	if id, ok := GetConnId(conn.Context()); ok {
		r.conns.Delete(id)
	}
	return false
}

// ctxConnIdKey 在上下文对象中持有连接的Id
type ctxConnIdKey struct{}

// Attach 将连接放入其中进行集中管理
func (r *reactor) Attach(conn *Conn) uint64 {
	connId := atomic.AddUint64(&r.seq, 1)
	conn.SetContext(context.WithValue(conn.Context(), ctxConnIdKey{}, connId))
	r.conns.Store(connId, conn)
	return connId
}

// GetConnId 从上下文对象中获取连接的Id
func GetConnId(ctx context.Context) (uint64, bool) {
	if raw := ctx.Value(ctxConnIdKey{}); raw != nil {
		id, ok := raw.(uint64)
		return id, ok
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if connIds := md.Get(_ConnIdMetadataKey); len(connIds) != 0 {
			if connId, err := strconv.ParseInt(connIds[0], 10, 64); err == nil {
				return uint64(connId), true
			}
		}
	}

	if conn, ok := ConnFromContext(ctx); ok {
		return GetConnId(conn.Context())
	}

	return 0, false
}

// globalReactor 全局唯一的连接管理中心
var globalReactor = new(reactor)

// GetConn 从全局对象中获取连接对象
func GetConn(id uint64) (*Conn, bool) {
	return globalReactor.GetConn(id)
}
