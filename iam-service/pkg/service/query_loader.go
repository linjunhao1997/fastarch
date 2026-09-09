package service

import (
	"context"

	"iam-service/pkg/model"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"gorm.io/gorm"
)

func loadAccountRoles(ctx context.Context, db *gorm.DB, account *model.Account) error {
	var bindings []model.RAccountRole
	if err := db.WithContext(ctx).
		Where(model.RAccountRoleColumnAccountId, account.Id).
		Select(model.RAccountRoleFields).
		Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		account.RoleList = []*model.Role{}
		return nil
	}
	roleIds := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		roleIds = append(roleIds, binding.RoleId)
	}
	var roles []*model.Role
	if err := db.WithContext(ctx).
		Where(model.RoleColumnId+" IN ?", roleIds).
		Select(model.RoleFields).
		Find(&roles).Error; err != nil {
		return err
	}
	roleById := make(map[int64]*model.Role, len(roles))
	for _, role := range roles {
		roleById[role.Id] = role
	}
	account.RoleList = make([]*model.Role, 0, len(bindings))
	for _, binding := range bindings {
		if role, exists := roleById[binding.RoleId]; exists {
			account.RoleList = append(account.RoleList, role)
		}
	}
	return nil
}

func loadRolePermissions(ctx context.Context, db *gorm.DB, role *model.Role) error {
	var bindings []rolePermissionBinding
	if err := db.WithContext(ctx).
		Table("t_r_role_permission").
		Where(model.RRolePermissionColumnRoleId, role.Id).
		Select([]string{model.RRolePermissionColumnRoleId, model.RRolePermissionColumnPermissionId}).
		Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		role.PermissionList = []*model.Permission{}
		return nil
	}
	permissionIds := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		permissionIds = append(permissionIds, binding.PermissionId)
	}
	var permissions []*model.Permission
	if err := db.WithContext(ctx).
		Where(model.PermissionColumnId+" IN ?", permissionIds).
		Select(model.PermissionFields).
		Find(&permissions).Error; err != nil {
		return err
	}
	permissionById := make(map[int64]*model.Permission, len(permissions))
	for _, permission := range permissions {
		permissionById[permission.Id] = permission
	}
	role.PermissionList = make([]*model.Permission, 0, len(bindings))
	for _, binding := range bindings {
		if permission, exists := permissionById[binding.PermissionId]; exists {
			role.PermissionList = append(role.PermissionList, permission)
		}
	}
	return nil
}

func loadPermissionResources(ctx context.Context, db *gorm.DB, permission *model.Permission) error {
	var bindings []permissionResourceBinding
	if err := db.WithContext(ctx).
		Table("t_r_permission_resource").
		Where(model.PermissionResourceColumnPermissionId, permission.Id).
		Select([]string{model.PermissionResourceColumnPermissionId, model.PermissionResourceColumnResourceId}).
		Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		permission.ResourceList = []*model.Resource{}
		return nil
	}
	resourceIds := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		resourceIds = append(resourceIds, binding.ResourceId)
	}
	var resources []*model.Resource
	if err := db.WithContext(ctx).
		Where(model.ResourceColumnId+" IN ?", resourceIds).
		Select(model.ResourceFields).
		Find(&resources).Error; err != nil {
		return err
	}
	resourceById := make(map[int64]*model.Resource, len(resources))
	for _, resource := range resources {
		resourceById[resource.Id] = resource
	}
	permission.ResourceList = make([]*model.Resource, 0, len(bindings))
	for _, binding := range bindings {
		if resource, exists := resourceById[binding.ResourceId]; exists {
			permission.ResourceList = append(permission.ResourceList, resource)
		}
	}
	return nil
}

