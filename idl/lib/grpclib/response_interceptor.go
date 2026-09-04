package grpclib

import (
	"context"

	"github.com/go-dev-frame/sponge/pkg/krand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	TraceID = "trace-id"
)

// traceIDContextKey 是 trace-id 在 context 中的键（供 Logging 拦截器与
// service 层日志读取，与响应 header 字段对齐）。
type traceIDContextKey struct{}

// TraceIDInterceptor 透传/生成 trace-id：上游 metadata 携带 trace-id 时沿用，
// 否则以 krand 生成；随后写入请求 metadata（供下游读取）与响应 header
// （供客户端关联日志，research.md D13-①）。
func TraceIDInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	traceID := ""
	if vals := md.Get(TraceID); len(vals) > 0 {
		traceID = vals[0]
	}
	if traceID == "" {
		traceID = krand.String(krand.R_All, 16)
	}

	md = md.Copy()
	md.Set(TraceID, traceID)
	ctx = metadata.NewIncomingContext(ctx, md)
	ctx = context.WithValue(ctx, traceIDContextKey{}, traceID)
	ctx = metadata.AppendToOutgoingContext(ctx, TraceID, traceID)

	// 响应 header 写入失败不影响请求处理（header 可能已随流发出）
	_ = grpc.SetHeader(ctx, metadata.Pairs(TraceID, traceID))

	return handler(ctx, req)
}

// TraceIDFromContext 返回当前请求的 trace-id（由 TraceIDInterceptor 写入）；
// 未经过 TraceID 拦截器时返回空串。
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDContextKey{}).(string); ok {
		return v
	}
	return ""
}
