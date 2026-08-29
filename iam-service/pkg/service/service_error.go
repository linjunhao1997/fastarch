package service

import (
	"errors"

	"google.golang.org/grpc/codes"
)

// Error 是业务错误：Code 为业务错误码，通过 grpclib.ErrorInterceptor 转成
// gRPC ErrorInfo 的 reason 透传给客户端；grpcCode 决定对应的 gRPC/HTTP 状态码。
type Error struct {
	Code     string
	grpcCode codes.Code
	Err      error
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

// BusinessCode 实现 grpclib.BusinessError，业务码随 ErrorInfo.Reason 下发。
func (e *Error) BusinessCode() string {
	return e.Code
}

// GRPCCode 实现 grpclib 的可选接口；未指定时默认 InvalidArgument。
func (e *Error) GRPCCode() codes.Code {
	if e.grpcCode == 0 {
		return codes.InvalidArgument
	}
	return e.grpcCode
}

// NewError 创建业务错误，gRPC 状态码默认 InvalidArgument。
func NewError(code string, err error) *Error {
	return &Error{Code: code, Err: err}
}

// NewErrorWithCode 创建业务错误并显式指定 gRPC 状态码。
func NewErrorWithCode(code string, grpcCode codes.Code, err error) *Error {
	return &Error{Code: code, grpcCode: grpcCode, Err: err}
}

var (
	ErrInvalidAccountPassword = NewErrorWithCode("1001001", codes.Unauthenticated, errors.New("invalid account password"))
	ErrUpdateAccountBusy      = NewErrorWithCode("1001002", codes.FailedPrecondition, errors.New("update account busy"))
)
