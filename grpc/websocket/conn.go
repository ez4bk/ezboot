package websocket

import (
	"context"
	"sync"

	"nhooyr.io/websocket"
)

// Conn 表示一个底层的 Websocket 连接
type Conn struct {
	ctx context.Context

	// 连接关闭事件, 会在底层连接被关闭时自动触发
	closeFunc []func()
	closeOnce sync.Once
	closed    bool

	underlying *websocket.Conn
}

// OnClose 注册连接关闭时的事件
func (c *Conn) OnClose(handler func()) {
	c.closeFunc = append(c.closeFunc, handler)
}

// SetContext 为连接中设置一个上下文对象
func (c *Conn) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// Context 获取连接中的上下文对象
func (c *Conn) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Close 连接关闭事件
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.closed = true
		err = c.underlying.Close(websocket.StatusNormalClosure, "stateless")
		if c.closeFunc != nil {
			go func() {
				for _, onClose := range c.closeFunc {
					onClose()
				}
			}()
		}
	})
	return err
}

// Read 从连接中读取一条消息, 默认为文本消息
func (c *Conn) Read(ctx context.Context) ([]byte, error) {
	_, data, err := c.underlying.Read(ctx)
	return data, c.errWraps(err)
}

// Write 将一条消息写入到连接中, 以文本消息方式写入
func (c *Conn) Write(ctx context.Context, data []byte) error {
	return c.errWraps(c.underlying.Write(ctx, websocket.MessageText, data))
}

// errWraps 处理连接过程中出现的各种错误, 如果是连接已关闭的异常
// 就直接调用 Close 方法触发事件通知
func (c *Conn) errWraps(err error) error {
	if status := websocket.CloseStatus(err); status != -1 {
		_ = c.Close()
	}
	return err
}

type ctxConnKey struct{}

// WithConnContext 创建一个上下文对象
func (c *Conn) WithConnContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxConnKey{}, c)
}

// ConnFromContext 从上下文对象中获取连接对象
func ConnFromContext(ctx context.Context) (*Conn, bool) {
	if raw := ctx.Value((ctxConnKey{})); raw != nil {
		conn, ok := raw.(*Conn)
		return conn, ok
	}
	return nil, false
}
