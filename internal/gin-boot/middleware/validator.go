package middleware

import (
	"context"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func ValidatorUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if pb, ok := req.(proto.Message); ok {
			if err := protovalidate.Validate(pb); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}
