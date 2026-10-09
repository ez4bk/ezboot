package gateway

import (
	"context"
	"net/http"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const (
	_HttpCodeMetadata = "x-http-code"
)

// SetGrpcHttpStatusCode 设置HTTP的响应状态码
func SetGrpcHttpStatusCode(ctx context.Context, code int) error {
	return grpc.SetHeader(ctx, metadata.Pairs(_HttpCodeMetadata, strconv.Itoa(code)))
}

// SendHttpLocation 发送重定向响应
func SendHttpLocation(ctx context.Context, location string) error {
	err := grpc.SetHeader(ctx, metadata.Pairs("Location", location))
	if err != nil {
		return err
	}
	return SetGrpcHttpStatusCode(ctx, 302)
}

// withForwardResponseOption 修改Http响应的状态码
func (gh *Handler) withForwardResponseOption(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	// set http status code
	if vals := md.HeaderMD.Get(_HttpCodeMetadata); len(vals) > 0 {
		code, err := strconv.Atoi(vals[0])
		if err != nil {
			return err
		}
		// delete the headers to not expose any grpc-metadata in http response
		delete(md.HeaderMD, _HttpCodeMetadata)
		delete(w.Header(), "Grpc-Metadata-X-Http-Code")
		w.WriteHeader(code)
	}
	return nil
}
