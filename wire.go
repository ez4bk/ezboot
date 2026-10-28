package ezboot

import (
	"github.com/google/wire"

	grpcclient "github.com/ez4bk/ezboot/grpc/client"
	"github.com/ez4bk/ezboot/tracing"
)

// ProviderSet 提供所有应用都会用到、且不需要定制的依赖与插件：
//
//	配置 + 日志 + 就绪状态 + tracing + gRPC 客户端连接池
//
// http 与 grpc 插件刻意不在这里：它们需要路由、service 注册、网关等
// 应用特有的参数，必须由应用自己的 provider 构造。把 grpc.New / http.New
// 放进这个集合会与应用的构造器产生同类型冲突（wire: multiple bindings）。
//
// infra 插件（MySQL / Redis / Kafka / RabbitMQ / Elasticsearch / Etcd）
// 也不在这里：只用 HTTP 的服务不应该因为引入框架就打开 Redis。
var ProviderSet = wire.NewSet(
	ProvideConfig,
	ProvideLogger,
	NewState,
	tracing.NewOTEL,
	grpcclient.NewGRPCClient,
)
