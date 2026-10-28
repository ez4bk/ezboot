package middleware

import (
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/env"
)

var (
	otelOnce sync.Once
	otelTP   *tracesdk.TracerProvider
	otelErr  error
)

// InitOTEL configures the global OpenTelemetry tracer provider and propagator,
// then registers the gRPC tracing interceptors. It returns nil when tracing
// is disabled.
//
// It is idempotent: the process-level provider is created once, and later
// calls return the same instance so a plugin can own its shutdown.
func InitOTEL(cfg *config.Config) (*tracesdk.TracerProvider, error) {
	otelOnce.Do(func() {
		otelTP, otelErr = initOTEL(cfg)
	})

	return otelTP, otelErr
}

func initOTEL(cfg *config.Config) (*tracesdk.TracerProvider, error) {
	if cfg == nil || !cfg.App.Middleware.Tracing.Startup {
		return nil, nil
	}

	je, err := jaeger.New(
		jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint(cfg.App.Middleware.Tracing.Jaeger.Endpoint),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(je),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.App.Name),
			attribute.String("environment", env.ModeName(env.Env.Mode)),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	RegisterTracingInterceptors(cfg)

	return tp, nil
}
