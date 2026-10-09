package grpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/ez4bk/ezboot/grpc/gateway"
	_cfg "github.com/ez4bk/ezboot/internal/gin-boot/config"
	_grpc "github.com/ez4bk/ezboot/internal/gin-boot/grpc-boot"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware"
)

// Server 是一个GRPC服务对象
type Server struct {
	gs *_grpc.ServerType
	gh *gateway.Handler
	rs map[interface{}]interface{}
	l  net.Listener

	hh http.Handler
}

// Start 启动GRPC服务
func (s *Server) Start(ctx context.Context) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if ex, ok := v.(error); ok {
				err = multierr.Append(err, ex)
			} else {
				err = multierr.Append(err, errors.Errorf("unknown error: %v", v))
			}
		}
	}()

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
		if s.hh, err = s.gh.Setup(ctx); err != nil {
			return err
		}
	}

	go func() {
		<-ctx.Done()
		s.gs.Driver().GracefulStop()
	}()

	reflection.Register(s.gs.Driver())
	return s.gs.Driver().Serve(s.l)
}

// ServeHTTP 将HTTP请求转发到网关中进行处理
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.hh != nil {
		s.hh.ServeHTTP(w, r)
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

// New 创建一个GRPC服务
func New(cfg *_cfg.Config, options ...ServerOption) (s *Server, err error) {
	l, err := net.Listen("tcp", cfg.App.Middleware.GRPC.Server.Default.Addr)
	if err != nil {
		return nil, err
	}

	s = &Server{l: l, rs: make(map[interface{}]interface{})}
	for _, option := range options {
		if err = option(s); err != nil {
			return nil, err
		}
	}

	s.gs = _grpc.Create(cfg.App.Name, cfg.App.Name)
	return s, err
}
