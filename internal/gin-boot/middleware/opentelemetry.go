package middleware

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"google.golang.org/grpc"

	"github.com/ez4bk/ezboot/internal/gin-boot/env"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware/tracing/otelxgrpc"
)

func (mw *Middleware) initOTEL() {
	if mw.conf.App.Middleware.Tracing.Startup {
		je, err := jaeger.New(
			jaeger.WithCollectorEndpoint(
				jaeger.WithEndpoint(mw.conf.App.Middleware.Tracing.Jaeger.Endpoint),
			),
		)

		if err != nil {
			panic(err)
		}

		mw.tracerProvider = tracesdk.NewTracerProvider(
			tracesdk.WithBatcher(je),
			tracesdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(mw.conf.App.Name),
				attribute.String("environment", env.ModeName(env.Env.Mode)),
			)),
		)

		otel.SetTracerProvider(mw.tracerProvider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		))

		var unaryInterceptors []grpc.UnaryServerInterceptor
		if mw.conf.App.Middleware.Tracing.Details.Enabled {
			unaryInterceptors = append(unaryInterceptors,
				otelxgrpc.UnaryLogger(),
				otelxgrpc.UnaryDumper(
					mw.conf.App.Middleware.Tracing.Details.Request,
					mw.conf.App.Middleware.Tracing.Details.Response,
				),
			)
		}

		AddGrpcUnaryInterceptorsFirst(unaryInterceptors...)
		AddGrpcStreamInterceptorsFirst()
	}
}
