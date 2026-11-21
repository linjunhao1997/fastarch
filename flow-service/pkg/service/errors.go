package service

import (
	"errors"

	"github.com/linjunhao1997/contrib/pkg/dict"
	"github.com/linjunhao1997/contrib/pkg/menu"
	"google.golang.org/grpc/codes"
)

// Error 是业务错误：Code 为 flow 段业务码（3001xxx）。
type Error struct {
	Code     string
	grpcCode codes.Code
	Err      error
}

func (e *Error) Error() string        { return e.Err.Error() }
func (e *Error) Unwrap() error        { return e.Err }
func (e *Error) BusinessCode() string { return e.Code }

func (e *Error) GRPCCode() codes.Code {
	if e.grpcCode == 0 {
		return codes.InvalidArgument
	}
	return e.grpcCode
}

func NewError(code string, err error) *Error { return &Error{Code: code, Err: err} }

func NewErrorWithCode(code string, grpcCode codes.Code, err error) *Error {
	return &Error{Code: code, grpcCode: grpcCode, Err: err}
}

var (
	// 3001001 是"证据必填"的专用码：agent 与前端都按它做提示，勿改。
	ErrEvidenceRequired      = NewErrorWithCode("3001001", codes.FailedPrecondition, errors.New("该问题需要提交证据才能闭环（commit / 工单号 / 部署单 / 文档链接）"))
	ErrTaskBlocked           = NewErrorWithCode("3001002", codes.FailedPrecondition, errors.New("任务仍有未闭环问题，先闭环或作废后再完成"))
	ErrRoleMismatch          = NewErrorWithCode("3001003", codes.PermissionDenied, errors.New("该任务不属于你当前角色，先切换角色再操作"))
	ErrQuestionClosed        = NewErrorWithCode("3001004", codes.FailedPrecondition, errors.New("问题已经闭环，不能重复回答"))
	ErrAnswerRequired        = NewErrorWithCode("3001005", codes.InvalidArgument, errors.New("闭环需给出结论：选一个选项或填写说明"))
	ErrTaskNotFound          = NewErrorWithCode("3001006", codes.NotFound, errors.New("任务不存在"))
	ErrQuestionNotFound      = NewErrorWithCode("3001007", codes.NotFound, errors.New("问题不存在"))
	ErrRoleNotFound          = NewErrorWithCode("3001008", codes.NotFound, errors.New("工作角色不存在"))
	ErrTaskNotOpen           = NewErrorWithCode("3001009", codes.FailedPrecondition, errors.New("任务已结束，不能再抛问题"))
	ErrInvalidAnswerKind     = NewErrorWithCode("3001010", codes.InvalidArgument, errors.New("闭环方式只能是 decision 或 evidence"))
	ErrOptionNotFound        = NewErrorWithCode("3001011", codes.InvalidArgument, errors.New("所选选项不在该问题的候选清单里"))
	ErrRoleNotHeld           = NewErrorWithCode("3001012", codes.PermissionDenied, errors.New("你不能切换到未持有的角色"))
	ErrUnauthenticated       = NewErrorWithCode("3001013", codes.Unauthenticated, errors.New("缺少登录身份"))
	ErrInvalidPermissionCode = NewErrorWithCode("3001014", codes.InvalidArgument, errors.New("权限码格式不合法或不属于 flow 服务段"))
	ErrSelfQuestion          = NewErrorWithCode("3001015", codes.InvalidArgument, errors.New("不能向自己当前的角色提问，请先切到需要协作的角色"))
	ErrTaskEnded             = NewErrorWithCode("3001016", codes.FailedPrecondition, errors.New("任务已结束（已完成或已作废），不能再执行该操作"))
	ErrConcurrentUpdate      = NewErrorWithCode("3001017", codes.Aborted, errors.New("操作冲突：你的数据刚被其他人改过，请刷新后重试"))
	ErrNotBoss               = NewErrorWithCode("3001018", codes.PermissionDenied, errors.New("只有当前角色是老板（boss）才能查看和分配成员角色，请先切换角色"))
	ErrMemberNotFound        = NewErrorWithCode("3001019", codes.NotFound, errors.New("成员不存在"))
	ErrRoleCodesEmpty        = NewErrorWithCode("3001020", codes.InvalidArgument, errors.New("至少要给成员保留一个工作角色"))
	ErrSelfBossLockout       = NewErrorWithCode("3001021", codes.FailedPrecondition, errors.New("不能移除自己身上的老板角色，否则将没有人能分配角色"))
	ErrEscalateTooDeep       = NewErrorWithCode("3001022", codes.FailedPrecondition, errors.New("升级链已到 3 层，再往下说明分工没定清，请直接找老板开会定责"))

	// v1.1 闭环质量：
	ErrInvalidEvidenceFormat = NewErrorWithCode("3001023", codes.InvalidArgument, errors.New("证据格式不合法：需为 URL / commit SHA / 工单号 / 部署单 / 仓库内文档路径（*.md/*.pdf）之一"))
	ErrQuestionNotAnswered   = NewErrorWithCode("3001024", codes.FailedPrecondition, errors.New("只有已闭环的问题才能重开"))
	ErrReopenForbidden       = NewErrorWithCode("3001025", codes.PermissionDenied, errors.New("只有原提问人可重开该问题；老板角色保留兜底权"))
	ErrTaskReopenEnded       = NewErrorWithCode("3001026", codes.FailedPrecondition, errors.New("任务已结束（已完成或已作废），不能重开其下的问题；请新建任务继续跟进"))

	// 菜单/字典错误别名（沿用 contrib 既有业务码）
	ErrMenuNotFound           = menu.ErrMenuNotFound
	ErrMenuHasChildren        = menu.ErrMenuHasChildren
	ErrUpdateMenuBusy         = menu.ErrUpdateMenuBusy
	ErrDictTypeCodeDuplicated = dict.ErrDictTypeCodeDuplicated
	ErrDictTypeReferenced     = dict.ErrDictTypeReferenced
	ErrUpdateDictTypeBusy     = dict.ErrUpdateDictTypeBusy
	ErrUpdateDictItemBusy     = dict.ErrUpdateDictItemBusy
)
