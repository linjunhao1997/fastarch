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

// collectUserPermissionCodes 计算用户绑定角色的权限码并集。
// 仅统计启用角色下的启用权限；停用角色/权限不产生权限码。
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
