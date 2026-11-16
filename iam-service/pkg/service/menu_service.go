package service

import (
	"context"
	"log/slog"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	"github.com/linjunhao1997/contrib/pkg/menu"
	"gorm.io/gorm"
)

// NewMenuService 构建通用菜单能力（contrib/pkg/menu，ADR 0001），注入 iam 宿主钩子：
//   - 应用上下文/账号身份取自 auth ctx（token 承载，多应用裁决 #8）；
//   - 菜单展示码校验走 iam 权限词汇规则（格式 + 服务段隔离裁决）。
//
// 菜单为纯展示配置（ADR 0002 + 纯码门槛裁决）：permission_code 是前端展示门槛，
// 可见性由前端按用户快照码过滤；角色-菜单绑定已退役，服务端不做角色过滤。
// 钩子在编译期静态齐全，NewMenuService 的校验错误不可达。
func NewMenuService(db *gorm.DB) *menu.MenuService {
	svc, err := menu.NewMenuService(menu.Config{
		DB:                       db,
		ApplicationIdFromContext: requireApplicationId,
		AccountIdFromContext:     auth.GetAccountId,
		ValidatePermissionCode:   validateMenuPermissionCode(db),
	})
	if err != nil {
		// 钩子静态齐全，理论不可达；防御性兜底
		slog.Error("build contrib menu service failed", "error", err)
		panic(err)
	}
	return svc
}

// validateMenuPermissionCode 菜单展示码的 iam 规则：格式（点式三段）+
// 服务段隔离（码第一段 = 所属应用注册的服务段）。
func validateMenuPermissionCode(db *gorm.DB) func(ctx context.Context, appId int64, code string) error {
	return func(ctx context.Context, appId int64, code string) error {
		if !model.IsValidPermissionCode(code) {
			return ErrInvalidPermissionCode
		}
		return requireCodeSegmentMatchesApp(ctx, db, appId, code)
	}
}
