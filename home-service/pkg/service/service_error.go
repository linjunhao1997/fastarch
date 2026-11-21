package service

import (
	"errors"

	"github.com/linjunhao1997/contrib/pkg/menu"
	"google.golang.org/grpc/codes"
)

// Error 是业务错误：Code 为业务错误码（home 段 2002xxx），经 grpclib
// ErrorInterceptor 转成带 ErrorInfo（reason=业务码，domain=home-service）的 gRPC 状态。
type Error struct {
	Code     string
	grpcCode codes.Code
	Err      error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

func (e *Error) BusinessCode() string { return e.Code }

func (e *Error) GRPCCode() codes.Code {
	if e.grpcCode == 0 {
		return codes.InvalidArgument
	}
	return e.grpcCode
}

func NewError(code string, err error) *Error {
	return &Error{Code: code, Err: err}
}

func NewErrorWithCode(code string, grpcCode codes.Code, err error) *Error {
	return &Error{Code: code, grpcCode: grpcCode, Err: err}
}

var (
	// 首页配置域：2002xxx。
	ErrHomepageNotFound         = NewErrorWithCode("2002001", codes.NotFound, errors.New("首页配置不存在"))
	ErrInvalidHomepageJson      = NewErrorWithCode("2002002", codes.InvalidArgument, errors.New("首页内容不是合法的 JSON 对象"))
	ErrHomepageTooLarge         = NewErrorWithCode("2002003", codes.InvalidArgument, errors.New("首页内容超过 200KB 上限"))
	ErrHomepageVersionConflict  = NewErrorWithCode("2002004", codes.Aborted, errors.New("首页配置已被他人更新，请刷新后重试"))
	ErrHomepageNothingToPublish = NewErrorWithCode("2002005", codes.FailedPrecondition, errors.New("当前草稿为空，无法发布"))
	ErrInvalidPermissionCode    = NewErrorWithCode("2002006", codes.InvalidArgument, errors.New("权限码格式不合法或不属于 home 服务段"))

	// 菜单错误别名（业务码沿用 contrib，保持既有客户端约定）
	ErrMenuNotFound    = menu.ErrMenuNotFound
	ErrMenuHasChildren = menu.ErrMenuHasChildren
	ErrUpdateMenuBusy  = menu.ErrUpdateMenuBusy
)
