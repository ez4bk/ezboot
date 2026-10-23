package ezboot

import (
	"google.golang.org/grpc"

	"github.com/ez4bk/ezboot/internal/gin-boot/grpc_boot"
)

type ServerType = grpc_boot.ServerType

func RegisterClient(serverKind string, newFunc interface{}) {
	grpc_boot.RegisterClient(serverKind, newFunc)
}

func InjectGrpcClient(v any) {
	grpc_boot.InjectAllClient(v)
}

// GetClient returns the cached gRPC connection configured under name.
func GetClient(name string) (*grpc.ClientConn, error) {
	return grpc_boot.GetClient(name)
}

// CloseClients closes every cached gRPC client connection.
func CloseClients() error {
	return grpc_boot.CloseClients()
}
