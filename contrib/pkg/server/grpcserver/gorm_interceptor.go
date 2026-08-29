package grpcserver

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// GormErrorInterceptor 把常见 gorm 错误映射为语义正确的 gRPC 状态码：
// ErrRecordNotFound → codes.NotFound（HTTP 404），避免记录不存在时返回 500。
func GormErrorInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err == nil {
		return resp, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return resp, status.Error(codes.NotFound, "record not found")
	}
	return resp, err
}
