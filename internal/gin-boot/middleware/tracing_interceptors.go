package middleware

import (
	"google.golang.org/grpc"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware/tracing/otelxgrpc"
)

// RegisterTracingInterceptors adds the OpenTelemetry gRPC interceptors.
// It must be called after InitOTEL has installed the global
// tracer provider.
func RegisterTracingInterceptors(cfg *config.Config) {
	if cfg == nil || !cfg.App.Middleware.Tracing.Startup {
		return
	}

	var unary []grpc.UnaryServerInterceptor
	if cfg.App.Middleware.Tracing.Details.Enabled {
		unary = append(unary,
			otelxgrpc.UnaryLogger(),
			otelxgrpc.UnaryDumper(
				cfg.App.Middleware.Tracing.Details.Request,
				cfg.App.Middleware.Tracing.Details.Response,
			),
		)
	}

	AddGrpcUnaryInterceptorsFirst(unary...)
	AddGrpcStreamInterceptorsFirst()
}
