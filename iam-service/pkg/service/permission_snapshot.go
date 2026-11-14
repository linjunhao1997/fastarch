package service

import (
	"context"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	"gorm.io/gorm"
)

// NewPermissionSnapshotLoader 返回从 DB 重建账号权限快照的回源函数：
// 计算账号启用角色下启用权限的权限码并集（权限码是唯一授权词汇，011 裁决）。
func NewPermissionSnapshotLoader(db *gorm.DB) auth.PermissionLoader {
	return func(ctx context.Context, accountId int64) ([]string, error) {
		var account model.Account
		err := db.WithContext(ctx).
			Select(model.AccountFields).
			Where(model.AccountColumnId, accountId).
			First(&account).Error
		if err != nil {
			return nil, err
		}
		if err := loadAccountPermissionGraph(ctx, db, &account); err != nil {
			return nil, err
		}
		return collectAccountPermissionCodes(account), nil
	}
}

// collectAccountPermissionCodes 计算账号绑定角色的权限码并集。
// 仅统计启用角色下的启用权限；停用角色/权限不产生权限码。
func collectAccountPermissionCodes(account model.Account) []string {
	codeDict := make(map[string]struct{})
	for _, role := range account.RoleList {
		if role.Disabled {
			continue
		}
		for _, permission := range role.PermissionList {
			if permission.Disabled {
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
