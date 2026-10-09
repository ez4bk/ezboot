package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// _DefaultHealthyEndpoint 默认的健康检查端点地址
	_DefaultHealthyEndpoint = "/_healthy"
)

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
