package websocket

import (
	"context"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/ez4bk/ezboot"
	"github.com/ez4bk/ezboot/grpc/gateway"
)

var (
	// ErrReadTimeout 读取数据超时
	ErrReadTimeout = errors.New("websocket: read timeout")
)

// Option 注册的额外参数
type Option func(cfg *config)

// WithKeepaliveTimeout 配置检测存活时间
func WithKeepaliveTimeout(keepalive time.Duration) Option {
	return func(cfg *config) {
		cfg.KeepaliveTimeout = keepalive
	}
}

// WithLogger 配置日志记录器
func WithLogger(logger *ezboot.Logger) Option {
	return func(cfg *config) {
		cfg.Logger = logger
	}
}

// FactoryFunc 客户端工厂方法
type FactoryFunc[T any] func(grpc.ClientConnInterface) T

// NewRegister 创建一个新的Websocket转gRPC的注册器
func NewRegister[T any](method string, uri string, factory FactoryFunc[T], regOpts ...Option) gateway.EndpointRegister {
	return func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
		conn, err := grpc.Dial(endpoint, opts...)
		if err != nil {
			return err
		}

		s := newServer()
		for _, opt := range regOpts {
			opt(s.cfg)
		}

		defer func() {
			if err != nil {
				if cErr := conn.Close(); cErr != nil {
					s.cfg.Logger.Infof("Failed to close conn to %s: %v", endpoint, cErr)
				}
				return
			}
			go func() {
				<-ctx.Done()
				if cErr := conn.Close(); cErr != nil {
					s.cfg.Logger.Infof("Failed to close conn to %s: %v", endpoint, cErr)
				}
			}()
		}()

		gc, err := newClient(factory, conn)
		if err != nil {
			return err
		}

		return mux.HandlePath(method, uri, s.genWebsocketHttpHandler(ctx, gc))
	}
}
