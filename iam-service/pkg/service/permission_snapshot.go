package service

import (
	"context"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	"gorm.io/gorm"
)

// NewPermissionSnapshotLoader 返回从 DB 重建账号权限快照的回源函数：
// 计算用户启用角色下启用权限的权限码并集（权限码是唯一授权词汇，011 裁决）。
func NewPermissionSnapshotLoader(db *gorm.DB) auth.PermissionLoader {
	return func(ctx context.Context, accountId int64) ([]string, error) {
		var user model.User
		err := db.WithContext(ctx).
			Select(model.UserFields).
			Where(model.UserColumnId, accountId).
			First(&user).Error
		if err != nil {
			return nil, err
		}
		if err := loadUserPermissionGraph(ctx, db, &user); err != nil {
			return nil, err
		}
		return collectUserPermissionCodes(user), nil
	}
}

// collectUserPermissionCodes 计算用户绑定角色的直授权限码并集。
// 唯一来源：角色直授权的启用权限码（t_r_role_permission）。
//
// 菜单不再产码（2026-11-17 裁决 B，ADR 0002：菜单回归纯展示配置，
// 授权仅角色直授；t_menu.permission_code 语义为前端展示门槛，不参与授权）。
// 仅统计启用角色下的启用权限；停用角色/停用权限不产码。
func collectUserPermissionCodes(user model.User) []string {
	codeDict := make(map[string]struct{})
	for _, role := range user.RoleList {
		if role.Status != model.StatusEnabled {
			continue
		}
		for _, permission := range role.PermissionList {
			if permission.Status != model.StatusEnabled {
				continue
			}
			codeDict[permission.Code] = struct{}{}
		}
	}
	codes := make([]string, 0, len(codeDict))
	for code := range codeDict {
		codes = append(codes, code)
	}
	return codes
}
