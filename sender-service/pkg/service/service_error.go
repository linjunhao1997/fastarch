package service

import (
	"errors"

	"github.com/linjunhao1997/contrib/pkg/dict"
	"github.com/linjunhao1997/contrib/pkg/menu"
	"google.golang.org/grpc/codes"
)

// Error 是业务错误：Code 为业务错误码（sender 段 2001xxx），经 grpclib
// ErrorInterceptor 转成带 ErrorInfo（reason=业务码，domain=sender-service）的 gRPC 状态。
type Error struct {
	Code     string
	grpcCode codes.Code
	Err      error
}

func (e *Error) Error() string   { return e.Err.Error() }
func (e *Error) Unwrap() error   { return e.Err }
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
	// 消息发送/发送记录域
	ErrMessageNotFound        = NewErrorWithCode("2001001", codes.NotFound, errors.New("发送记录不存在"))
	ErrTemplateNotFound       = NewErrorWithCode("2001002", codes.NotFound, errors.New("消息模板不存在"))
	ErrTemplateCodeDuplicated = NewErrorWithCode("2001003", codes.AlreadyExists, errors.New("模板编码已被占用"))
	ErrUpdateTemplateBusy     = NewErrorWithCode("2001004", codes.FailedPrecondition, errors.New("模板已被他人修改，请刷新后重试"))
	ErrInvalidPermissionCode  = NewErrorWithCode("2001005", codes.InvalidArgument, errors.New("权限码格式不合法或不属于当前应用服务段"))

	// 菜单/字典错误别名（业务码沿用 contrib，保持既有客户端约定）
	ErrMenuNotFound           = menu.ErrMenuNotFound
	ErrMenuHasChildren        = menu.ErrMenuHasChildren
	ErrUpdateMenuBusy         = menu.ErrUpdateMenuBusy
	ErrDictTypeCodeDuplicated = dict.ErrDictTypeCodeDuplicated
	ErrDictTypeReferenced     = dict.ErrDictTypeReferenced
	ErrUpdateDictTypeBusy     = dict.ErrUpdateDictTypeBusy
	ErrUpdateDictItemBusy     = dict.ErrUpdateDictItemBusy
)
