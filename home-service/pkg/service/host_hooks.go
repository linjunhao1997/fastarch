package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/linjunhao1997/contrib/pkg/menu"
	"gorm.io/gorm"
)

// homeServiceSegment 是 home 应用的服务段（权限码第一段 = 应用 code，GLOSSARY）。
const homeServiceSegment = "home"

var permissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)

// fixedApplicationId 供菜单钩子使用：home 的应用 id 固定，不取 token 里的
// applicationId（那是 IAM 多应用控制台的运行时切换语义，ADR 0005）。
func fixedApplicationId(appId int64) func(context.Context) (int64, error) {
	return func(context.Context) (int64, error) { return appId, nil }
}

// validateHomePermissionCode 校验菜单展示码：格式合法且服务段属于 home。
// 权限码是唯一授权词汇（GLOSSARY），菜单码是前端展示门槛（ADR 0002），
// 授权词汇规则归宿主（home），故在此校验服务段隔离。
func validateHomePermissionCode(_ context.Context, _ int64, code string) error {
	if !permissionCodePattern.MatchString(code) {
		return ErrInvalidPermissionCode
	}
	segment := strings.SplitN(code, ".", 2)[0]
	if segment != homeServiceSegment {
		return ErrInvalidPermissionCode
	}
	return nil
}

// NewMenuService 构建 contrib 菜单能力（ADR 0001，home 自持表），注入 home 宿主钩子。
// 返回类型 *menu.MenuService 的方法集是 home.v1.MenuServiceServer 的超集，
// 可直接注册到只暴露 ListMyMenuTree 的 proto 服务。
func NewMenuService(db *gorm.DB, applicationId int64) (*menu.MenuService, error) {
	return menu.NewMenuService(menu.Config{
		DB:                       db,
		ApplicationIdFromContext: fixedApplicationId(applicationId),
		AccountIdFromContext:     authz.GetAccountId,
		ValidatePermissionCode:   validateHomePermissionCode,
	})
}
