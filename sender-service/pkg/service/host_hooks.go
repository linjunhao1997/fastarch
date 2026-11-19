package service

import (
	"context"
	"regexp"
	"strings"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/linjunhao1997/contrib/pkg/dict"
	"github.com/linjunhao1997/contrib/pkg/menu"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// senderServiceSegment 是 sender 应用的服务段（权限码第一段 = 应用 code，GLOSSARY）。
const senderServiceSegment = "sender"

var permissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)

// fixedApplicationId 供菜单/字典钩子使用：sender 的应用 id 固定，不取 token 里的
// applicationId（那是 IAM 多应用控制台的运行时切换语义，ADR 0005）。
func fixedApplicationId(appId int64) func(context.Context) (int64, error) {
	return func(context.Context) (int64, error) { return appId, nil }
}

// dataApplicationId 返回当前请求的数据归属应用 id：优先取 IAM 校验 appCode 后回传的
// dataAppId（人从 sender 控制台选中的应用），未注入时回落到宿主固定应用（AK/SK
// 机机调用沿凭证归属由 IAM 注入，历史/内部调用用 sender 默认）。
func dataApplicationId(ctx context.Context, fallback int64) int64 {
	if id := authz.GetDataAppId(ctx); id > 0 {
		return id
	}
	return fallback
}

// dataApplicationSummary 按当前请求的数据归属构造展示摘要：
// id 取 IAM 校验回传的 dataAppId，code 取已校验的 appCode，name 暂无目录时以 code 代替；
// 均未注入（AK/SK 机机调用/内部调用）时回落到宿主固定摘要。
// 注意：name 不是展示名，待 sender 持有应用目录或 IAM 回传展示名后完善（见任务清单）。
func dataApplicationSummary(ctx context.Context, fallback *common_v1.ApplicationSummary) *common_v1.ApplicationSummary {
	appId := authz.GetDataAppId(ctx)
	code := authz.GetDataAppCode(ctx)
	if appId <= 0 || code == "" {
		return fallback
	}
	return &common_v1.ApplicationSummary{
		Id:     cast.ToString(appId),
		Code:   code,
		Name:   code,
		Status: "enabled",
	}
}

// validateSenderPermissionCode 校验菜单展示码：格式合法且服务段属于 sender。
// 权限码是唯一授权词汇（GLOSSARY），菜单码是前端展示门槛（ADR 0002），
// 授权词汇规则归宿主（sender），故在此校验服务段隔离。
func validateSenderPermissionCode(ctx context.Context, appId int64, code string) error {
	if !permissionCodePattern.MatchString(code) {
		return ErrInvalidPermissionCode
	}
	segment := strings.SplitN(code, ".", 2)[0]
	if segment != senderServiceSegment {
		return ErrInvalidPermissionCode
	}
	return nil
}

// NewMenuService 构建 contrib 菜单能力（ADR 0001，sender 自持表），注入 sender 宿主钩子。
func NewMenuService(db *gorm.DB, applicationId int64) (*menu.MenuService, error) {
	return menu.NewMenuService(menu.Config{
		DB:                       db,
		ApplicationIdFromContext: fixedApplicationId(applicationId),
		AccountIdFromContext:     authz.GetAccountId,
		ValidatePermissionCode:   validateSenderPermissionCode,
	})
}

// NewDictService 构建 contrib 字典能力（sender 自持表），注入 sender 宿主钩子。
func NewDictService(db *gorm.DB, applicationId int64) (*dict.DictService, error) {
	return dict.NewDictService(dict.Config{
		DB:                       db,
		ApplicationIdFromContext: fixedApplicationId(applicationId),
		AccountIdFromContext:     authz.GetAccountId,
	})
}
