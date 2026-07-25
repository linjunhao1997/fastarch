package grpclib

import (
	"buf.build/go/protovalidate"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func ValidationUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	msg, ok := req.(proto.Message)
	if !ok {
		return req, nil
	}
	if err := protovalidate.Validate(msg); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}
