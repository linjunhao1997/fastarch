package model

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	"gorm.io/gorm"

	"iam-service/pkg/constant"
)

const (
	BuiltinPermissionMenuAll = "builtin.menu.all"
	BuiltinPermissionHTTPAll = "builtin.http.all"
	BuiltinPermissionGRPCAll = "builtin.grpc.all"
)

func Seed(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		applicationId, err := seedApplication(tx)
		if err != nil {
			return err
		}
		if err := seedResources(tx, applicationId); err != nil {
			return err
		}
		if err := migrateLegacyAdminRole(tx); err != nil {
			return err
		}
		adminRole, err := seedAdminRole(tx, applicationId)
		if err != nil {
			return err
		}
		if _, err := seedBuiltinPermissions(tx, applicationId, adminRole.Id); err != nil {
			return err
		}
		if err := seedAccountApplications(tx); err != nil {
			return err
		}
		return nil
	})
}

// credentialSeedResources 是凭证域（specs/009-ak-sk-auth T006）的资源种子清单：
// 7 条 HTTP 路由 + 7 个 gRPC full method + 1 个菜单键。资源登记后由
// seedBuiltinPermissions 全量绑定到 builtin "*" 权限（管理员可见），
// 其余角色经资源管理按需授权。幂等：按 (application_id, kind, key) 补齐缺失行。
var credentialSeedResources = []Resource{
	// HTTP 路由资源（key = "<METHOD> <path>"，与 HTTP_ROUTES 镜像）
	{Kind: PermissionKindHTTPAPI, Key: "POST /authentication/access-credentials", Name: "创建访问凭证", RpcFullMethod: "/iam.v1.AccessCredentialService/CreateAccessCredential"},
	{Kind: PermissionKindHTTPAPI, Key: "GET /authentication/access-credentials", Name: "凭证列表", RpcFullMethod: "/iam.v1.AccessCredentialService/ListAccessCredentials"},
	{Kind: PermissionKindHTTPAPI, Key: "GET /authentication/access-credentials/{id}", Name: "凭证详情", RpcFullMethod: "/iam.v1.AccessCredentialService/GetAccessCredential"},
	{Kind: PermissionKindHTTPAPI, Key: "PATCH /authentication/access-credentials/{id}", Name: "编辑凭证", RpcFullMethod: "/iam.v1.AccessCredentialService/UpdateAccessCredential"},
	{Kind: PermissionKindHTTPAPI, Key: "POST /authentication/access-credentials/{id}/status", Name: "启用/禁用凭证", RpcFullMethod: "/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus"},
	{Kind: PermissionKindHTTPAPI, Key: "POST /authentication/access-credentials/{id}/reset-secret", Name: "重置凭证密钥", RpcFullMethod: "/iam.v1.AccessCredentialService/ResetAccessCredentialSecret"},
	{Kind: PermissionKindHTTPAPI, Key: "DELETE /authentication/access-credentials/{id}", Name: "删除凭证", RpcFullMethod: "/iam.v1.AccessCredentialService/DeleteAccessCredential"},
	// gRPC 资源（key = full method，与 GRPC_FULL_METHODS 镜像）
	{Kind: PermissionKindGRPC, Key: "/iam.v1.AccessCredentialService/CreateAccessCredential", Name: "创建访问凭证"},
	{Kind: PermissionKindGRPC, Key: "/iam.v1.AccessCredentialService/ListAccessCredentials", Name: "凭证列表"},
	{Kind: PermissionKindGRPC, Key: "/iam.v1.AccessCredentialService/GetAccessCredential", Name: "凭证详情"},
	{Kind: PermissionKindGRPC, Key: "/iam.v1.AccessCredentialService/UpdateAccessCredential", Name: "编辑凭证"},
	{Kind: PermissionKindGRPC, Key: "/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus", Name: "启用/禁用凭证"},
	{Kind: PermissionKindGRPC, Key: "/iam.v1.AccessCredentialService/ResetAccessCredentialSecret", Name: "重置凭证密钥"},
	{Kind: PermissionKindGRPC, Key: "/iam.v1.AccessCredentialService/DeleteAccessCredential", Name: "删除凭证"},
	// 菜单资源（key 与 MENU_KEYS / 前端 handle.key 镜像）
	{Kind: PermissionKindMenu, Key: "access_credential_list", Name: "访问凭证"},
}

