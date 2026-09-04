package service

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/spf13/cast"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"gorm.io/gorm"

	"iam-service/pkg/constant"
	"iam-service/pkg/model"
)

func setupAuthorizationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	err = db.AutoMigrate(&model.Application{}, &model.Role{}, &model.Permission{}, &model.Account{}, &model.RAccountRole{})
	if err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func createTestRole(t *testing.T, db *gorm.DB, id int64, code string, version int64) model.Role {
	t.Helper()
	role := model.Role{
		Id:       id,
		Name:     "role",
		Code:     code,
		Version:  version,
		Disabled: false,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	return role
}

func createTestPermission(t *testing.T, db *gorm.DB, id int64, code string, version int64) model.Permission {
	t.Helper()
	permission := model.Permission{
		Id:       id,
		Name:     "permission",
		Code:     code,
		Version:  version,
		Disabled: false,
	}
	if err := db.Create(&permission).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}
	return permission
}

func TestCreateRoleCodeUnique(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestRole(t, db, 1, "role-code", 1)
	service := NewAuthorizationService(db, nil)

	_, err := service.CreateRole(context.Background(), &iam_v1.CreateRoleRequest{Name: "role", RoleCode: "role-code"})
	if err != ErrRoleCodeDuplicated {
		t.Fatalf("CreateRole() error = %v, want %v", err, ErrRoleCodeDuplicated)
	}
}

func TestUpdateRole(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	role := createTestRole(t, db, 1, "role-code", 2)
	permission := createTestPermission(t, db, 10, "permission-code", 1)
	service := NewAuthorizationService(db, nil)

	_, err := service.UpdateRole(context.Background(), &iam_v1.UpdateRoleRequest{
		Id:                      1,
		Name:                    "updated",
		Version:                 1,
		AddedPermissionCodeList: []string{permission.Code},
	})
	if err != ErrUpdateRoleBusy {
		t.Fatalf("UpdateRole() error = %v, want %v", err, ErrUpdateRoleBusy)
	}

	_, err = service.UpdateRole(context.Background(), &iam_v1.UpdateRoleRequest{
		Id:                      1,
		Name:                    "updated",
		Version:                 2,
		AddedPermissionCodeList: []string{"missing-permission"},
	})
	if err != ErrPermissionNotFound {
		t.Fatalf("UpdateRole() error = %v, want %v", err, ErrPermissionNotFound)
	}

	_, err = service.UpdateRole(context.Background(), &iam_v1.UpdateRoleRequest{
		Id:                      1,
		Name:                    "updated",
		Version:                 2,
		AddedPermissionCodeList: []string{permission.Code},
	})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}

	var got model.Role
	if err := db.Preload("PermissionList").First(&got, role.Id).Error; err != nil {
		t.Fatalf("get role: %v", err)
	}
	if got.Version != 3 || got.Name != "updated" {
		t.Fatalf("role version/name = %d/%q, want 3/updated", got.Version, got.Name)
	}
	if len(got.PermissionList) != 1 || got.PermissionList[0].Code != permission.Code {
		t.Fatalf("permissions = %+v, want %q", got.PermissionList, permission.Code)
	}
}

