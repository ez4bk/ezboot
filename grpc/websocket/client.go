package websocket

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ez4bk/ezboot/xjson"
	"github.com/ez4bk/ezboot/xref"
)

var (
	// ErrWebsocketMethodNotFound 表示找不到对应的Websocket方法
	ErrWebsocketMethodNotFound = errors.New("grpc: method not found")

	// _ConnIdMetadataKey 连接Id在元数据中的键名
	_ConnIdMetadataKey = "X-Websocket-ConnId"
)

// grpcClient 是一个使用反射处理的GRPC客户端
type grpcClient struct {
	c reflect.Value
	m map[string]reflect.Value
	r map[string]reflect.Type
}

// Call 尝试调用一个服务接口对象
func (c *grpcClient) Call(ctx context.Context, kind string, req any) (resp any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("internal error: %req", rec)
		}
	}()

	kind = strings.ToLower(kind)
	if method, ok := c.m[kind]; ok {
		reqObject := xref.NewObject[any](c.r[kind])
		if err = xjson.Convert(reqObject, req); err != nil {
			return nil, err
		}

		if connId, hasConnId := GetConnId(ctx); hasConnId {
			ctx = metadata.AppendToOutgoingContext(ctx, _ConnIdMetadataKey, fmt.Sprintf("%d", connId))
		}

		callResults := method.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(reqObject),
		})
		if callResults[1].IsNil() {
			return callResults[0].Interface(), nil
		}
		return nil, callResults[1].Interface().(error)
	}
	return nil, ErrWebsocketMethodNotFound
}

// newClient 创建一个GRPC客户端
func newClient[T any](factory func(grpc.ClientConnInterface) T, conn grpc.ClientConnInterface) (c *grpcClient, err error) {
	client := factory(conn)
	c = &grpcClient{
		c: reflect.ValueOf(client),
		m: make(map[string]reflect.Value),
		r: make(map[string]reflect.Type),
	}

	clientType := reflect.TypeOf(client)
	for i := 0; i < clientType.NumMethod(); i++ {
		methodType := clientType.Method(i)
		if methodType.IsExported() {
			methodValue := c.c.Method(i)
			c.m[strings.ToLower(methodType.Name)] = methodValue
			c.r[strings.ToLower(methodType.Name)] = methodValue.Type().In(1)
		}
	}

	return c, nil
}
