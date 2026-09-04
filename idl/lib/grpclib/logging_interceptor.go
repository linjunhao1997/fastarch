package grpclib

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingUnaryServerInterceptor 以 slog 记录每个已完成 RPC 的结构化日志：
// method、grpc_code、business_code（实现 BusinessError 接口的错误或带 ErrorInfo
// 详情的状态错误透出业务码）、duration_ms、trace_id；成功 Info 级、错误 Warn 级
// （research.md D13-①，宪法 X）。底层错误仅入日志，不透出客户端。
func LoggingUnaryServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)

	attrs := []any{
		"method", info.FullMethod,
		"grpc_code", status.Code(err).String(),
		"duration_ms", time.Since(start).Milliseconds(),
		"trace_id", TraceIDFromContext(ctx),
	}

	var be BusinessError
	if err != nil {
		if errors.As(err, &be) {
			attrs = append(attrs, "business_code", be.BusinessCode())
		} else {
			for _, detail := range status.Convert(err).Details() {
				if errorInfo, ok := detail.(*errdetails.ErrorInfo); ok {
					attrs = append(attrs, "business_code", errorInfo.Reason)
					break
				}
			}
		}
		attrs = append(attrs, "error", err)
		slog.Warn("grpc rpc failed", attrs...)
	} else {
		slog.Info("grpc rpc completed", attrs...)
	}

	return resp, err
}
