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
//   - 应用上下文/账号/管理员身份取自 auth ctx（token 承载，多应用裁决 #8）；
//   - 角色绑定经 t_r_user_role / t_r_role_menu（仅决定菜单可见性）；
//   - 菜单码校验走 iam 权限词汇规则（格式 + 服务段隔离裁决）。
//
// 菜单不参与授权（ADR 0002）：permission_code 为前端展示门槛，绑定变更
// 不影响权限快照，故无需快照失效回调。
// 钩子在编译期静态齐全，NewMenuService 的校验错误不可达。
func NewMenuService(db *gorm.DB) *menu.MenuService {
	svc, err := menu.NewMenuService(menu.Config{
		DB:                       db,
		ApplicationIdFromContext: requireApplicationId,
		AccountIdFromContext:     auth.GetAccountId,
		IsAdminFromContext:       auth.IsAdminFromContext,
		RoleIdsForAccount:        pluckAccountRoleIds(db),
		RoleExists:               roleExists(db),
		ValidatePermissionCode:   validateMenuPermissionCode(db),
	})
	if err != nil {
		// 钩子静态齐全，理论不可达；防御性兜底
		slog.Error("build contrib menu service failed", "error", err)
		panic(err)
	}
	return svc
}

// pluckAccountRoleIds 账号 → 绑定角色 id 集合（t_r_user_role）。
func pluckAccountRoleIds(db *gorm.DB) func(ctx context.Context, accountId int64) ([]int64, error) {
	return func(ctx context.Context, accountId int64) ([]int64, error) {
		var roleIds []int64
		err := db.WithContext(ctx).Model(model.RUserRole{}).
			Where(model.RUserRoleColumnUserId, accountId).
			Pluck(model.RUserRoleColumnRoleId, &roleIds).Error
		return roleIds, err
	}
}

// roleExists 角色存在性（t_r_role）。
func roleExists(db *gorm.DB) func(ctx context.Context, roleId int64) (bool, error) {
	return func(ctx context.Context, roleId int64) (bool, error) {
		var count int64
		err := db.WithContext(ctx).Model(model.Role{}).
			Where(model.RoleColumnId, roleId).
			Count(&count).Error
		return count > 0, err
	}
}

// validateMenuPermissionCode 菜单绑码的 iam 规则：格式（点式三段）+
// 服务段隔离（码第一段 = 所属应用注册的服务段）。
func validateMenuPermissionCode(db *gorm.DB) func(ctx context.Context, appId int64, code string) error {
	return func(ctx context.Context, appId int64, code string) error {
		if !model.IsValidPermissionCode(code) {
			return ErrInvalidPermissionCode
		}
		return requireCodeSegmentMatchesApp(ctx, db, appId, code)
	}
}

