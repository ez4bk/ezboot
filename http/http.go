package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"

	_cfg "github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/env"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware/tracing/otelgin"
	"github.com/ez4bk/ezboot/internal/logging"
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

// NewEngine 创建带默认中间件的 gin 引擎。pprof 与调用方自定义的路由
// 都会挂到这个引擎上。
func NewEngine(cfg *_cfg.Config) *gin.Engine {
	if env.Env != nil && cfg != nil {
		gin.SetMode(env.GinMode(env.Env.Mode))
	}

	engine := gin.New()
	engine.Use(
		otelgin.Middleware(cfg.App.Name,
			otelgin.WithEnabled(cfg.App.Middleware.Tracing.Startup),
			otelgin.WithExcludes(append(cfg.App.Middleware.Tracing.Http.Excludes, "/metrics", "/debug")...)),
		middleware.GinLogger(append(cfg.App.Middleware.HTTP.Logger.Excludes, "/debug/pprof/cmdline", "/metrics")...),
		logging.GinRecovery(),
		sentrygin.New(sentrygin.Options{Repanic: true}),
	)

	return engine
}

// Server 是一个HTTP插件。它实现 ezboot.Plugin：
//
// Start 绑定端口并开始服务（阻塞），Ready 报告端口已绑定，
// Drain 优雅关闭在途请求，Close 无事可做。
type Server struct {
	cfg    *_cfg.Config
	engine *gin.Engine
	// pprofAddr 非空时表示 pprof 使用独立端口
	pprofAddr string

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	pprofLn  net.Listener
	pprofSrv *http.Server
}

// Engine 返回所使用的底层Http服务
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Name 返回插件名。
func (s *Server) Name() string { return "http" }

// Start 绑定端口并开始服务，直到服务停止。
func (s *Server) Start(ctx context.Context) error {
	addr := net.JoinHostPort(s.cfg.App.Host, strconv.Itoa(s.cfg.App.Port))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	hs := &http.Server{Handler: s.engine}

	s.mu.Lock()
	s.listener = ln
	s.server = hs
	s.mu.Unlock()

	errCh := make(chan error, 2)
	go func() { errCh <- hs.Serve(ln) }()

	if s.pprofAddr != "" {
		pln, err := net.Listen("tcp", s.pprofAddr)
		if err != nil {
			_ = hs.Close()
			return err
		}

		ph := gin.New()
		pprof.Register(ph)

		ps := &http.Server{Handler: ph}

		s.mu.Lock()
		s.pprofLn = pln
		s.pprofSrv = ps
		s.mu.Unlock()

		go func() { errCh <- ps.Serve(pln) }()
	}

	err = <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Ready 报告业务端口是否已经绑定。
func (s *Server) Ready(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return errors.New("http: listener is not bound")
	}
	return nil
}

// Drain 停止接收新请求并等待在途请求结束。
func (s *Server) Drain(ctx context.Context) error {
	s.mu.Lock()
	hs, ps := s.server, s.pprofSrv
	s.mu.Unlock()

	var errs []error
	if ps != nil {
		errs = append(errs, ps.Shutdown(ctx))
	}
	if hs != nil {
		errs = append(errs, hs.Shutdown(ctx))
	}

	return errors.Join(errs...)
}

// Close 无事可做：连接已经由 Drain 关闭。
func (s *Server) Close(context.Context) error { return nil }

// ServeHTTP 让 Server 可以当作 http.Handler 使用。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}

// ServerOption 配置HTTP服务的额外参数
type ServerOption func(s *Server) error

// WithEngine 替换默认创建的 gin 引擎。
func WithEngine(engine *gin.Engine) ServerOption {
	return func(s *Server) error {
		if engine == nil {
			return errors.New("http: engine is nil")
		}
		s.engine = engine
		return nil
	}
}

// WithAdditionHandler 增加额外的处理方法到Http服务中
func WithAdditionHandler(method Method, pattern string, h http.Handler) ServerOption {
	return func(s *Server) error {
		switch method {
		case MethodAny:
			s.engine.Any(pattern, gin.WrapH(h))
		case MethodGet, MethodHead, MethodPost, MethodPut, MethodPatch, MethodDelete,
			MethodConnect, MethodOptions, MethodTrace:

			s.engine.Handle(string(method), pattern, gin.WrapH(h))
		}
		return nil
	}
}

// WithAdditionGinHandler 增加额外的处理方法到Http服务中
func WithAdditionGinHandler(method Method, pattern string, h gin.HandlerFunc) ServerOption {
	return func(s *Server) error {
		switch method {
		case MethodAny:
			s.engine.Any(pattern, h)
		case MethodGet, MethodHead, MethodPost, MethodPut, MethodPatch, MethodDelete,
			MethodConnect, MethodOptions, MethodTrace:

			s.engine.Handle(string(method), pattern, h)
		}
		return nil
	}
}

// Middleware 基于Gin的Http中间件函数签名
type Middleware = gin.HandlerFunc

// WithMiddleware 为Http请求配置中间件
func WithMiddleware(middlewares ...Middleware) ServerOption {
	return func(s *Server) error {
		s.engine.Use(middlewares...)
		return nil
	}
}

// WithExcludeLogger 为Http请求配置
func WithExcludeLogger(paths ...string) ServerOption {
	return func(s *Server) error { return nil }
}

// New 创建一个HTTP服务对象。它只做构造期的工作（路由、中间件、pprof
// 注册），不监听端口、不发起 I/O。
func New(cfg *_cfg.Config, options ...ServerOption) (*Server, error) {
	s := &Server{cfg: cfg, engine: NewEngine(cfg)}

	for _, option := range options {
		if err := option(s); err != nil {
			return nil, err
		}
	}

	s.setupPprof()

	return s, nil
}

// setupPprof 决定 pprof 挂在业务端口还是独立端口。
func (s *Server) setupPprof() {
	business := s.cfg.App.PprofHost == s.cfg.App.Host && s.cfg.App.PprofPort == s.cfg.App.Port
	empty := s.cfg.App.PprofHost == "" && s.cfg.App.PprofPort == 0

	if business || empty {
		pprof.Register(s.engine)
		return
	}

	s.pprofAddr = net.JoinHostPort(s.cfg.App.PprofHost, strconv.Itoa(s.cfg.App.PprofPort))
}
