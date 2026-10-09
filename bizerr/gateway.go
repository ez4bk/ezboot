package bizerr

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"

	"github.com/ez4bk/ezboot/grpc/gateway"
	"github.com/ez4bk/ezboot/internal/pberr"
)

// GatewayErrorHandler 网关上对业务错误进行处理
func GatewayErrorHandler() gateway.ErrorHandler {
	return func(ctx context.Context, mux *runtime.ServeMux, m runtime.Marshaler,
		w http.ResponseWriter, r *http.Request, err error) error {

		if errStatus, ok := status.FromError(err); ok {
			for _, detail := range errStatus.Details() {
				switch v := detail.(type) {
				case *pberr.WithHttpStatus:
					err = &runtime.HTTPStatusError{
						Err:        err,
						HTTPStatus: int(v.HttpCode),
					}
				}
			}
		}
		return err
	}
}
