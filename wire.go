package ezboot

import (
	"github.com/google/wire"

	"github.com/ez4bk/ezboot/grpc"
	grpcclient "github.com/ez4bk/ezboot/grpc/client"
	"github.com/ez4bk/ezboot/http"
	"github.com/ez4bk/ezboot/tracing"
)

// ProviderSet 提供所有项目都会用到的基础依赖。
var ProviderSet = wire.NewSet(
	ProvideConfig,
	ProvideLogger,
)

// DefaultProviderSet 提供默认插件集合：
//
//	OTEL + HTTP + gRPC + gRPC 客户端连接池
//
// infra 插件（MySQL / Redis / Kafka / RabbitMQ / Elasticsearch / Etcd）
// 不在这里：只用 HTTP 的服务不应该因为引入框架就打开 Redis。
var DefaultProviderSet = wire.NewSet(
	tracing.NewOTEL,
	grpc.New,
	http.New,
	grpcclient.NewGRPCClient,
)
