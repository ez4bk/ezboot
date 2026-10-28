package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// _DefaultHealthyEndpoint 默认的健康检查端点地址
	_DefaultHealthyEndpoint = "/_healthy"
	// _DefaultReadyEndpoint 默认的就绪检查端点地址
	_DefaultReadyEndpoint = "/readyz"
)

// ReadyFunc 判断当前实例是否可以接收流量。
type ReadyFunc func(ctx context.Context) error

// ReadyChecker 是 ezboot.State 的结构化接口。这样 http 包不需要
// import 根包，也就不会形成循环依赖。
type ReadyChecker interface {
	Ready() error
}

// WithReadyChecker 用 ezboot.State 作为就绪判断来源。
func WithReadyChecker(rc ReadyChecker) ServerOption {
	return func(s *Server) error {
		if rc == nil {
			return errors.New("http: ready checker is nil")
		}
		return WithReady(func(context.Context) error { return rc.Ready() })(s)
	}
}

// healthyOK 健康检查的结果
var healthyOK struct {
	Status string `json:"status"`
}

func init() {
	healthyOK.Status = "OK"
}

// WithHealthy 添加健康检查内容
func WithHealthy(endpoint ...string) ServerOption {
	if len(endpoint) == 0 {
		endpoint = append(endpoint, _DefaultHealthyEndpoint)
	}

	return func(s *Server) error {
		// make the healthy endpoint not logged
		s.cfg.App.Middleware.HTTP.Logger.Excludes = append(
			s.cfg.App.Middleware.HTTP.Logger.Excludes, endpoint...)

		s.Engine().Handle(http.MethodGet, endpoint[0], func(c *gin.Context) {
			c.JSON(http.StatusOK, healthyOK)
		})
		return nil
	}
}

// WithReady 添加就绪检查端点。依赖未配置时由 ReadyFunc 自行跳过，配置了
// 但不可用时返回 503。
func WithReady(ready ReadyFunc, endpoint ...string) ServerOption {
	if len(endpoint) == 0 {
		endpoint = append(endpoint, _DefaultReadyEndpoint)
	}

	return func(s *Server) error {
		s.Engine().Handle(http.MethodGet, endpoint[0], func(c *gin.Context) {
			if ready == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
				return
			}
			if err := ready(c.Request.Context()); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status": "not ready",
					"error":  err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, healthyOK)
		})
		return nil
	}
}
