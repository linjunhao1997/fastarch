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

// flowServiceSegment 是 flow 应用的服务段（权限码第一段 = 应用 code）。
const flowServiceSegment = "flow"

var permissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)

// fixedApplicationId 供菜单/字典钩子使用：flow 的应用 id 固定，不看 token 里的运行时切换。
func fixedApplicationId(appId int64) func(context.Context) (int64, error) {
	return func(context.Context) (int64, error) { return appId, nil }
}

// validateFlowPermissionCode 校验菜单展示码：格式合法且服务段属于 flow。
func validateFlowPermissionCode(_ context.Context, _ int64, code string) error {
	if !permissionCodePattern.MatchString(code) {
		return ErrInvalidPermissionCode
	}
	if strings.SplitN(code, ".", 2)[0] != flowServiceSegment {
		return ErrInvalidPermissionCode
	}
	return nil
}

// NewMenuService 构建 contrib 菜单能力（flow 自持表），注入 flow 宿主钩子。
func NewMenuService(db *gorm.DB, applicationId int64) (*menu.MenuService, error) {
	return menu.NewMenuService(menu.Config{
		DB:                       db,
		ApplicationIdFromContext: fixedApplicationId(applicationId),
		AccountIdFromContext:     authz.GetAccountId,
		ValidatePermissionCode:   validateFlowPermissionCode,
	})
}

// NewDictService 构建 contrib 字典能力（flow 自持表）。
func NewDictService(db *gorm.DB, applicationId int64) (*dict.DictService, error) {
	return dict.NewDictService(dict.Config{
		DB:                       db,
		ApplicationIdFromContext: fixedApplicationId(applicationId),
		AccountIdFromContext:     authz.GetAccountId,
	})
}
