package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"

	boot "github.com/ez4bk/ezboot/internal/gin-boot"
	_cfg "github.com/ez4bk/ezboot/internal/gin-boot/config"
)

// Method 注册HTTP路由时需要指定的HTTP方法
type Method string

const (
	MethodAny     Method = "ANY"
	MethodGet     Method = http.MethodGet
	MethodHead    Method = http.MethodHead
	MethodPost    Method = http.MethodPost
	MethodPut     Method = http.MethodPut
	MethodPatch   Method = http.MethodPatch
	MethodDelete  Method = http.MethodDelete
	MethodConnect Method = http.MethodConnect
	MethodOptions Method = http.MethodOptions
	MethodTrace   Method = http.MethodTrace
)

// Server 是一个HTTP的服务程序
type Server struct {
	cfg *_cfg.Config
	w   *boot.Server
}

// Engine 返回所使用的底层Http服务
func (s *Server) Engine() *gin.Engine {
	return s.w.Driver
}

// Start 启动HTTP服务
func (s *Server) Start(ctx context.Context) (err error) {
	hs := &http.Server{
		Addr:    s.cfg.App.Host + ":" + strconv.Itoa(s.cfg.App.Port),
		Handler: s.w.Driver,
	}

	go func() {
		<-ctx.Done()

		// waiting for grpc exiting
		_ = hs.Shutdown(context.Background())
	}()

	if err = hs.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

// ServerOption 配置HTTP服务的额外参数
type ServerOption func(s *Server) error

// WithAdditionHandler 增加额外的处理方法到Http服务中
func WithAdditionHandler(method Method, pattern string, h http.Handler) ServerOption {
	return func(s *Server) error {
		switch method {
		case MethodAny:
			s.w.Driver.Any(pattern, gin.WrapH(h))
		case MethodGet, MethodHead, MethodPost, MethodPut, MethodPatch, MethodDelete,
			MethodConnect, MethodOptions, MethodTrace:

			s.w.Driver.Handle(string(method), pattern, gin.WrapH(h))
		}
		return nil
	}
}

// WithAdditionGinHandler 增加额外的处理方法到Http服务中
func WithAdditionGinHandler(method Method, pattern string, h gin.HandlerFunc) ServerOption {
	return func(s *Server) error {
		switch method {
		case MethodAny:
			s.w.Driver.Any(pattern, h)
		case MethodGet, MethodHead, MethodPost, MethodPut, MethodPatch, MethodDelete,
			MethodConnect, MethodOptions, MethodTrace:

			s.w.Driver.Handle(string(method), pattern, h)
		}
		return nil
	}
}

// Middleware 基于Gin的Http中间件函数签名
type Middleware = gin.HandlerFunc

// WithMiddleware 为Http请求配置中间件
func WithMiddleware(middlewares ...Middleware) ServerOption {
	return func(s *Server) error {
		s.w.Driver.Use(middlewares...)
		return nil
	}
}

// WithExcludeLogger 为Http请求配置
func WithExcludeLogger(paths ...string) ServerOption {
	return func(s *Server) error { return nil }
}

// New 创建一个HTTP服务对象
func New(cfg *_cfg.Config, options ...ServerOption) (*Server, error) {
	s := &Server{cfg: cfg, w: boot.Default()}
	s.w.Driver.Use(sentrygin.New(sentrygin.Options{
		Repanic: true,
	}))

	for _, option := range options {
		if err := option(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}
