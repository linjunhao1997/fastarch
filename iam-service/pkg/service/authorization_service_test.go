package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
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
	err = db.AutoMigrate(&model.Application{}, &model.Role{}, &model.Permission{}, &model.Account{}, &model.RAccountRole{}, &model.RRolePermission{}, &model.Resource{}, &model.PermissionResource{})
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
	createTestApplication(t, db, 1, "iam")
	createTestRole(t, db, 1, "role-code", 1)
	service := NewAuthorizationService(db, nil)

	_, err := service.CreateRole(context.Background(), &iam_v1.CreateRoleRequest{ApplicationId: 1, Name: "role", RoleCode: "role-code"})
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
	application := model.Application{Id: 1, Code: constant.ApplicationCode, Name: "IAM", Disabled: 0}
	if err := db.Create(&application).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	role := createTestRole(t, db, 1, "role-code", 1)
	role.ApplicationId = application.Id
	if err := db.Model(&role).Update("application_id", role.ApplicationId).Error; err != nil {
		t.Fatalf("bind role application: %v", err)
	}
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
	application := model.Application{Id: 1, Code: constant.ApplicationCode, Name: "IAM", Disabled: 0}
	if err := db.Create(&application).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	role := createTestRole(t, db, 1, "role-code", 1)
	role.ApplicationId = application.Id
	if err := db.Model(&role).Update("application_id", role.ApplicationId).Error; err != nil {
		t.Fatalf("bind role application: %v", err)
	}
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
	application := model.Application{Id: 1, Code: constant.ApplicationCode, Name: "IAM", Disabled: 0}
	if err := db.Create(&application).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	role := createTestRole(t, db, 1, "role-code", 1)
	role.ApplicationId = application.Id
	if err := db.Model(&role).Update("application_id", role.ApplicationId).Error; err != nil {
		t.Fatalf("bind role application: %v", err)
	}
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
	createTestApplication(t, db, 1, "IAM")

	created, err := service.CreateRole(createCtx, &iam_v1.CreateRoleRequest{
		ApplicationId: 1,
		Name:          "role",
		RoleCode:      "audit-role",
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
	menuResource := createTestResource(t, db, 1, model.PermissionKindMenu, "audit.menu.key", "审计菜单")

	created, err := service.CreatePermission(createCtx, &iam_v1.CreatePermissionRequest{
		ApplicationId:   0,
		Name:            "permission",
		Code:            "audit-permission",
		Kind:            model.PermissionKindMenu,
		ResourceKeyList: []string{menuResource.Key},
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

func TestListRolesKeyword(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestRole(t, db, 1, "alpha", 1)
	createTestRole(t, db, 2, "beta", 1)
	service := NewAuthorizationService(db, nil)

	response, err := service.ListRoles(context.Background(), &iam_v1.ListRolesRequest{Keyword: "alpha"})
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(response.Roles) != 1 || response.Roles[0].RoleCode != "alpha" {
		t.Fatalf("ListRoles() = %v, want alpha", response.Roles)
	}
}

func TestListPermissionsKeyword(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestPermission(t, db, 1, "alpha", 1)
	createTestPermission(t, db, 2, "beta", 1)
	service := NewAuthorizationService(db, nil)

	response, err := service.ListPermissions(context.Background(), &iam_v1.ListPermissionsRequest{Keyword: "alpha"})
	if err != nil {
		t.Fatalf("ListPermissions() error = %v", err)
	}
	if len(response.Permissions) != 1 || response.Permissions[0].PermissionCode != "alpha" {
		t.Fatalf("ListPermissions() = %v, want alpha", response.Permissions)
	}
}

func TestListSubscriptionsKeyword(t *testing.T) {
	service := NewAuthorizationService(setupAuthorizationTestDB(t), nil)
	response, err := service.ListSubscriptions(context.Background(), &iam_v1.ListSubscriptionsRequest{Keyword: "alpha"})
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}
	if response == nil {
		t.Fatal("ListSubscriptions() response is nil")
	}
}

// ── US2: 三域访问控制（specs/005-typed-permission-model T020-T025）──

// setupTypedPermissionEnv 构造「账号→角色→权限→资源」的完整测试链路，
// 资源覆盖三种类型，返回账号 id 与 service 实例。
func setupTypedPermissionEnv(t *testing.T) (*gorm.DB, *AuthorizationService, int64) {
	t.Helper()
	db := setupAuthorizationTestDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	menuRes := createTestResource(t, db, 1, model.PermissionKindMenu, "iam.account_management.account_list", "用户列表")
	httpRes := createTestResource(t, db, 2, model.PermissionKindHTTPAPI, "GET /authorization/permissions", "权限列表")
	if err := db.Model(&model.Resource{}).Where(model.ResourceColumnId, httpRes.Id).
		Update(model.ResourceColumnRpcFullMethod, "/iam.v1.AuthorizationService/ListPermissions").Error; err != nil {
		t.Fatalf("update resource: %v", err)
	}
	grpcRes := createTestResource(t, db, 3, model.PermissionKindGRPC, "/iam.v1.AuthorizationService/ListRoles", "角色列表")

	role := createTestRole(t, db, 1, "role-code", 1)
	for _, res := range []model.Resource{menuRes, httpRes, grpcRes} {
		kindPerm := model.Permission{Id: res.Id + 100, Name: "perm", Code: "perm-" + res.Key, Kind: res.Kind, CreateTime: time.Now(), UpdateTime: time.Now()}
		if err := db.Create(&kindPerm).Error; err != nil {
			t.Fatalf("create permission: %v", err)
		}
		if err := db.Model(&kindPerm).Association("ResourceList").Append(&res); err != nil {
			t.Fatalf("bind resource: %v", err)
		}
		if err := db.Model(&role).Association("PermissionList").Append(&kindPerm); err != nil {
			t.Fatalf("bind permission: %v", err)
		}
	}
	account := model.Account{Id: 1, Email: "user@example.com", CreateTime: time.Now(), UpdateTime: time.Now()}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := db.Model(&account).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	return db, &AuthorizationService{db: db, rdb: rdb}, account.Id
}

// GetMyPermissions 返回结构化资源集合（三域，research.md D9/D10）。
func TestGetMyPermissionsReturnsStructuredResourceKeys(t *testing.T) {
	_, service, accountId := setupTypedPermissionEnv(t)
	response, err := service.GetMyPermissions(auth.WithAccountId(context.Background(), accountId), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyPermissions() error = %v", err)
	}
	wantMenu := []string{"iam.account_management.account_list"}
	wantHTTP := []string{"/iam.v1.AuthorizationService/ListPermissions"}
	wantRPC := []string{"/iam.v1.AuthorizationService/ListRoles"}
	if !reflect.DeepEqual(response.MenuKeyList, wantMenu) {
		t.Fatalf("MenuKeyList = %v, want %v", response.MenuKeyList, wantMenu)
	}
	if !reflect.DeepEqual(response.HttpFullMethodList, wantHTTP) {
		t.Fatalf("HttpFullMethodList = %v, want %v", response.HttpFullMethodList, wantHTTP)
	}
	if !reflect.DeepEqual(response.RpcFullMethodList, wantRPC) {
		t.Fatalf("RpcFullMethodList = %v, want %v", response.RpcFullMethodList, wantRPC)
	}
}

// 停用角色后其资源即时失效（FR-011/SC-004）。
func TestGetMyPermissionsExcludesDisabledRole(t *testing.T) {
	db, service, accountId := setupTypedPermissionEnv(t)
	if err := db.Model(model.Role{}).Where(model.RoleColumnId, 1).Update(model.RoleColumnDisabled, true).Error; err != nil {
		t.Fatalf("disable role: %v", err)
	}
	response, err := service.GetMyPermissions(auth.WithAccountId(context.Background(), accountId), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyPermissions() error = %v", err)
	}
	if len(response.MenuKeyList) != 0 || len(response.HttpFullMethodList) != 0 || len(response.RpcFullMethodList) != 0 {
		t.Fatalf("disabled role should yield empty permissions, got %+v", response)
	}
}

// CheckPermission 按资源 key 语义判定（契约 §2.8）：rpc 域 full method 匹配。
func TestCheckPermissionMatchesResourceKey(t *testing.T) {
	_, service, accountId := setupTypedPermissionEnv(t)
	ctx := auth.WithAccountId(context.Background(), accountId)

	resp, err := service.CheckPermission(ctx, &iam_v1.CheckPermissionRequest{
		ApplicationCode: constant.ApplicationCode,
		PermissionCode:  "/iam.v1.AuthorizationService/ListRoles",
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if !resp.Ok {
		t.Fatal("CheckPermission() = false, want true for granted rpc key")
	}

	// 未授权方法
	resp, err = service.CheckPermission(ctx, &iam_v1.CheckPermissionRequest{
		ApplicationCode: constant.ApplicationCode,
		PermissionCode:  "/iam.v1.AccountService/DeleteAccount",
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if resp.Ok {
		t.Fatal("CheckPermission() = true, want false for ungranted method")
	}

	// 旧格式复合权限码不做兼容转换（契约 §2.8）
	resp, err = service.CheckPermission(ctx, &iam_v1.CheckPermissionRequest{
		ApplicationCode: constant.ApplicationCode,
		PermissionCode:  constant.ApplicationCode + ":admin",
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if resp.Ok {
		t.Fatal("CheckPermission() = true, want false for legacy composite code")
	}
}

// ── US3: Seeder 初始化（specs/005-typed-permission-model T027/T033）──

// VR-4：管理员角色禁止删除。
func TestDeleteRoleRejectsAdminRole(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	service := NewAuthorizationService(db, nil)
	adminRole := createTestRole(t, db, 1, model.RoleAdmin, 1)

	_, err := service.DeleteRole(context.Background(), &iam_v1.DeleteRoleRequest{Id: cast.ToInt64(adminRole.Id)})
	if err == nil || !errors.Is(err, ErrRoleAdminProtected) {
		t.Fatalf("DeleteRole() error = %v, want ErrRoleAdminProtected", err)
	}
	var count int64
	if err := db.Model(model.Role{}).Where(model.RoleColumnId, adminRole.Id).Count(&count).Error; err != nil {
		t.Fatalf("count role: %v", err)
	}
	if count != 1 {
		t.Fatal("admin role should not be deleted")
	}
}

// ── US1: 类型化权限 CRUD（specs/005-typed-permission-model T009）──

func createTestResource(t *testing.T, db *gorm.DB, id int64, kind, key, name string) model.Resource {
	t.Helper()
	resource := model.Resource{
		Id:         id,
		Kind:       kind,
		Key:        key,
		Name:       name,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatalf("create resource: %v", err)
	}
	return resource
}

func TestCreatePermissionValidatesKindAndResources(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	menuRes := createTestResource(t, db, 1, model.PermissionKindMenu, "iam.account_management.account_list", "用户列表")
	grpcRes := createTestResource(t, db, 2, model.PermissionKindGRPC, "/iam.v1.AccountService/ListAccounts", "查询账号列表")
	service := NewAuthorizationService(db, nil)
	ctx := context.Background()

	cases := []struct {
		name    string
		request *iam_v1.CreatePermissionRequest
		wantErr error
	}{
		{
			name:    "kind 非法",
			request: &iam_v1.CreatePermissionRequest{Name: "p", Code: "c1", Kind: "bogus", ResourceKeyList: []string{menuRes.Key}},
			wantErr: ErrInvalidPermissionKind,
		},
		{
			name:    "未绑定资源",
			request: &iam_v1.CreatePermissionRequest{Name: "p", Code: "c2", Kind: model.PermissionKindMenu},
			wantErr: ErrPermissionResourceRequired,
		},
		{
			name:    "资源不存在",
			request: &iam_v1.CreatePermissionRequest{Name: "p", Code: "c4", Kind: model.PermissionKindMenu, ResourceKeyList: []string{"no.such.key"}},
			wantErr: ErrResourceNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.CreatePermission(ctx, tc.request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreatePermission() error = %v, want %v", err, tc.wantErr)
			}
		})
	}

	// 合法创建：kind 与绑定持久化
	created, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{
		Name: "接口权限", Code: "grpc-perm", Kind: model.PermissionKindGRPC,
		ResourceKeyList: []string{grpcRes.Key},
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	if created.Kind != model.PermissionKindGRPC {
		t.Fatalf("created.Kind = %q, want %q", created.Kind, model.PermissionKindGRPC)
	}
	var count int64
	if err := db.Model(&model.PermissionResource{}).Where(model.PermissionResourceColumnPermissionId, created.Id).Count(&count).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if count != 1 {
		t.Fatalf("binding count = %d, want 1", count)
	}
}

func TestCreatePermissionCodeDuplicated(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	menuRes := createTestResource(t, db, 1, model.PermissionKindMenu, "iam.menu.a", "菜单A")
	service := NewAuthorizationService(db, nil)
	ctx := context.Background()
	req := &iam_v1.CreatePermissionRequest{Name: "p", Code: "dup", Kind: model.PermissionKindMenu, ResourceKeyList: []string{menuRes.Key}}
	if _, err := service.CreatePermission(ctx, req); err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	if _, err := service.CreatePermission(ctx, req); !errors.Is(err, ErrPermissionCodeDuplicated) {
		t.Fatalf("CreatePermission() error = %v, want %v", err, ErrPermissionCodeDuplicated)
	}
}

func TestUpdatePermissionReplacesResourceBindings(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	res1 := createTestResource(t, db, 1, model.PermissionKindMenu, "iam.menu.a", "菜单A")
	res2 := createTestResource(t, db, 2, model.PermissionKindMenu, "iam.menu.b", "菜单B")
	res3 := createTestResource(t, db, 3, model.PermissionKindMenu, "iam.menu.c", "菜单C")
	grpcRes := createTestResource(t, db, 4, model.PermissionKindGRPC, "/iam.v1.X/Y", "X")
	service := NewAuthorizationService(db, nil)
	ctx := context.Background()

	created, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{
		Name: "p", Code: "perm", Kind: model.PermissionKindGRPC,
		ResourceKeyList: []string{grpcRes.Key, res1.Key, res2.Key},
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	permId := cast.ToInt64(created.Id)

	// 替换式更新：[a,b] → [c]
	if _, err := service.UpdatePermission(ctx, &iam_v1.UpdatePermissionRequest{
		Id: permId, Version: created.Version, ResourceKeyList: []string{res3.Key},
	}); err != nil {
		t.Fatalf("UpdatePermission() error = %v", err)
	}
	var perm model.Permission
	if err := db.Where(model.PermissionColumnId, permId).First(&perm).Error; err != nil {
		t.Fatalf("find permission: %v", err)
	}
	var boundKeys []string
	var bindings []model.PermissionResource
	if err := db.Where(model.PermissionResourceColumnPermissionId, permId).Find(&bindings).Error; err != nil {
		t.Fatalf("find bindings: %v", err)
	}
	for _, b := range bindings {
		var res model.Resource
		if err := db.Where(model.ResourceColumnId, b.ResourceId).First(&res).Error; err != nil {
			t.Fatalf("find resource: %v", err)
		}
		boundKeys = append(boundKeys, res.Key)
	}
	if len(boundKeys) != 1 || boundKeys[0] != res3.Key {
		t.Fatalf("boundKeys = %v, want [%s]", boundKeys, res3.Key)
	}

	// 不提供 resource_key_list 时不触碰绑定
	if _, err := service.UpdatePermission(ctx, &iam_v1.UpdatePermissionRequest{
		Id: permId, Version: created.Version + 1, Name: "renamed",
	}); err != nil {
		t.Fatalf("UpdatePermission() error = %v", err)
	}
	var count int64
	if err := db.Model(&model.PermissionResource{}).Where(model.PermissionResourceColumnPermissionId, permId).Count(&count).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if count != 1 {
		t.Fatalf("binding count after rename = %d, want 1", count)
	}
}

func TestGetPermissionReturnsKindAndResources(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	res := createTestResource(t, db, 1, model.PermissionKindHTTPAPI, "GET /authorization/permissions", "权限列表")
	if err := db.Model(&model.Resource{}).Where(model.ResourceColumnId, res.Id).
		Update(model.ResourceColumnRpcFullMethod, "/iam.v1.AuthorizationService/ListPermissions").Error; err != nil {
		t.Fatalf("update resource: %v", err)
	}
	service := NewAuthorizationService(db, nil)
	ctx := context.Background()

	created, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{
		Name: "p", Code: "perm", Kind: model.PermissionKindHTTPAPI,
		ResourceKeyList: []string{res.Key},
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}

	got, err := service.GetPermission(ctx, &common_v1.GetRequest{Id: cast.ToInt64(created.Id)})
	if err != nil {
		t.Fatalf("GetPermission() error = %v", err)
	}
	if got.Kind != model.PermissionKindHTTPAPI {
		t.Fatalf("got.Kind = %q, want %q", got.Kind, model.PermissionKindHTTPAPI)
	}
	if len(got.ResourceList) != 1 || got.ResourceList[0].Key != res.Key || got.ResourceList[0].RpcFullMethod != "/iam.v1.AuthorizationService/ListPermissions" {
		t.Fatalf("got.ResourceList = %+v", got.ResourceList)
	}
}

func TestListPermissionsKindFilter(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	menuRes := createTestResource(t, db, 1, model.PermissionKindMenu, "iam.menu.a", "菜单A")
	grpcRes := createTestResource(t, db, 2, model.PermissionKindGRPC, "/iam.v1.A/B", "B")
	service := NewAuthorizationService(db, nil)
	ctx := context.Background()
	if _, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{Name: "m", Code: "m1", Kind: model.PermissionKindMenu, ResourceKeyList: []string{menuRes.Key}}); err != nil {
		t.Fatalf("create menu permission: %v", err)
	}
	if _, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{Name: "g", Code: "g1", Kind: model.PermissionKindGRPC, ResourceKeyList: []string{grpcRes.Key}}); err != nil {
		t.Fatalf("create grpc permission: %v", err)
	}

	resp, err := service.ListPermissions(ctx, &iam_v1.ListPermissionsRequest{Kind: model.PermissionKindGRPC})
	if err != nil {
		t.Fatalf("ListPermissions() error = %v", err)
	}
	if len(resp.Permissions) != 1 || resp.Permissions[0].Kind != model.PermissionKindGRPC {
		t.Fatalf("ListPermissions(kind=grpc) = %+v", resp.Permissions)
	}
}

func TestListResourcesFilterAndPagination(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	for i := 1; i <= 5; i++ {
		createTestResource(t, db, int64(i), model.PermissionKindMenu, fmt.Sprintf("iam.menu.%d", i), fmt.Sprintf("菜单%d", i))
	}
	createTestResource(t, db, 6, model.PermissionKindGRPC, "/iam.v1.A/B", "B")
	service := NewAuthorizationService(db, nil)
	ctx := context.Background()

	resp, err := service.ListResources(ctx, &iam_v1.ListResourcesRequest{Kind: model.PermissionKindMenu, CurrentPage: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resp.Resources) != 2 {
		t.Fatalf("ListResources(page_size=2) len = %d, want 2", len(resp.Resources))
	}
	if resp.Page == nil || resp.Page.Total != 5 {
		t.Fatalf("ListResources() page = %+v, want total 5", resp.Page)
	}

	resp, err = service.ListResources(ctx, &iam_v1.ListResourcesRequest{Kind: model.PermissionKindGRPC})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resp.Resources) != 1 || resp.Resources[0].Kind != model.PermissionKindGRPC {
		t.Fatalf("ListResources(kind=grpc) = %+v", resp.Resources)
	}
}

func TestListResourcesApplicationIdFilter(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createMenuResource(t, db, 1, 1, nil, "iam.app1.a", "应用1菜单A", 0)
	createMenuResource(t, db, 2, 1, nil, "iam.app1.b", "应用1菜单B", 0)
	createMenuResource(t, db, 3, 2, nil, "iam.app2.a", "应用2菜单A", 0)
	service := NewAuthorizationService(db, nil)
	ctx := context.Background()

	resp, err := service.ListResources(ctx, &iam_v1.ListResourcesRequest{ApplicationId: 1})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resp.Resources) != 2 || resp.Page == nil || resp.Page.Total != 2 {
		t.Fatalf("ListResources(application_id=1) = %+v, page = %+v", resp.Resources, resp.Page)
	}
	for _, r := range resp.Resources {
		if r.ApplicationId != 1 {
			t.Fatalf("ListResources(application_id=1) contains resource %s with application_id %d", r.Id, r.ApplicationId)
		}
	}

	// 不传 application_id 保持全局行为
	resp, err = service.ListResources(ctx, &iam_v1.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if resp.Page == nil || resp.Page.Total != 3 {
		t.Fatalf("ListResources() page = %+v, want total 3", resp.Page)
	}
}

// ── specs/008-account-app-context：我的菜单树（T019）──

func createMenuResource(t *testing.T, db *gorm.DB, id int64, applicationId int64, parentId *int64, key, name string, displayOrder int32) model.Resource {
	t.Helper()
	now := time.Now()
	resource := model.Resource{
		Id:            id,
		ApplicationId: applicationId,
		ParentId:      parentId,
		Kind:          model.PermissionKindMenu,
		Key:           key,
		Name:          name,
		DisplayOrder:  displayOrder,
		CreateTime:    now,
		UpdateTime:    now,
		Version:       1,
	}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatalf("create menu resource: %v", err)
	}
	return resource
}

func setPermissionSnapshot(t *testing.T, rdb *redis.Client, accountId int64, keys []string) {
	t.Helper()
	raw, err := json.Marshal(keys)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := rdb.Set(context.Background(), constant.GetAccountPermissionKey(accountId), raw, time.Minute).Err(); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}
}

func setupMenuTreeFixture(t *testing.T) (*gorm.DB, *redis.Client, model.Application) {
	t.Helper()
	db := setupAuthorizationTestDB(t)
	rdb, _ := setupTestRedis(t)
	now := time.Now()
	app := model.Application{
		Id:         500,
		Code:       "500",
		Name:       "菜单应用",
		Disabled:   0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	return db, rdb, app
}

func collectMenuKeys(nodes []*iam_v1.ResourceTreeNode) map[string]bool {
	out := make(map[string]bool)
	var walk func(list []*iam_v1.ResourceTreeNode)
	walk = func(list []*iam_v1.ResourceTreeNode) {
		for _, n := range list {
			out[n.Resource.Key] = true
			walk(n.Children)
		}
	}
	walk(nodes)
	return out
}

// 普通用户：仅返回权限键命中的菜单节点，祖先路径随命中节点返回
func TestListMyMenuTreePermissionFiltering(t *testing.T) {
	db, rdb, app := setupMenuTreeFixture(t)
	rootKey := "account_management"
	root := createMenuResource(t, db, 1, app.Id, nil, rootKey, "账号管理", 1)
	rootId := root.Id
	createMenuResource(t, db, 2, app.Id, &rootId, "account_management.account_list", "用户列表", 1)
	createMenuResource(t, db, 3, app.Id, nil, "application_management", "应用管理", 2)
	// 非 menu 资源不得出现
	createMenuResource(t, db, 4, app.Id, nil, "GET /x", "http 资源", 3)
	if err := db.Model(&model.Resource{}).Where(model.ResourceColumnId, 4).
		Update(model.ResourceColumnKind, model.PermissionKindHTTPAPI).Error; err != nil {
		t.Fatalf("update kind: %v", err)
	}
	setPermissionSnapshot(t, rdb, 2, []string{
		constant.SnapshotKeyPrefixMenu + "account_management.account_list",
	})

	service := NewAuthorizationService(db, rdb)
	resp, err := service.ListMyMenuTree(auth.WithAccountId(context.Background(), 2), &iam_v1.ListMyMenuTreeRequest{ApplicationId: app.Id})
	if err != nil {
		t.Fatalf("ListMyMenuTree() error = %v", err)
	}

	keys := collectMenuKeys(resp.Roots)
	if !keys["account_management"] || !keys["account_management.account_list"] {
		t.Fatalf("permitted menu + ancestors missing: %v", keys)
	}
	if keys["application_management"] {
		t.Fatalf("unpermitted menu visible: %v", keys)
	}
	if keys["GET /x"] {
		t.Fatalf("non-menu resource visible: %v", keys)
	}
}

// 管理员：菜单可见性同样受权限键约束（规格变更，原全量可见分支已移除）
func TestListMyMenuTreeAdminFollowsPermissions(t *testing.T) {
	db, rdb, app := setupMenuTreeFixture(t)
	createMenuResource(t, db, 1, app.Id, nil, "account_management", "账号管理", 1)
	createMenuResource(t, db, 2, app.Id, nil, "application_management", "应用管理", 2)
	setPermissionSnapshot(t, rdb, 2, []string{
		constant.SnapshotKeyPrefixMenu + "account_management",
	})

	service := NewAuthorizationService(db, rdb)
	resp, err := service.ListMyMenuTree(
		auth.WithAdmin(auth.WithAccountId(context.Background(), 2)),
		&iam_v1.ListMyMenuTreeRequest{ApplicationId: app.Id},
	)
	if err != nil {
		t.Fatalf("ListMyMenuTree() error = %v", err)
	}
	keys := collectMenuKeys(resp.Roots)
	if !keys["account_management"] {
		t.Fatalf("permitted menu missing for admin: %v", keys)
	}
	if keys["application_management"] {
		t.Fatalf("unpermitted menu visible for admin: %v", keys)
	}
}

// 应用停用：业务错误拒绝
func TestListMyMenuTreeDisabledApplication(t *testing.T) {
	db, rdb, app := setupMenuTreeFixture(t)
	if err := db.Model(&model.Application{}).Where(model.ApplicationColumnId, app.Id).
		Update(model.ApplicationColumnDisabled, 1).Error; err != nil {
		t.Fatalf("disable application: %v", err)
	}

	service := NewAuthorizationService(db, rdb)
	_, err := service.ListMyMenuTree(auth.WithAccountId(context.Background(), 2), &iam_v1.ListMyMenuTreeRequest{ApplicationId: app.Id})
	if err != ErrApplicationDisabled {
		t.Fatalf("ListMyMenuTree() error = %v, want ErrApplicationDisabled", err)
	}
}

// 空菜单：返回空 roots 而非报错
func TestListMyMenuTreeEmpty(t *testing.T) {
	db, rdb, app := setupMenuTreeFixture(t)
	// 读穿回源需要账号存在（正常调用链中 JWT 已保证）
	createTestAccount(t, db, 2, "viewer@example.com", 1)

	service := NewAuthorizationService(db, rdb)
	resp, err := service.ListMyMenuTree(auth.WithAccountId(context.Background(), 2), &iam_v1.ListMyMenuTreeRequest{ApplicationId: app.Id})
	if err != nil {
		t.Fatalf("ListMyMenuTree() error = %v", err)
	}
	if len(resp.Roots) != 0 {
		t.Fatalf("roots = %v, want empty", resp.Roots)
	}
}
