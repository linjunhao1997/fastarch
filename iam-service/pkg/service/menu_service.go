package service

import (
	"context"
	"log/slog"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	"github.com/linjunhao1997/contrib/pkg/menu"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// NewMenuService 构建通用菜单能力（contrib/pkg/menu，ADR 0001），注入 iam 宿主钩子：
//   - 应用上下文/账号/管理员身份取自 auth ctx（token 承载，多应用裁决 #8）；
//   - 角色绑定经 t_r_user_role / t_r_role_menu；
//   - 菜单绑码校验走 iam 权限词汇规则（格式 + 服务段隔离裁决）；
//   - 绑定变更后失效持有该角色用户的权限快照（菜单授权推导 #6 裁决）。
//
// 钩子在编译期静态齐全，NewMenuService 的校验错误不可达。
func NewMenuService(db *gorm.DB, rdb *redis.Client) *menu.MenuService {
	svc, err := menu.NewMenuService(menu.Config{
		DB:                       db,
		ApplicationIdFromContext: requireApplicationId,
		AccountIdFromContext:     auth.GetAccountId,
		IsAdminFromContext:       auth.IsAdminFromContext,
		RoleIdsForAccount:        pluckAccountRoleIds(db),
		RoleExists:               roleExists(db),
		ValidatePermissionCode:   validateMenuPermissionCode(db),
		OnRoleMenusChanged: func(ctx context.Context, roleId int64) {
			invalidateRoleUsersSnapshot(ctx, db, rdb, roleId)
		},
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

// invalidateRoleUsersSnapshot 删除持有指定角色的全部用户的权限快照键。
// 快照为派生数据：删除失败仅记日志，由短 TTL 兜底。
func invalidateRoleUsersSnapshot(ctx context.Context, db *gorm.DB, rdb *redis.Client, roleId int64) {
	if rdb == nil {
		return
	}
	var userIds []int64
	err := db.Model(model.RUserRole{}).
		Where(model.RUserRoleColumnRoleId, roleId).
		Pluck(model.RUserRoleColumnUserId, &userIds).Error
	if err != nil {
		slog.Warn("pluck role user ids failed", "role_id", roleId, "error", err)
		return
	}
	for _, userId := range userIds {
		if err := rdb.Del(ctx, constant.GetAccountPermissionKey(userId)).Err(); err != nil {
			slog.Warn("invalidate user permission snapshot failed", "user_id", userId, "error", err)
		}
	}
}
