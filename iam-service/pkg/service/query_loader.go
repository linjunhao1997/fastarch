package service

import (
	"context"

	"iam-service/pkg/model"

	contribmodel "github.com/linjunhao1997/contrib/model"
	"gorm.io/gorm"
)

// loadUsersApplications 批量查询用户准入的应用（t_r_user_application → t_application），
// 按用户分组返回应用模型；无准入的用户得到空切片（非 nil）。
// 两次 IN 查询（绑定 + 应用行），供详情/列表回填避免 N+1；
// 绑定指向已删除应用时跳过（fail-safe，不产出幽灵应用）。
func loadUsersApplications(ctx context.Context, db *gorm.DB, users []model.User) (map[int64][]*model.Application, error) {
	appsByUser := make(map[int64][]*model.Application, len(users))
	if len(users) == 0 {
		return appsByUser, nil
	}
	userIds := make([]int64, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
		appsByUser[user.Id] = []*model.Application{}
	}
	var bindings []model.RUserApplication
	if err := db.WithContext(ctx).
		Where(model.RUserApplicationColumnUserId+" IN ?", userIds).
		Select(model.RUserApplicationFields).
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return appsByUser, nil
	}
	appIds := make([]int64, 0, len(bindings))
	seen := make(map[int64]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, exists := seen[binding.ApplicationId]; exists {
			continue
		}
		seen[binding.ApplicationId] = struct{}{}
		appIds = append(appIds, binding.ApplicationId)
	}
	var apps []*model.Application
	if err := db.WithContext(ctx).
		Where(model.ApplicationColumnId+" IN ?", appIds).
		Select(model.ApplicationFields).
		Find(&apps).Error; err != nil {
		return nil, err
	}
	appById := make(map[int64]*model.Application, len(apps))
	for _, app := range apps {
		appById[app.Id] = app
	}
	for _, binding := range bindings {
		if app, exists := appById[binding.ApplicationId]; exists {
			appsByUser[binding.UserId] = append(appsByUser[binding.UserId], app)
		}
	}
	return appsByUser, nil
}

// loadUserRoles 加载用户的角色列表（经 t_r_user_role 关联，按绑定顺序）。
func loadUserRoles(ctx context.Context, db *gorm.DB, user *model.User) error {
	var bindings []model.RUserRole
	if err := db.WithContext(ctx).
		Where(model.RUserRoleColumnUserId, user.Id).
		Select(model.RUserRoleFields).
		Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		user.RoleList = []*model.Role{}
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
	user.RoleList = make([]*model.Role, 0, len(bindings))
	for _, binding := range bindings {
		if role, exists := roleById[binding.RoleId]; exists {
			user.RoleList = append(user.RoleList, role)
		}
	}
	return nil
}

