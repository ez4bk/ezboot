// Package client 把进程级 gRPC 客户端连接池暴露成一个插件。
//
// 连接本身在第一次 GetClient 时才建立，所以 Start 只负责等待退出；
// Close 统一关闭池里的所有连接。
package client

import (
	"context"
	"errors"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
	"github.com/ez4bk/ezboot/internal/gin-boot/grpc_boot"
)

// GRPCClient 是 gRPC 客户端连接池插件。
type GRPCClient struct {
	cfg *config.Config
}

// NewGRPCClient 创建一个客户端插件。连接池的配置由框架在
// LoadFrameworkConfig / InitTelemetry 阶段载入，这里只持有配置以便
// 未来的按需重建。
func NewGRPCClient(cfg *config.Config) (*GRPCClient, error) {
	if cfg == nil {
		return nil, errors.New("grpc/client: config is nil")
	}
	return &GRPCClient{cfg: cfg}, nil
}

// Name 返回插件名。
func (c *GRPCClient) Name() string { return "grpc-client" }

// Start 阻塞到 ctx 结束。
func (c *GRPCClient) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Ready 默认认为客户端池随时可用：连接在第一次调用时才建立。
func (c *GRPCClient) Ready(context.Context) error { return nil }

// Drain 无事可做：在途 RPC 由 gRPC server 的排空负责。
func (c *GRPCClient) Drain(context.Context) error { return nil }

// Close 关闭池里的所有连接，并拒绝后续的 GetClient。
func (c *GRPCClient) Close(context.Context) error {
	return grpc_boot.CloseClients()
}
