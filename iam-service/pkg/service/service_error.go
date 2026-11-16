package service

import (
	"errors"

	"github.com/linjunhao1997/contrib/pkg/dict"
	"github.com/linjunhao1997/contrib/pkg/menu"
	"google.golang.org/grpc/codes"
)

// Error 是业务错误：Code 为业务错误码，通过 grpclib ErrorInterceptor 转成
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
	ErrInvalidAccountPassword   = NewErrorWithCode("1001001", codes.Unauthenticated, errors.New("invalid account password"))
	ErrUpdateAccountBusy        = NewErrorWithCode("1001002", codes.FailedPrecondition, errors.New("用户已被他人修改，请刷新后重试"))
	ErrRoleCodeDuplicated       = NewErrorWithCode("1001005", codes.AlreadyExists, errors.New("角色编码已被占用"))
	ErrPermissionCodeDuplicated = NewErrorWithCode("1001006", codes.AlreadyExists, errors.New("权限编码已被占用"))
	ErrAccountReferenced        = NewErrorWithCode("1001007", codes.FailedPrecondition, errors.New("用户已被绑定角色，请先解除绑定"))
	ErrAccountDisabled          = NewErrorWithCode("1001008", codes.PermissionDenied, errors.New("账号已禁用"))
	ErrRoleReferenced           = NewErrorWithCode("1001008", codes.FailedPrecondition, errors.New("角色已被用户绑定，请先解除绑定"))
	ErrPermissionReferenced     = NewErrorWithCode("1001009", codes.FailedPrecondition, errors.New("权限已被角色引用，请先解除引用"))
	ErrRoleNotFound             = NewErrorWithCode("1001010", codes.InvalidArgument, errors.New("角色编码不存在"))
	ErrPermissionNotFound       = NewErrorWithCode("1001011", codes.InvalidArgument, errors.New("权限编码不存在"))
	ErrUpdateRoleBusy           = NewErrorWithCode("1001012", codes.FailedPrecondition, errors.New("角色已被他人修改，请刷新后重试"))
	ErrUpdatePermissionBusy     = NewErrorWithCode("1001013", codes.FailedPrecondition, errors.New("权限已被他人修改，请刷新后重试"))
	ErrInvalidPermissionCode    = NewErrorWithCode("1001014", codes.InvalidArgument, errors.New("权限码格式不合法，须为 服务.模块.操作（如 iam.user.delete）"))
	ErrRoleAdminProtected       = NewErrorWithCode("1001018", codes.FailedPrecondition, errors.New("管理员角色为系统内置，禁止删除"))
	ErrUsernameDuplicated       = NewErrorWithCode("1001027", codes.AlreadyExists, errors.New("用户名已被占用"))
	ErrUsernameRequired         = NewErrorWithCode("1001046", codes.InvalidArgument, errors.New("用户名不能为空"))
	ErrEmailDuplicated          = NewErrorWithCode("1001048", codes.AlreadyExists, errors.New("邮箱已被占用"))
	ErrPasswordRequired         = NewErrorWithCode("1001049", codes.InvalidArgument, errors.New("初始密码不能为空"))
	// specs/009-ak-sk-auth 访问凭证域（1001040 起）
	ErrAccessCredentialNotFound       = NewErrorWithCode("1001040", codes.NotFound, errors.New("访问凭证不存在"))
	ErrAccessCredentialAccountInvalid = NewErrorWithCode("1001041", codes.FailedPrecondition, errors.New("绑定用户不存在或已停用"))
	ErrAccessCredentialLimitExceeded  = NewErrorWithCode("1001042", codes.ResourceExhausted, errors.New("该用户凭证数量已达上限"))
	ErrAccessCredentialSkDecrypt      = NewErrorWithCode("1001043", codes.Internal, errors.New("凭证密钥解密失败，请检查加密密钥配置"))
	ErrAccessCredentialUpdateBusy     = NewErrorWithCode("1001044", codes.FailedPrecondition, errors.New("凭证已被他人修改，请刷新后重试"))
	ErrAccountReferencedByCredential  = NewErrorWithCode("1001045", codes.FailedPrecondition, errors.New("用户名下存在访问凭证，请先删除凭证"))
	// 菜单/字典错误已随通用能力抽至 contrib（ADR 0001）；
	// 此处保留别名以维持既有引用与业务码
	ErrMenuNotFound           = menu.ErrMenuNotFound
	ErrMenuHasChildren        = menu.ErrMenuHasChildren
	ErrUpdateMenuBusy         = menu.ErrUpdateMenuBusy
	ErrDictTypeCodeDuplicated = dict.ErrDictTypeCodeDuplicated
	ErrDictTypeReferenced     = dict.ErrDictTypeReferenced
	ErrUpdateDictTypeBusy     = dict.ErrUpdateDictTypeBusy
	ErrUpdateDictItemBusy     = dict.ErrUpdateDictItemBusy
	// 多应用域（#8 裁决：应用上下文由 token 承载，数据按应用隔离）
	ErrApplicationContextRequired    = NewErrorWithCode("1001057", codes.FailedPrecondition, errors.New("请先选择应用"))
	ErrPermissionCodeSegmentMismatch = NewErrorWithCode("1001058", codes.InvalidArgument, errors.New("权限码服务段与当前应用不一致"))
	ErrApplicationNotFoundForScope   = NewErrorWithCode("1001059", codes.NotFound, errors.New("应用不存在"))
	ErrApplicationCodeDuplicated     = NewErrorWithCode("1001060", codes.AlreadyExists, errors.New("应用服务段编码已被占用"))
)
