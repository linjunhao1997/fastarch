package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

func (a *AuthorizationService) GetRole(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Role, error) {
	var role model.Role
	err := a.db.WithContext(ctx).
		Select(model.RoleFields).
		Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	if err := loadRolePermissions(ctx, a.db, &role); err != nil {
		return nil, err
	}
	return roleToPb(role), nil
}

func (a *AuthorizationService) ListRoles(ctx context.Context, request *iam_v1.ListRolesRequest) (*iam_v1.ListRolesResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.RoleColumnName + " LIKE ?",
		model.RoleColumnCode + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		return applyKeywordFilter(a.db.WithContext(ctx).Model(model.Role{}), keywordConditions, request.Keyword)
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var roles []model.Role
	err = buildQuery().
		Order(model.RoleColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	if err := loadRolePermissionsForRoles(ctx, a.db, roles); err != nil {
		return nil, err
	}
	roleList := make([]*iam_v1.Role, 0, len(roles))
	for _, r := range roles {
		roleList = append(roleList, roleToPb(r))
	}
	return &iam_v1.ListRolesResponse{
		Roles: roleList,
		Page:  helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a *AuthorizationService) UpdateRole(ctx context.Context, request *iam_v1.UpdateRoleRequest) (*emptypb.Empty, error) {
	var role model.Role
	err := a.db.WithContext(ctx).
		Select(model.RoleFields).
		Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	if err := loadRolePermissions(ctx, a.db, &role); err != nil {
		return nil, err
	}

	// 权限码校验：新增/移除清单中的码必须已存在
	permissionByCode := make(map[string]model.Permission)
	allCodes := make([]string, 0, len(request.AddedPermissionCodeList)+len(request.RemovedPermissionCodeList))
	allCodes = append(allCodes, request.AddedPermissionCodeList...)
	allCodes = append(allCodes, request.RemovedPermissionCodeList...)
	if len(allCodes) > 0 {
		var permissions []model.Permission
		err = a.db.WithContext(ctx).
			Where(model.PermissionColumnCode+" IN ?", allCodes).
			Find(&permissions).Error
		if err != nil {
			return nil, err
		}
		for _, p := range permissions {
			permissionByCode[p.Code] = p
		}
		for _, code := range request.AddedPermissionCodeList {
			if _, ok := permissionByCode[code]; !ok {
				slog.Warn("update role rejected: permission code not found",
					"role_id", request.Id, "permission_code", code)
				return nil, ErrPermissionNotFound
			}
		}
	}

	// 与当前绑定求差集，避免重复绑定/重复解绑
	boundPermissions := make(map[string]model.Permission, len(role.PermissionList))
	for _, permission := range role.PermissionList {
		boundPermissions[permission.Code] = *permission
	}
	addedPermissions := make([]model.Permission, 0, len(request.AddedPermissionCodeList))
	for _, code := range request.AddedPermissionCodeList {
		if _, exists := boundPermissions[code]; !exists {
			addedPermissions = append(addedPermissions, permissionByCode[code])
		}
	}
	removedPermissions := make([]model.Permission, 0, len(request.RemovedPermissionCodeList))
	for _, code := range request.RemovedPermissionCodeList {
		if permission, exists := boundPermissions[code]; exists {
			removedPermissions = append(removedPermissions, permission)
		}
	}

	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.RoleColumnName] = request.Name
	}
	if len(request.EnabledStatus) > 0 {
		updates[model.RoleColumnStatus] = request.EnabledStatus
	}
	updates[model.RoleColumnUpdateTime] = time.Now()
	// 审计字段服务端注入：仅更新修改人与修改时间
	updates[model.RoleColumnUpdatedBy] = cast.ToString(auth.GetAccountId(ctx))
	updates[model.RoleColumnVersion] = gorm.Expr(model.RoleColumnVersion + " + 1")

	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(model.Role{}).
			Where(model.RoleColumnId, request.Id).
			Where(model.RoleColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUpdateRoleBusy
		}
		association := tx.Model(&role).Association("PermissionList")
		// GORM Append 空切片仍会重复插入关联行，必须跳过空清单
		if len(addedPermissions) > 0 {
			if err := association.Append(addedPermissions); err != nil {
				return err
			}
		}
		if len(removedPermissions) > 0 {
			return association.Delete(removedPermissions)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrUpdateRoleBusy) {
			slog.Warn("update role rejected: version conflict", "role_id", request.Id, "version", request.Version)
			return nil, err
		}
		slog.Warn("update role failed", "role_id", request.Id, "version", request.Version, "error", err)
		return nil, err
	}
	// 角色状态/权限绑定变更立即生效：失效持有该角色账号的权限快照
	a.invalidateRoleAccountsSnapshot(ctx, request.Id)
	return &emptypb.Empty{}, nil
}

