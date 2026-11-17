package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/linjunhao1997/contrib/pkg/dict"
	"github.com/linjunhao1997/contrib/pkg/menu"
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
