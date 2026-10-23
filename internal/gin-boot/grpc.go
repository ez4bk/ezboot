package boot

import (
	"time"

	"google.golang.org/grpc"

	_grpc "github.com/ez4bk/ezboot/internal/gin-boot/grpc_boot"
)

type grpcType struct {
}

var (
	GRPC = &grpcType{}
)

// NewClient NewClient
func (g *grpcType) NewClient(addr string, timeout time.Duration, dialOptions ...grpc.DialOption) (*grpc.ClientConn, error) {
	return _grpc.NewClient(addr, timeout, dialOptions...)
}

// DefaultServer DefaultServer
func (g *grpcType) DefaultServer() (*_grpc.ServerType, error) {
	return _grpc.DefaultServer()
}

// DefaultGateway DefaultServer
func (g *grpcType) DefaultGateway() (*_grpc.ServerType, error) {
	return _grpc.DefaultServer()
}