func loadRolePermissions(ctx context.Context, db *gorm.DB, role *model.Role) error {
	var bindings []model.RRolePermission
	if err := db.WithContext(ctx).
		Where(model.RRolePermissionColumnRoleId, role.Id).
		Select(model.RRolePermissionFields).
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

// loadUserPermissionGraph 一次批量加载用户的全部角色及其权限集合。
func loadUserPermissionGraph(ctx context.Context, db *gorm.DB, user *model.User) error {
	if err := loadUserRoles(ctx, db, user); err != nil {
		return err
	}
	if len(user.RoleList) == 0 {
		return nil
	}
	roleIds := make([]int64, 0, len(user.RoleList))
	for _, role := range user.RoleList {
		roleIds = append(roleIds, role.Id)
	}
	var bindings []model.RRolePermission
	if err := db.WithContext(ctx).
		Where(model.RRolePermissionColumnRoleId+" IN ?", roleIds).
		Select(model.RRolePermissionFields).
		Find(&bindings).Error; err != nil {
		return err
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
	for _, role := range user.RoleList {
		role.PermissionList = permissionsByRole[role.Id]
		if role.PermissionList == nil {
			role.PermissionList = []*model.Permission{}
		}
	}
	// 菜单不再参与授权推导（ADR 0002）：快照仅取角色直授码，
	// 故此处不加载角色绑定菜单（菜单可见性由 my-menu-tree 独立读取）。
	return nil
}

// loadRolePermissionsForRoles 批量加载角色集合的权限列表（角色列表页使用）。
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

// loadUsersRoles 批量加载多个用户的角色列表（列表页组装，禁 N+1，
// 绕开 Preload 生成 SELECT *）。
func loadUsersRoles(ctx context.Context, db *gorm.DB, users []model.User) error {
	if len(users) == 0 {
		return nil
	}
	userIds := make([]int64, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}
	var bindings []model.RUserRole
	if err := db.WithContext(ctx).
		Where(model.RUserRoleColumnUserId+" IN ?", userIds).
		Select(model.RUserRoleFields).
		Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		for index := range users {
			users[index].RoleList = []*model.Role{}
		}
		return nil
	}
	roleIds := make([]int64, 0, len(bindings))
	bindingSeen := make(map[int64]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, exists := bindingSeen[binding.RoleId]; exists {
			continue
		}
		bindingSeen[binding.RoleId] = struct{}{}
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
	rolesByUser := make(map[int64][]*model.Role, len(users))
	for _, binding := range bindings {
		if role, exists := roleById[binding.RoleId]; exists {
			rolesByUser[binding.UserId] = append(rolesByUser[binding.UserId], role)
		}
	}
	for index := range users {
		users[index].RoleList = rolesByUser[users[index].Id]
	}
	return nil
}

// loadRoleMenus 加载角色的菜单列表（经 t_r_role_menu 关联，按绑定顺序）。
func loadRoleMenus(ctx context.Context, db *gorm.DB, role *model.Role) error {
	var bindings []contribmodel.RRoleMenu
	if err := db.WithContext(ctx).
		Where(contribmodel.RRoleMenuColumnRoleId, role.Id).
		Select(contribmodel.RRoleMenuFields).
		Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		role.MenuList = []*contribmodel.Menu{}
		return nil
	}
	menuIds := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		menuIds = append(menuIds, binding.MenuId)
	}
	var menus []*contribmodel.Menu
	if err := db.WithContext(ctx).
		Where(contribmodel.MenuColumnId+" IN ?", menuIds).
		Select(contribmodel.MenuFields).
		Find(&menus).Error; err != nil {
		return err
	}
	menuById := make(map[int64]*contribmodel.Menu, len(menus))
	for _, menu := range menus {
		menuById[menu.Id] = menu
	}
	role.MenuList = make([]*contribmodel.Menu, 0, len(bindings))
	for _, binding := range bindings {
		if menu, exists := menuById[binding.MenuId]; exists {
			role.MenuList = append(role.MenuList, menu)
		}
	}
	return nil
}

// loadAccessibleApplications 加载用户可进入的应用清单（多应用裁决 #8）：
// admin 角色旁路全部启用应用；其他用户按 t_r_user_application 准入绑定过滤。
// 仅返回启用状态应用；停用应用一律不可进入。
func loadAccessibleApplications(ctx context.Context, db *gorm.DB, userId int64, isAdmin bool) ([]model.Application, error) {
	var apps []model.Application
	if isAdmin {
		err := db.WithContext(ctx).
			Where(model.ApplicationColumnStatus, model.StatusEnabled).
			Select(model.ApplicationFields).
			Find(&apps).Error
		if err != nil {
			return nil, err
		}
		return apps, nil
	}
	var bindings []model.RUserApplication
	err := db.WithContext(ctx).
		Model(model.RUserApplication{}).
		Where(model.RUserApplicationColumnUserId, userId).
		Select(model.RUserApplicationFields).
		Find(&bindings).Error
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return []model.Application{}, nil
	}
	appIds := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		appIds = append(appIds, binding.ApplicationId)
	}
	err = db.WithContext(ctx).
		Where(model.ApplicationColumnId+" IN ?", appIds).
		Where(model.ApplicationColumnStatus, model.StatusEnabled).
		Select(model.ApplicationFields).
		Find(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}
