package websocket

import (
	"context"

	"github.com/ez4bk/ezboot/exp"
	"github.com/ez4bk/ezboot/xjson"
)

// ConnGroup 对连接进行分组
type ConnGroup[G any] struct {
	ms exp.LazyMapSet[G, uint64]
}

// Attach 记录每个Id对应的连接集合
func (g *ConnGroup[G]) Attach(gid G, connId uint64) {
	g.ms.Put(gid, connId)
}

// Detach 删除Id对应的连接
func (g *ConnGroup[G]) Detach(gid G, connId uint64) {
	g.ms.Remove(gid, connId)
}

// Range 遍历Id对应的连接, walk 函数的返回值表示是否继续遍历
func (g *ConnGroup[G]) Range(gid G, walk func(connId uint64) bool) {
	g.ms.RangeSet(gid, walk)
}

// Broadcast 对组内的所有连接广播一条消息
func (g *ConnGroup[G]) Broadcast(ctx context.Context, gid G, id *string, kind string, msg any) (err error) {
	g.Range(gid, func(connId uint64) bool {
		if conn, ok := GetConn(connId); ok {
			_ = conn.Write(ctx, xjson.MustMarshal(&Message{
				Id:   id,
				Kind: kind,
				Data: msg,
			}))
		}
		return true
	})
	return
}

// NewGroup 创建一个连接组
func NewGroup[K any]() *ConnGroup[K] { return &ConnGroup[K]{} }