func loadAccountPermissionGraph(ctx context.Context, db *gorm.DB, account *model.Account) error {
	if err := loadAccountRoles(ctx, db, account); err != nil {
		return err
	}
	if len(account.RoleList) == 0 {
		return nil
	}
	roleIds := make([]int64, 0, len(account.RoleList))
	for _, role := range account.RoleList {
		roleIds = append(roleIds, role.Id)
	}
	var roleBindings []rolePermissionBinding
	if err := db.WithContext(ctx).
		Table("t_r_role_permission").
		Where(model.RRolePermissionColumnRoleId+" IN ?", roleIds).
		Select([]string{model.RRolePermissionColumnRoleId, model.RRolePermissionColumnPermissionId}).
		Find(&roleBindings).Error; err != nil {
		return err
	}
	permissionIds := make([]int64, 0, len(roleBindings))
	for _, binding := range roleBindings {
		permissionIds = append(permissionIds, binding.PermissionId)
	}
	var permissions []*model.Permission
	if err := db.WithContext(ctx).
		Where(model.PermissionColumnId+" IN ?", permissionIds).
		Select(model.PermissionFields).
		Find(&permissions).Error; err != nil {
		return err
	}
	permissionById := make(map[int64]*model.Permission, len(permissions))
	for _, permission := range permissions {
		permissionById[permission.Id] = permission
	}
	var permissionBindings []permissionResourceBinding
	if err := db.WithContext(ctx).
		Table("t_r_permission_resource").
		Where(model.PermissionResourceColumnPermissionId+" IN ?", permissionIds).
		Select([]string{model.PermissionResourceColumnPermissionId, model.PermissionResourceColumnResourceId}).
		Find(&permissionBindings).Error; err != nil {
		return err
	}
	resourceIds := make([]int64, 0, len(permissionBindings))
	for _, binding := range permissionBindings {
		resourceIds = append(resourceIds, binding.ResourceId)
	}
	var resources []*model.Resource
	if err := db.WithContext(ctx).
		Where(model.ResourceColumnId+" IN ?", resourceIds).
		Select(model.ResourceFields).
		Find(&resources).Error; err != nil {
		return err
	}
	resourceById := make(map[int64]*model.Resource, len(resources))
	for _, resource := range resources {
		resourceById[resource.Id] = resource
	}
	permissionsByRole := make(map[int64][]*model.Permission)
	for _, binding := range roleBindings {
		if permission, exists := permissionById[binding.PermissionId]; exists {
			permissionsByRole[binding.RoleId] = append(permissionsByRole[binding.RoleId], permission)
		}
	}
	resourcesByPermission := make(map[int64][]*model.Resource)
	for _, binding := range permissionBindings {
		if resource, exists := resourceById[binding.ResourceId]; exists {
			resourcesByPermission[binding.PermissionId] = append(resourcesByPermission[binding.PermissionId], resource)
		}
	}
	for _, role := range account.RoleList {
		role.PermissionList = permissionsByRole[role.Id]
	}
	for _, permission := range permissions {
		permission.ResourceList = resourcesByPermission[permission.Id]
	}
	return nil
}

func loadRolePermissionsForRoles(ctx context.Context, db *gorm.DB, roles []model.Role) error {
	if len(roles) == 0 {
		return nil
	}
	roleIds := make([]int64, 0, len(roles))
	for _, role := range roles {
		roleIds = append(roleIds, role.Id)
	}
	var bindings []model.RRolePermission
	if err := db.WithContext(ctx).
		Where(model.RRolePermissionColumnRoleId+" IN ?", roleIds).
		Select(model.RRolePermissionFields).
		Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		for index := range roles {
			roles[index].PermissionList = []*model.Permission{}
		}
		return nil
	}
	permissionIds := make([]int64, 0, len(bindings))
	bindingSeen := make(map[int64]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, exists := bindingSeen[binding.PermissionId]; exists {
			continue
		}
		bindingSeen[binding.PermissionId] = struct{}{}
		permissionIds = append(permissionIds, binding.PermissionId)
	}
	var permissions []*model.Permission
	if err := db.WithContext(ctx).
		Where(model.PermissionColumnId+" IN ?", permissionIds).
		Select(model.PermissionFields).
		Find(&permissions).Error; err != nil {
		return err
	}
	permissionById := make(map[int64]*model.Permission, len(permissions))
	for _, permission := range permissions {
		permissionById[permission.Id] = permission
	}
	permissionsByRole := make(map[int64][]*model.Permission)
	for _, binding := range bindings {
		if permission, exists := permissionById[binding.PermissionId]; exists {
			permissionsByRole[binding.RoleId] = append(permissionsByRole[binding.RoleId], permission)
		}
	}
	for index := range roles {
		roles[index].PermissionList = permissionsByRole[roles[index].Id]
	}
	return nil
}

type rolePermissionBinding struct {
	RoleId       int64
	PermissionId int64
}

type permissionResourceBinding struct {
	PermissionId int64
	ResourceId   int64
}

// loadApplicationSummaries batch-loads application summaries for the given IDs.
func loadApplicationSummaries(ctx context.Context, db *gorm.DB, ids []int64) (map[int64]*iam_v1.ApplicationSummary, error) {
	if len(ids) == 0 {
		return make(map[int64]*iam_v1.ApplicationSummary), nil
	}
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	var applications []model.Application
	if err := db.WithContext(ctx).
		Where(model.ApplicationColumnId+" IN ?", unique).
		Select(model.ApplicationFields).
		Find(&applications).Error; err != nil {
		return nil, err
	}
	result := make(map[int64]*iam_v1.ApplicationSummary, len(applications))
	for _, app := range applications {
		result[app.Id] = &iam_v1.ApplicationSummary{
			Id:            app.Id,
			Name:          app.Name,
			Code:          app.Code,
			EnabledStatus: func() string { if app.Disabled == 0 { return "enabled" }; return "disabled" }(),
			Description:   app.Description,
		}
	}
	return result, nil
}
