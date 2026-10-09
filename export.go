package ezboot

import grpc_boot "github.com/ez4bk/ezboot/internal/gin-boot/grpc-boot"

type ServerType = grpc_boot.ServerType

func RegisterClient(serverKind string, newFunc interface{}) {
	grpc_boot.RegisterClient(serverKind, newFunc)
}

func InjectGrpcClient(v any) {
	grpc_boot.InjectAllClient(v)
}
