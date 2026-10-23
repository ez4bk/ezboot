package middleware

import (
	"context"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/ez4bk/ezboot/bizerr"
)

func ValidatorUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if pb, ok := req.(proto.Message); ok {
			if err := protovalidate.Validate(pb); err != nil {
				return nil, bizerr.InvalidArgument.Render(err.Error())
			}
		}
		return handler(ctx, req)
	}
}
