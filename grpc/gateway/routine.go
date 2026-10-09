package gateway

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
)

// routineErrorHandler 路由相关错误的处理逻辑
// By default http error codes mapped on the following error codes:
//
//	NotFound -> grpc.NotFound
//	StatusBadRequest -> grpc.InvalidArgument
//	MethodNotAllowed -> grpc.Unimplemented
//	Other -> grpc.Internal, method is not expecting to be called for anything else
func routineErrorHandler(_ context.Context, _ *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, _ *http.Request, httpStatus int) bool {
	err := status.Error(codes.Internal, "Unexpected routing error")
	switch httpStatus {
	case http.StatusNotFound:
		err = status.Error(codes.NotFound, http.StatusText(httpStatus))
	case http.StatusBadRequest:
		err = status.Error(codes.InvalidArgument, http.StatusText(httpStatus))
	case http.StatusMethodNotAllowed:
		err = status.Error(codes.Unimplemented, http.StatusText(httpStatus))
	}

	pbMessage := status.Convert(err).Proto()
	w.Header().Set("Content-Type", m.ContentType(pbMessage))

	buf, err := m.Marshal(pbMessage)
	if err != nil {
		grpclog.Infof("Failed to marshal status: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(httpStatus)
		if _, err = w.Write(buf); err != nil {
			grpclog.Infof("Failed to write response: %v", err)
		}
	}

	return true
}