func (a *AuthorizationService) DeleteRole(ctx context.Context, request *iam_v1.DeleteRoleRequest) (*emptypb.Empty, error) {
	var role model.Role
	err := a.db.WithContext(ctx).
		Select(model.RoleFields).
		Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	// 管理员角色为系统内置，禁止删除
	if role.Code == model.RoleAdmin {
		slog.Warn("delete role rejected: admin role protected", "role_id", request.Id, "role_code", role.Code)
		return nil, ErrRoleAdminProtected
	}
	var count int64
	err = a.db.WithContext(ctx).Model(model.RUserRole{}).
		Where(model.RUserRoleColumnRoleId, request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check role user bindings failed", "role_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete role rejected: bound by users", "role_id", request.Id, "binding_count", count)
		return nil, ErrRoleReferenced
	}
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 级联清理角色-权限绑定（t_r_role_permission）
		if err := tx.Model(&role).Association("PermissionList").Clear(); err != nil {
			return err
		}
		// 级联清理角色-菜单绑定（t_r_role_menu）
		if err := tx.Model(&role).Association("MenuList").Clear(); err != nil {
			return err
		}
		return tx.Where(model.RoleColumnId, request.Id).Delete(model.Role{}).Error
	})
	if err != nil {
		slog.Warn("delete role failed", "role_id", request.Id, "error", err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (a *AuthorizationService) DeletePermission(ctx context.Context, request *iam_v1.DeletePermissionRequest) (*emptypb.Empty, error) {
	var count int64
	err := a.db.WithContext(ctx).Model(model.RRolePermission{}).
		Where(model.RRolePermissionColumnPermissionId, request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check permission role references failed", "permission_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete permission rejected: referenced by roles", "permission_id", request.Id, "reference_count", count)
		return nil, ErrPermissionReferenced
	}
	result := a.db.WithContext(ctx).
		Where(model.PermissionColumnId, request.Id).
		Delete(model.Permission{})
	if result.Error != nil {
		slog.Warn("delete permission failed", "permission_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete permission rejected: permission not found", "permission_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

// invalidateRoleAccountsSnapshot 删除持有指定角色的全部账号的权限快照键，
// 使角色状态/绑定变更立即生效（research.md D6-①）。
func (a *AuthorizationService) invalidateRoleAccountsSnapshot(ctx context.Context, roleId int64) {
	var userIds []int64
	err := a.db.Model(model.RUserRole{}).
		Where(model.RUserRoleColumnRoleId, roleId).
		Pluck(model.RUserRoleColumnUserId, &userIds).Error
	if err != nil {
		slog.Warn("pluck role user ids failed", "role_id", roleId, "error", err)
		return
	}
	a.deleteAccountPermissionSnapshot(ctx, userIds)
}

// invalidatePermissionAccountsSnapshot 删除引用指定权限的全部账号的权限快照键。
// 两张关联表无模型级关联，走原生 SQL JOIN 取 user_id。
func (a *AuthorizationService) invalidatePermissionAccountsSnapshot(ctx context.Context, permissionId int64) {
	var userIds []int64
	err := a.db.Raw(
		"SELECT DISTINCT ur.user_id FROM `t_r_user_role` ur "+
			"JOIN `t_r_role_permission` rp ON rp.role_id = ur.role_id "+
			"WHERE rp.permission_id = ?", permissionId).
		Scan(&userIds).Error
	if err != nil {
		slog.Warn("pluck permission user ids failed", "permission_id", permissionId, "error", err)
		return
	}
	a.deleteAccountPermissionSnapshot(ctx, userIds)
}

// deleteAccountPermissionSnapshot 批量删除权限快照键。快照为派生数据：
// 删除失败仅记日志，由短 TTL 兜底。
func (a *AuthorizationService) deleteAccountPermissionSnapshot(ctx context.Context, userIds []int64) {
	if a.rdb == nil {
		return
	}
	for _, userId := range userIds {
		if err := a.rdb.Del(ctx, constant.GetAccountPermissionKey(userId)).Err(); err != nil {
			slog.Warn("invalidate user permission snapshot failed", "user_id", userId, "error", err)
		}
	}
}