func seedResources(tx *gorm.DB, applicationId int64) error {
	keys := make([]string, 0, len(credentialSeedResources))
	for _, item := range credentialSeedResources {
		keys = append(keys, item.Key)
	}
	var existing []Resource
	if err := tx.Select(ResourceFields).
		Where(ResourceColumnApplicationId, applicationId).
		// key 为 MySQL 保留字，显式 SQL 片段必须反引号（gorm 仅对 Eq 形式自动引用）
		Where("`key` IN ?", keys).
		Find(&existing).Error; err != nil {
		return err
	}
	existingKeys := make(map[string]struct{}, len(existing))
	for _, row := range existing {
		existingKeys[row.Kind+"|"+row.Key] = struct{}{}
	}

	now := time.Now()
	missing := make([]Resource, 0, len(credentialSeedResources))
	for _, item := range credentialSeedResources {
		if _, ok := existingKeys[item.Kind+"|"+item.Key]; ok {
			continue
		}
		missing = append(missing, Resource{
			Id:            krand.NewID(),
			ApplicationId: applicationId,
			Kind:          item.Kind,
			Key:           item.Key,
			Name:          item.Name,
			RpcFullMethod: item.RpcFullMethod,
			CreateTime:    now,
			UpdateTime:    now,
			Version:       1,
		})
	}
	if len(missing) == 0 {
		return nil
	}
	if err := tx.Create(&missing).Error; err != nil {
		return err
	}
	slog.Info("seed credential resources", "application_id", applicationId, "created", len(missing))
	return nil
}

