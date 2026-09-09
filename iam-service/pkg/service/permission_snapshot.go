package service

import (
	"context"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	"gorm.io/gorm"
)

// NewPermissionSnapshotLoader 返回从 DB 重建账号权限快照的回源函数：
// 计算账号启用角色下启用权限的资源键并集（带类型前缀，research.md D5）。
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
		return collectAccountResourceKeys(account), nil
	}
}

// collectAccountResourceKeys 计算账号绑定角色的资源键并集（specs/005-typed-permission-model FR-009）：
//   - menu 类型权限     → "menu:<路由键>"
//   - http_api 类型权限 → "http:<rpc_full_method>"
//   - grpc 类型权限     → "rpc:<full_method>"
//
// 仅统计启用角色下的启用权限；停用角色/权限不产生资源键（FR-011）。
func collectAccountResourceKeys(account model.Account) []string {
	keyDict := make(map[string]struct{})
	for _, role := range account.RoleList {
		if role.Disabled {
			continue
		}
		for _, permission := range role.PermissionList {
			if permission.Disabled {
				continue
			}
			for _, resource := range permission.ResourceList {
				switch resource.Kind {
				case model.PermissionKindMenu:
					keyDict[constant.SnapshotKeyPrefixMenu+resource.Key] = struct{}{}
				case model.PermissionKindHTTPAPI:
					if len(resource.RpcFullMethod) > 0 {
						keyDict[constant.SnapshotKeyPrefixHTTP+resource.RpcFullMethod] = struct{}{}
					}
				case model.PermissionKindGRPC:
					keyDict[constant.SnapshotKeyPrefixRPC+resource.Key] = struct{}{}
				}
			}
		}
	}
	keys := make([]string, 0, len(keyDict))
	for key := range keyDict {
		keys = append(keys, key)
	}
	return keys
}
