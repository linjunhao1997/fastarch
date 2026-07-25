package grpclib

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	TraceID = "trace-id"
)

func TraceIDInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	traceID := ""
	if vals := md.Get(TraceID); len(vals) > 0 {
		traceID = vals[0]
	}

	if traceID != "" {
		outgoingMD := metadata.Pairs(TraceID, traceID)
		ctx = metadata.NewOutgoingContext(ctx, outgoingMD)
	}

	return handler(ctx, req)
}
