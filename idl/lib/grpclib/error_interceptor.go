package grpclib

import (
	"context"
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BusinessError 是业务错误的抽象：业务码通过 ErrorInfo.Reason 透传给客户端
// （grpc-gateway 会放进错误响应 JSON 的 details 数组）。
type BusinessError interface {
	error
	BusinessCode() string
}

// grpcCodeError 可选实现：业务错误自定义 gRPC 状态码，默认 InvalidArgument。
type grpcCodeError interface {
	GRPCCode() codes.Code
}

// ErrorInterceptor 把业务错误转换为携带 ErrorInfo 详情的 gRPC 状态，
// 非业务错误（已是 gRPC status 的错误、gorm 错误等）原样透传。
// domain 标识错误来源服务，如 "iam-service"。
func ErrorInterceptor(domain string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		var be BusinessError
		if !errors.As(err, &be) {
			return resp, err
		}

		code := codes.InvalidArgument
		var ce grpcCodeError
		if errors.As(err, &ce) {
			code = ce.GRPCCode()
		}

		st := status.New(code, be.Error())
		withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
			Reason: be.BusinessCode(),
			Domain: domain,
		})
		if err != nil {
			return resp, st.Err()
		}
		return resp, withDetails.Err()
	}
}