// migrateLegacyAdminRole 将历史 code="admin" 角色迁移为 RoleAdmin。
// 历史角色原地改名（保留 id 与账号绑定）；若已存在重复的 RoleAdmin 角色，
// 先把其账号/权限绑定并入历史角色并删除重复行，否则改名会撞 t_role.code 唯一索引。
func migrateLegacyAdminRole(tx *gorm.DB) error {
	var legacyRoles []Role
	if err := tx.Where("code = ?", RoleAdminLegacy).Find(&legacyRoles).Error; err != nil {
		return err
	}
	if len(legacyRoles) == 0 {
		return nil
	}

	target := legacyRoles[0]
	var duplicate Role
	err := tx.Where("code = ?", RoleAdmin).First(&duplicate).Error
	switch {
	case err == nil && duplicate.Id != target.Id:
		if err := mergeRoleBindings(tx, []int64{duplicate.Id}, target.Id); err != nil {
			return err
		}
		if err := tx.Delete(&Role{}, duplicate.Id).Error; err != nil {
			return err
		}
		slog.Info("merged duplicate admin role into legacy", "merged_id", duplicate.Id, "into_id", target.Id)
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		return err
	}

	// 防御：历史角色多于一个时，其余同样并入 target 后删除
	if len(legacyRoles) > 1 {
		extraIds := make([]int64, 0, len(legacyRoles)-1)
		for _, other := range legacyRoles[1:] {
			extraIds = append(extraIds, other.Id)
		}
		if err := mergeRoleBindings(tx, extraIds, target.Id); err != nil {
			return err
		}
		if err := tx.Where("id IN ?", extraIds).Delete(&Role{}).Error; err != nil {
			return err
		}
	}

	now := time.Now()
	result := tx.Model(&Role{}).
		Where("id = ?", target.Id).
		Updates(map[string]interface{}{
			"code":        RoleAdmin,
			"update_time": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		slog.Info("migrated legacy admin role", "from", RoleAdminLegacy, "to", RoleAdmin)
	}
	return nil
}

// mergeRoleBindings 把 from 角色的账号/权限绑定并入 to 角色。
// 集合式操作（宪法禁止循环访问数据库）：先删 to 已有的重复绑定
//（撞复合唯一索引），再把剩余改绑到 to。
func mergeRoleBindings(tx *gorm.DB, fromRoleIds []int64, toRoleId int64) error {
	var conflictingAccountIds []int64
	if err := tx.Raw(
		"SELECT account_id FROM t_r_account_role WHERE role_id IN ? AND account_id IN (SELECT account_id FROM t_r_account_role WHERE role_id = ?)",
		fromRoleIds, toRoleId,
	).Scan(&conflictingAccountIds).Error; err != nil {
		return err
	}
	if err := tx.Where("role_id IN ? AND account_id IN ?", fromRoleIds, conflictingAccountIds).
		Delete(&RAccountRole{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&RAccountRole{}).
		Where("role_id IN ?", fromRoleIds).
		Update("role_id", toRoleId).Error; err != nil {
		return err
	}

	var conflictingPermissionIds []int64
	if err := tx.Raw(
		"SELECT permission_id FROM t_r_role_permission WHERE role_id IN ? AND permission_id IN (SELECT permission_id FROM t_r_role_permission WHERE role_id = ?)",
		fromRoleIds, toRoleId,
	).Scan(&conflictingPermissionIds).Error; err != nil {
		return err
	}
	if err := tx.Where("role_id IN ? AND permission_id IN ?", fromRoleIds, conflictingPermissionIds).
		Delete(&RRolePermission{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&RRolePermission{}).
		Where("role_id IN ?", fromRoleIds).
		Update("role_id", toRoleId).Error; err != nil {
		return err
	}
	return nil
}

func seedApplication(tx *gorm.DB) (int64, error) {
	var existing int64
	if err := tx.Model(&Application{}).Count(&existing).Error; err != nil {
		return 0, err
	}
	if existing > 0 {
		var app Application
		if err := tx.First(&app).Error; err != nil {
			return 0, err
		}
		return app.Id, nil
	}
	pkId := krand.NewID()
	now := time.Now()
	app := Application{
		Id:         pkId,
		Code:       constant.ApplicationCode,
		Name:       "IAM",
		Disabled:   0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := tx.Create(&app).Error; err != nil {
		return 0, err
	}
	slog.Info("seed application", "id", app.Id, "code", app.Code)
	return app.Id, nil
}

func seedAdminRole(tx *gorm.DB, applicationId int64) (*Role, error) {
	var existing Role
	err := tx.Where("code = ?", RoleAdmin).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	now := time.Now()
	role := Role{
		Id:            krand.NewID(),
		Name:          "Administrator",
		Code:          RoleAdmin,
		ApplicationId: applicationId,
		Disabled:      false,
		CreateTime:    now,
		UpdateTime:    now,
		Version:       1,
	}
	if err := tx.Create(&role).Error; err != nil {
		return nil, err
	}
	slog.Info("seed admin role", "role_id", role.Id, "code", RoleAdmin)
	return &role, nil
}

func seedBuiltinPermissions(tx *gorm.DB, applicationId int64, adminRoleId int64) (int64, error) {
	now := time.Now()
	permissions := []Permission{
		{Code: BuiltinPermissionMenuAll, Name: "All Menu Access", Value: "*", Kind: PermissionKindMenu, Description: "Full access to all menus", ApplicationId: applicationId, CreateTime: now, UpdateTime: now},
		{Code: BuiltinPermissionHTTPAll, Name: "All HTTP Access", Value: "*", Kind: PermissionKindHTTPAPI, Description: "Full access to all HTTP endpoints", ApplicationId: applicationId, CreateTime: now, UpdateTime: now},
		{Code: BuiltinPermissionGRPCAll, Name: "All gRPC Access", Value: "*", Kind: PermissionKindGRPC, Description: "Full access to all gRPC methods", ApplicationId: applicationId, CreateTime: now, UpdateTime: now},
	}
	created := 0
	// 集合式查询已有权限（宪法禁止循环访问数据库）。
	// 也不能在结构体上预填 Id：GORM 的 FirstOrCreate/Find 会把非零主键
	// 追加为查询条件，导致永远查不到已有记录，INSERT 时撞 code 唯一索引。
	codes := make([]string, 0, len(permissions))
	for i := range permissions {
		codes = append(codes, permissions[i].Code)
	}
	var existingPermissions []Permission
	if err := tx.Where("code IN ?", codes).Find(&existingPermissions).Error; err != nil {
		return 0, err
	}
	existingByCode := make(map[string]Permission, len(existingPermissions))
	for _, existing := range existingPermissions {
		existingByCode[existing.Code] = existing
	}
	for i := range permissions {
		if existing, ok := existingByCode[permissions[i].Code]; ok {
			permissions[i] = existing
			continue
		}
		permissions[i].Id = krand.NewID()
		if err := tx.Create(&permissions[i]).Error; err != nil {
			return 0, err
		}
		created++
	}

	resourceIdsByKind := map[string][]int64{
		PermissionKindMenu:    {},
		PermissionKindHTTPAPI: {},
		PermissionKindGRPC:    {},
	}
	var resources []Resource
	if err := tx.Find(&resources).Error; err != nil {
		return 0, err
	}
	for _, r := range resources {
		resourceIdsByKind[r.Kind] = append(resourceIdsByKind[r.Kind], r.Id)
	}

	bindingSet := map[string]struct{}{}
	var existingBindings []PermissionResource
	if err := tx.Find(&existingBindings).Error; err != nil {
		return 0, err
	}
	for _, b := range existingBindings {
		bindingSet[permResourceKey(b.PermissionId, b.ResourceId)] = struct{}{}
	}

	missingBindings := make([]PermissionResource, 0)
	for _, permission := range permissions {
		for _, resourceId := range resourceIdsByKind[permission.Kind] {
			key := permResourceKey(permission.Id, resourceId)
			if _, ok := bindingSet[key]; ok {
				continue
			}
			missingBindings = append(missingBindings, PermissionResource{PermissionId: permission.Id, ResourceId: resourceId})
			bindingSet[key] = struct{}{}
		}
	}
	if len(missingBindings) > 0 {
		if err := tx.CreateInBatches(missingBindings, 100).Error; err != nil {
			return 0, err
		}
	}

	var grantedPermissionIds []int64
	if err := tx.Raw(
		"SELECT permission_id FROM t_r_role_permission WHERE role_id = ?", adminRoleId,
	).Scan(&grantedPermissionIds).Error; err != nil {
		return 0, err
	}
	grantedSet := make(map[int64]struct{}, len(grantedPermissionIds))
	for _, permissionId := range grantedPermissionIds {
		grantedSet[permissionId] = struct{}{}
	}
	grantedNew := 0
	newGrants := make([]map[string]interface{}, 0)
	for _, permission := range permissions {
		if _, ok := grantedSet[permission.Id]; ok {
			continue
		}
		newGrants = append(newGrants, map[string]interface{}{
			"role_id":       adminRoleId,
			"permission_id": permission.Id,
		})
		grantedNew++
	}
	if len(newGrants) > 0 {
		if err := tx.Table("t_r_role_permission").Create(&newGrants).Error; err != nil {
			return 0, err
		}
	}

	if created > 0 || len(missingBindings) > 0 || grantedNew > 0 {
		slog.Info("seed builtin permissions",
			"created", created, "bindings", len(missingBindings), "granted_to_admin", grantedNew)
	}
	return int64(created), nil
}

func permResourceKey(permissionId, resourceId int64) string {
	return strconv.FormatInt(permissionId, 10) + ":" + strconv.FormatInt(resourceId, 10)
}

func seedAccountApplications(tx *gorm.DB) error {
	var pairs []RAccountApplication
	err := tx.Table("t_r_account_role AS ar").
		Select("DISTINCT ar.account_id AS account_id, r.application_id AS application_id").
		Joins("JOIN t_role AS r ON r.id = ar.role_id AND r.disabled = ?", false).
		Scan(&pairs).Error
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return nil
	}

	var existing []RAccountApplication
	if err := tx.Select(RAccountApplicationFields).Find(&existing).Error; err != nil {
		return err
	}
	existingSet := make(map[string]struct{}, len(existing))
	for _, row := range existing {
		existingSet[permResourceKey(row.AccountId, row.ApplicationId)] = struct{}{}
	}

	missing := make([]RAccountApplication, 0, len(pairs))
	for _, pair := range pairs {
		if pair.ApplicationId == 0 || pair.AccountId == 0 {
			continue
		}
		if _, ok := existingSet[permResourceKey(pair.AccountId, pair.ApplicationId)]; ok {
			continue
		}
		existingSet[permResourceKey(pair.AccountId, pair.ApplicationId)] = struct{}{}
		missing = append(missing, RAccountApplication{
			AccountId:     pair.AccountId,
			ApplicationId: pair.ApplicationId,
		})
	}
	if len(missing) == 0 {
		return nil
	}
	if err := tx.Create(&missing).Error; err != nil {
		return err
	}
	slog.Info("seed account applications from roles",
		"initialized", len(missing))
	return nil
}