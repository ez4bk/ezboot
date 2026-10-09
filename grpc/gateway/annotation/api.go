package annotation

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/ez4bk/ezboot/grpc/gateway"
)

const (
	_HttpUrlMetadataKey          = "X-Http-Url"
	_HttpHeaderMetadataKeyPrefix = "X-Http-Header-"
)

// Http 在gRPC上下文对象中添加Http的元数据
func Http() gateway.Annotator {
	return func(ctx context.Context, req *http.Request, md metadata.MD) {
		md.Set(_HttpUrlMetadataKey, "https://"+req.Host+req.URL.String())
		for k, v := range req.Header {
			md.Set(_HttpHeaderMetadataKeyPrefix+strings.ToLower(k), v...)
		}
	}
}

// UrlFromContext 从上下文对象中获取请求地址
func UrlFromContext(ctx context.Context) (string, bool) {
	return loadFromMetadata(ctx, _HttpUrlMetadataKey)
}

// HostnameFromContext 从上下文对象中获取域名字符串
func HostnameFromContext(ctx context.Context) (string, error) {
	if api, ok := UrlFromContext(ctx); ok {
		if u, err := url.Parse(api); err == nil {
			return u.Host, nil
		}
	}
	return "", nil
}

// HeaderFromContext 从上下文对象中获取Http头信息
func HeaderFromContext(ctx context.Context, name string) (string, bool) {
	return loadFromMetadata(ctx, _HttpHeaderMetadataKeyPrefix+strings.ToLower(name))
}

// loadFromMetadata 从上下文对象中获取一个值
func loadFromMetadata(ctx context.Context, name string) (string, bool) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(name); len(vals) != 0 {
			return vals[0], true
		}
	}
	return "", false
}
