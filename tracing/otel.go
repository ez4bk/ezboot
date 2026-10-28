// Package tracing 把 OpenTelemetry 的 tracer provider 暴露成一个插件。
//
// 初始化必须早于任何 gRPC server 的构造（拦截器在 driver 构造时读取），
// 因此框架在 ProvideConfig 阶段就会调用一次 InitOTEL；本插件只是拿到
// 同一个 provider，并在 Close 时负责 flush。
package tracing

import (
	"context"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/middleware"
)

// OTEL 是 OpenTelemetry 插件。tracing.startup 为 false 时它是空插件。
type OTEL struct {
	tp *tracesdk.TracerProvider
}

// NewOTEL 创建插件并取得进程级 tracer provider。
func NewOTEL(cfg *config.Config) (*OTEL, error) {
	tp, err := middleware.InitOTEL(cfg)
	if err != nil {
		return nil, err
	}
	return &OTEL{tp: tp}, nil
}

// Name 返回插件名。
func (o *OTEL) Name() string { return "otel" }

// Start 阻塞到 ctx 结束。
func (o *OTEL) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Ready 默认认为 tracing 随时可用。
func (o *OTEL) Ready(context.Context) error { return nil }

// Drain 无事可做。
func (o *OTEL) Drain(context.Context) error { return nil }

// Close 冲刷剩余的 span。它应当最后执行，所以插件要排在列表最前。
func (o *OTEL) Close(ctx context.Context) error {
	if o.tp == nil {
		return nil
	}
	return o.tp.Shutdown(ctx)
}
