package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/ez4bk/ezboot/grpc/gateway"
	_cfg "github.com/ez4bk/ezboot/internal/gin-boot/config"
	_grpc "github.com/ez4bk/ezboot/internal/gin-boot/grpc_boot"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware"
)

// Server 是一个GRPC插件。它实现 ezboot.Plugin：
//
// Start 注册服务、绑定端口并开始 Serve（阻塞），Ready 报告端口已绑定，
// Drain 优雅停止（超时后强制 Stop），Close 无事可做。
type Server struct {
	cfg *_cfg.Config
	gs  *_grpc.ServerType
	gh  *gateway.Handler
	rs  map[interface{}]interface{}

	mu sync.Mutex
	l  net.Listener
	hh http.Handler
}

// Name 返回插件名。
func (s *Server) Name() string { return "grpc" }

// Start 注册服务、绑定端口并开始 Serve，直到服务停止。
func (s *Server) Start(ctx context.Context) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if ex, ok := v.(error); ok {
				err = ex
				return
			}
			err = fmt.Errorf("grpc: unknown error: %v", v)
		}
	}()

	ln, err := net.Listen("tcp", s.cfg.App.Middleware.GRPC.Server.Default.Addr)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.l = ln
	s.mu.Unlock()

	for svc, reg := range s.rs {
		rf := reflect.ValueOf(reg)
		if rf.Kind() != reflect.Func {
			return fmt.Errorf("invalid GRPC server passed, expected func, got %T", reg)
		}

		rf.Call([]reflect.Value{
			reflect.ValueOf(s.gs.Driver()),
			reflect.ValueOf(svc),
		})
	}

	if s.gh != nil {
		hh, err := s.gh.Setup(ctx)
		if err != nil {
			return err
		}

		s.mu.Lock()
		s.hh = hh
		s.mu.Unlock()
	}

	reflection.Register(s.gs.Driver())

	if err := s.gs.Driver().Serve(ln); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}

	return nil
}

// Ready 报告端口是否已经绑定。
func (s *Server) Ready(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.l == nil {
		return errors.New("grpc: listener is not bound")
	}
	if s.gh != nil && s.hh == nil {
		return errors.New("grpc: gateway is not ready")
	}
	return nil
}

// Drain 优雅停止：等待在途 RPC 结束，超时后强制停止。
func (s *Server) Drain(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		s.gs.Driver().GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.gs.Driver().Stop()
		return ctx.Err()
	}
}

// Close 无事可做：连接已经由 Drain 关闭。
func (s *Server) Close(context.Context) error { return nil }

// ServeHTTP 将HTTP请求转发到网关中进行处理
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	hh := s.hh
	s.mu.Unlock()

	if hh != nil {
		hh.ServeHTTP(w, r)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// ServerOption GRPC服务的额外参数配置
type ServerOption func(s *Server) error

// WithGatewayHandler 为GRPC增加网关功能
func WithGatewayHandler(gh *gateway.Handler) ServerOption {
	return func(s *Server) error {
		s.gh = gh
		return nil
	}
}

// WithGRPCServerRegister 注册GRPC服务
func WithGRPCServerRegister(register interface{}, svc interface{}) ServerOption {
	return func(s *Server) error {
		s.rs[svc] = register
		return nil
	}
}

// WithGRPCUnaryInterceptor 为服务器增加一个拦截器
func WithGRPCUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) ServerOption {
	return func(s *Server) error {
		middleware.AddGrpcUnaryInterceptors(interceptor)

		return nil
	}
}

// New 创建一个GRPC服务。它只构造 driver，不监听端口。
func New(cfg *_cfg.Config, options ...ServerOption) (s *Server, err error) {
	s = &Server{cfg: cfg, rs: make(map[interface{}]interface{})}

	for _, option := range options {
		if err = option(s); err != nil {
			return nil, err
		}
	}

	_grpc.SetConfig(&cfg.App.Middleware.GRPC.Server.Default)
	s.gs = _grpc.Create(cfg.App.Name, cfg.App.Name)

	return s, nil
}