func TestUpdatePermission(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	permission := createTestPermission(t, db, 1, "permission-code", 1)
	service := NewAuthorizationService(db, nil)

	_, err := service.CreatePermission(context.Background(), &iam_v1.CreatePermissionRequest{
		ApplicationId: 1,
		Name:          "duplicate",
		Code:          permission.Code,
	})
	if err != ErrPermissionCodeDuplicated {
		t.Fatalf("CreatePermission() error = %v, want %v", err, ErrPermissionCodeDuplicated)
	}

	_, err = service.UpdatePermission(context.Background(), &iam_v1.UpdatePermissionRequest{
		Id:      1,
		Name:    "updated",
		Version: 2,
	})
	if err != ErrUpdatePermissionBusy {
		t.Fatalf("UpdatePermission() error = %v, want %v", err, ErrUpdatePermissionBusy)
	}

	_, err = service.UpdatePermission(context.Background(), &iam_v1.UpdatePermissionRequest{
		Id:      1,
		Name:    "updated",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("UpdatePermission() error = %v", err)
	}

	var got model.Permission
	if err := db.First(&got, permission.Id).Error; err != nil {
		t.Fatalf("get permission: %v", err)
	}
	if got.Version != 2 || got.Name != "updated" {
		t.Fatalf("permission version/name = %d/%q, want 2/updated", got.Version, got.Name)
	}
}

func TestDeletePermission(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	role := createTestRole(t, db, 1, "role-code", 1)
	referenced := createTestPermission(t, db, 10, "referenced", 1)
	unreferenced := createTestPermission(t, db, 11, "unreferenced", 1)
	if err := db.Model(&role).Association("PermissionList").Append(&referenced); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	service := NewAuthorizationService(db, nil)

	if _, err := service.DeletePermission(context.Background(), &iam_v1.DeletePermissionRequest{Id: referenced.Id}); err != ErrPermissionReferenced {
		t.Fatalf("DeletePermission() error = %v, want %v", err, ErrPermissionReferenced)
	}
	if _, err := service.DeletePermission(context.Background(), &iam_v1.DeletePermissionRequest{Id: unreferenced.Id}); err != nil {
		t.Fatalf("DeletePermission() error = %v", err)
	}
	if _, err := service.DeletePermission(context.Background(), &iam_v1.DeletePermissionRequest{Id: unreferenced.Id}); err != gorm.ErrRecordNotFound {
		t.Fatalf("second DeletePermission() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

func TestDeleteRole(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	account := model.Account{
		Id:         1,
		Email:      "one@example.com",
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	role := createTestRole(t, db, 1, "role-code", 1)
	permission := createTestPermission(t, db, 10, "permission-code", 1)
	if err := db.Model(&role).Association("PermissionList").Append(&permission); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	if err := db.Model(&account).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	service := NewAuthorizationService(db, nil)

	if _, err := service.DeleteRole(context.Background(), &iam_v1.DeleteRoleRequest{Id: role.Id}); err != ErrRoleReferenced {
		t.Fatalf("DeleteRole() error = %v, want %v", err, ErrRoleReferenced)
	}
	if err := db.Model(&account).Association("RoleList").Delete(&role); err != nil {
		t.Fatalf("unbind role: %v", err)
	}
	if _, err := service.DeleteRole(context.Background(), &iam_v1.DeleteRoleRequest{Id: role.Id}); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}

	var permissionCount, roleCount int64
	if err := db.Model(&model.Role{}).Count(&roleCount).Error; err != nil {
		t.Fatalf("count role: %v", err)
	}
	if err := db.Model(&model.Permission{}).Where("id = ?", permission.Id).Count(&permissionCount).Error; err != nil {
		t.Fatalf("count permission: %v", err)
	}
	if roleCount != 0 || permissionCount != 1 {
		t.Fatalf("role/permission count = %d/%d, want 0/1", roleCount, permissionCount)
	}
}

func TestGetRolePermissionList(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	role := createTestRole(t, db, 1, "role-code", 1)
	permission := createTestPermission(t, db, 10, "permission-code", 1)
	if err := db.Model(&role).Association("PermissionList").Append(&permission); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	service := NewAuthorizationService(db, nil)

	response, err := service.GetRole(context.Background(), &common_v1.GetRequest{Id: 1})
	if err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	if len(response.PermissionCodeList) != 1 || response.PermissionCodeList[0] != permission.Code {
		t.Fatalf("PermissionCodeList = %v, want [%s]", response.PermissionCodeList, permission.Code)
	}
}

// T044-② UpdateRole 权限集合变更后，所有绑定该角色的账号快照键被删除，
// 未绑定账号的快照键不受影响（D6-①）。
func TestUpdateRoleInvalidatesBoundAccountsCache(t *testing.T) {
	ctx := context.Background()
	db := setupAuthorizationTestDB(t)
	rdb, mr := setupTestRedis(t)
	account := model.Account{
		Id:         1,
		Email:      "one@example.com",
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	role := createTestRole(t, db, 1, "role-code", 1)
	permission := createTestPermission(t, db, 10, "permission-code", 1)
	if err := db.Model(&account).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}

	boundKey := constant.GetAccountPermissionKey(account.Id)
	unboundKey := constant.GetAccountPermissionKey(2)
	for _, key := range []string{boundKey, unboundKey} {
		if err := rdb.Set(ctx, key, "[]", time.Minute).Err(); err != nil {
			t.Fatalf("seed snapshot %q: %v", key, err)
		}
	}

	service := NewAuthorizationService(db, rdb)
	if _, err := service.UpdateRole(ctx, &iam_v1.UpdateRoleRequest{
		Id:                      role.Id,
		Version:                 1,
		AddedPermissionCodeList: []string{permission.Code},
	}); err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}

	if mr.Exists(boundKey) {
		t.Fatalf("bound account snapshot key %q still exists, want deleted", boundKey)
	}
	if !mr.Exists(unboundKey) {
		t.Fatalf("unbound account snapshot key %q was deleted, want kept", unboundKey)
	}
}

// T060 审计字段注入（research.md D16）：CreateRole 注入四件套，UpdateRole 仅更新
// update_time/updated_by，created_by/create_time 保持不变；GetRole 回填审计字段。
func TestRoleAuditFields(t *testing.T) {
	createCtx := context.WithValue(context.Background(), "accountId", "42")
	updateCtx := context.WithValue(context.Background(), "accountId", "43")
	db := setupAuthorizationTestDB(t)
	service := NewAuthorizationService(db, nil)

	created, err := service.CreateRole(createCtx, &iam_v1.CreateRoleRequest{
		Name:     "role",
		RoleCode: "audit-role",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if created.CreatedBy != "42" || created.UpdatedBy != "42" {
		t.Fatalf("created_by/updated_by = %q/%q, want 42/42", created.CreatedBy, created.UpdatedBy)
	}
	if created.CreateTime == nil || created.UpdateTime == nil {
		t.Fatalf("create_time/update_time = %v/%v, want both set", created.CreateTime, created.UpdateTime)
	}

	roleId := cast.ToInt64(created.Id)
	_, err = service.UpdateRole(updateCtx, &iam_v1.UpdateRoleRequest{
		Id:      roleId,
		Name:    "updated",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}

	var got model.Role
	if err := db.First(&got, roleId).Error; err != nil {
		t.Fatalf("get role: %v", err)
	}
	if got.CreatedBy != 42 || got.UpdatedBy != 43 {
		t.Fatalf("created_by/updated_by = %d/%d, want 42/43", got.CreatedBy, got.UpdatedBy)
	}
	if got.CreateTime.Unix() != created.CreateTime.AsTime().Unix() {
		t.Fatalf("create_time = %v, want unchanged %v", got.CreateTime, created.CreateTime.AsTime())
	}
	if got.UpdateTime.IsZero() {
		t.Fatal("update_time is zero, want updated")
	}

	detail, err := service.GetRole(context.Background(), &common_v1.GetRequest{Id: roleId})
	if err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	if detail.CreatedBy != "42" || detail.UpdatedBy != "43" {
		t.Fatalf("GetRole created_by/updated_by = %q/%q, want 42/43", detail.CreatedBy, detail.UpdatedBy)
	}
	if detail.CreateTime == nil || detail.UpdateTime == nil {
		t.Fatalf("GetRole create_time/update_time = %v/%v, want both set", detail.CreateTime, detail.UpdateTime)
	}
}

// T060 审计字段注入（research.md D16）：CreatePermission / UpdatePermission 对称覆盖。
func TestPermissionAuditFields(t *testing.T) {
	createCtx := context.WithValue(context.Background(), "accountId", "42")
	updateCtx := context.WithValue(context.Background(), "accountId", "43")
	db := setupAuthorizationTestDB(t)
	service := NewAuthorizationService(db, nil)

	created, err := service.CreatePermission(createCtx, &iam_v1.CreatePermissionRequest{
		ApplicationId: 1,
		Name:          "permission",
		Code:          "audit-permission",
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	if created.CreatedBy != "42" || created.UpdatedBy != "42" {
		t.Fatalf("created_by/updated_by = %q/%q, want 42/42", created.CreatedBy, created.UpdatedBy)
	}
	if created.CreateTime == nil || created.UpdateTime == nil {
		t.Fatalf("create_time/update_time = %v/%v, want both set", created.CreateTime, created.UpdateTime)
	}

	permissionId := cast.ToInt64(created.Id)
	_, err = service.UpdatePermission(updateCtx, &iam_v1.UpdatePermissionRequest{
		Id:      permissionId,
		Name:    "updated",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("UpdatePermission() error = %v", err)
	}

	var got model.Permission
	if err := db.First(&got, permissionId).Error; err != nil {
		t.Fatalf("get permission: %v", err)
	}
	if got.CreatedBy != 42 || got.UpdatedBy != 43 {
		t.Fatalf("created_by/updated_by = %d/%d, want 42/43", got.CreatedBy, got.UpdatedBy)
	}
	if got.CreateTime.Unix() != created.CreateTime.AsTime().Unix() {
		t.Fatalf("create_time = %v, want unchanged %v", got.CreateTime, created.CreateTime.AsTime())
	}

	detail, err := service.GetPermission(context.Background(), &common_v1.GetRequest{Id: permissionId})
	if err != nil {
		t.Fatalf("GetPermission() error = %v", err)
	}
	if detail.CreatedBy != "42" || detail.UpdatedBy != "43" {
		t.Fatalf("GetPermission created_by/updated_by = %q/%q, want 42/43", detail.CreatedBy, detail.UpdatedBy)
	}
}
