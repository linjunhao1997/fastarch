package service

import (
	"context"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	"gorm.io/gorm"
)

// NewPermissionSnapshotLoader 返回从 DB 重建账号权限快照的回源函数：
// 查询账号绑定角色的权限编码并集，语义与登录写入快照一致（research.md D6-②）。
func NewPermissionSnapshotLoader(db *gorm.DB) auth.PermissionLoader {
	return func(ctx context.Context, accountId int64) ([]string, error) {
		var account model.Account
		err := db.WithContext(ctx).
			Preload("RoleList.PermissionList").
			Preload("RoleList.Application").
			Where(model.AccountColumnId, accountId).
			First(&account).Error
		if err != nil {
			return nil, err
		}
		return collectAccountPermissionCodes(account), nil
	}
}

// collectAccountPermissionCodes 计算账号绑定角色的权限编码并集：
// admin 角色持有 admin 权限，普通角色取"<应用名>:<权限编码>"。
func collectAccountPermissionCodes(account model.Account) []string {
	codeDict := make(map[string]struct{})
	for _, role := range account.RoleList {
		if role.Code == "admin" {
			codeDict[constant.ApplicationCode+":admin"] = struct{}{}
			continue
		}
		for _, permission := range role.PermissionList {
			codeDict[role.Application.Name+":"+permission.Code] = struct{}{}
		}
	}
	codes := make([]string, 0, len(codeDict))
	for code := range codeDict {
		codes = append(codes, code)
	}
	return codes
}
