package service

import (
	"context"
	"errors"
	"slices"
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
	if err := model.AutoMigrateModels(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func createTestRole(t *testing.T, db *gorm.DB, id int64, code string, version int64) model.Role {
	t.Helper()
	role := model.Role{
		Id:         id,
		Name:       "role",
		Code:       code,
		Version:    version,
		Status:     model.StatusEnabled,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	return role
}

// 多应用裁决 #8：测试统一使用 id=900/code=iam 的应用上下文。
const testAppId = int64(900)

func setupTestApplication(t *testing.T, db *gorm.DB) {
	t.Helper()
	app := model.Application{
		Id:         testAppId,
		AppId:      "900",
		Code:       "iam",
		Name:       "IAM 测试应用",
		Status:     model.StatusEnabled,
		Version:    1,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
}

func appCtx(ctx context.Context) context.Context {
	return auth.WithApplicationId(ctx, testAppId)
}

func createTestPermission(t *testing.T, db *gorm.DB, id int64, code string, version int64) model.Permission {
	t.Helper()
	permission := model.Permission{
		Id:            id,
		Name:          "permission",
		Code:          code,
		ApplicationId: testAppId,
		Version:       version,
		Status:        model.StatusEnabled,
		Type:          "api",
		CreateTime:    time.Now(),
		UpdateTime:    time.Now(),
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
	permission := createTestPermission(t, db, 10, "iam.permission.list", 1)
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
		AddedPermissionCodeList: []string{"iam.missing.code"},
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
	setupTestApplication(t, db)
	permission := createTestPermission(t, db, 1, "iam.permission.list", 1)
	service := NewAuthorizationService(db, nil)

	_, err := service.CreatePermission(appCtx(context.Background()), &iam_v1.CreatePermissionRequest{
		Name: "duplicate",
		Code: permission.Code,
	})
	if err != ErrPermissionCodeDuplicated {
		t.Fatalf("CreatePermission() error = %v, want %v", err, ErrPermissionCodeDuplicated)
	}

	_, err = service.UpdatePermission(appCtx(context.Background()), &iam_v1.UpdatePermissionRequest{
		Id:      1,
		Name:    "updated",
		Version: 2,
	})
	if err != ErrUpdatePermissionBusy {
		t.Fatalf("UpdatePermission() error = %v, want %v", err, ErrUpdatePermissionBusy)
	}

	_, err = service.UpdatePermission(appCtx(context.Background()), &iam_v1.UpdatePermissionRequest{
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
	setupTestApplication(t, db)
	role := createTestRole(t, db, 1, "role-code", 1)
	referenced := createTestPermission(t, db, 10, "iam.permission.list", 1)
	unreferenced := createTestPermission(t, db, 11, "iam.role.list", 1)
	if err := db.Model(&role).Association("PermissionList").Append(&referenced); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	service := NewAuthorizationService(db, nil)

	if _, err := service.DeletePermission(appCtx(context.Background()), &iam_v1.DeletePermissionRequest{Id: referenced.Id}); err != ErrPermissionReferenced {
		t.Fatalf("DeletePermission() error = %v, want %v", err, ErrPermissionReferenced)
	}
	if _, err := service.DeletePermission(appCtx(context.Background()), &iam_v1.DeletePermissionRequest{Id: unreferenced.Id}); err != nil {
		t.Fatalf("DeletePermission() error = %v", err)
	}
	if _, err := service.DeletePermission(appCtx(context.Background()), &iam_v1.DeletePermissionRequest{Id: unreferenced.Id}); err != gorm.ErrRecordNotFound {
		t.Fatalf("second DeletePermission() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

func TestDeleteRole(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	user := createTestUser(t, db, 1, "one", 1)
	role := createTestRole(t, db, 1, "role-code", 1)
	permission := createTestPermission(t, db, 10, "iam.permission.list", 1)
	if err := db.Model(&role).Association("PermissionList").Append(&permission); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	if err := db.Model(&user).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	service := NewAuthorizationService(db, nil)

	if _, err := service.DeleteRole(context.Background(), &iam_v1.DeleteRoleRequest{Id: role.Id}); err != ErrRoleReferenced {
		t.Fatalf("DeleteRole() error = %v, want %v", err, ErrRoleReferenced)
	}
	if err := db.Model(&user).Association("RoleList").Delete(&role); err != nil {
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
	permission := createTestPermission(t, db, 10, "iam.permission.list", 1)
	if err := db.Model(&role).Association("PermissionList").Append(&permission); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	service := NewAuthorizationService(db, nil)

	response, err := service.GetRole(context.Background(), &common_v1.GetRequest{Id: 1})
	if err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	if len(response.PermissionList) != 1 || response.PermissionList[0].PermissionCode != permission.Code {
		t.Fatalf("PermissionList = %v, want [%s]", response.PermissionList, permission.Code)
	}
}

// UpdateRole 权限集合变更后，所有绑定该角色的账号快照键被删除，
// 未绑定账号的快照键不受影响。
func TestUpdateRoleInvalidatesBoundAccountsCache(t *testing.T) {
	ctx := context.Background()
	db := setupAuthorizationTestDB(t)
	rdb, mr := setupTestRedis(t)
	user := createTestUser(t, db, 1, "one", 1)
	role := createTestRole(t, db, 1, "role-code", 1)
	permission := createTestPermission(t, db, 10, "iam.permission.list", 1)
	if err := db.Model(&user).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}

	boundKey := constant.GetAccountPermissionKey(user.Id)
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

// 审计字段注入：CreateRole 注入四件套，UpdateRole 仅更新
// update_time/updated_by，created_by/create_time 保持不变；GetRole 回填审计字段。
func TestRoleAuditFields(t *testing.T) {
	createCtx := auth.WithDataAppId(appCtx(context.WithValue(context.Background(), "accountId", "42")), testAppId)
	updateCtx := auth.WithDataAppId(appCtx(context.WithValue(context.Background(), "accountId", "43")), testAppId)
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
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
	if got.CreatedBy != "42" || got.UpdatedBy != "43" {
		t.Fatalf("created_by/updated_by = %q/%q, want 42/43", got.CreatedBy, got.UpdatedBy)
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

// 审计字段注入：CreatePermission / UpdatePermission 对称覆盖。
func TestPermissionAuditFields(t *testing.T) {
	createCtx := auth.WithDataAppId(appCtx(auth.WithAccountId(context.Background(), 42)), testAppId)
	updateCtx := auth.WithDataAppId(appCtx(auth.WithAccountId(context.Background(), 43)), testAppId)
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
	service := NewAuthorizationService(db, nil)

	created, err := service.CreatePermission(createCtx, &iam_v1.CreatePermissionRequest{
		Name: "permission",
		Code: "iam.audit.perm",
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
	if got.CreatedBy != "42" || got.UpdatedBy != "43" {
		t.Fatalf("created_by/updated_by = %q/%q, want 42/43", got.CreatedBy, got.UpdatedBy)
	}
	if got.CreateTime.Unix() != created.CreateTime.AsTime().Unix() {
		t.Fatalf("create_time = %v, want unchanged %v", got.CreateTime, created.CreateTime.AsTime())
	}

	detail, err := service.GetPermission(appCtx(context.Background()), &common_v1.GetRequest{Id: permissionId})
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
	setupTestApplication(t, db)
	createTestPermission(t, db, 1, "iam.alpha.code", 1)
	createTestPermission(t, db, 2, "iam.beta.code", 1)
	service := NewAuthorizationService(db, nil)

	response, err := service.ListPermissions(appCtx(context.Background()), &iam_v1.ListPermissionsRequest{Keyword: "alpha"})
	if err != nil {
		t.Fatalf("ListPermissions() error = %v", err)
	}
	if len(response.Permissions) != 1 || response.Permissions[0].PermissionCode != "iam.alpha.code" {
		t.Fatalf("ListPermissions() = %v, want iam.alpha.code", response.Permissions)
	}
}

// ── 权限快照与判定（权限码模型）──

// setupPermissionCodeEnv 构造「账号→角色→权限码」的测试链路，返回账号 id 与 service 实例。
func setupPermissionCodeEnv(t *testing.T) (*gorm.DB, *AuthorizationService, int64) {
	t.Helper()
	db := setupAuthorizationTestDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	role := createTestRole(t, db, 1, "role-code", 1)
	for i, code := range []string{"iam.permission.list", "iam.role.list"} {
		perm := model.Permission{Id: int64(100 + i), Name: code, Code: code, Status: model.StatusEnabled, CreateTime: time.Now(), UpdateTime: time.Now()}
		if err := db.Create(&perm).Error; err != nil {
			t.Fatalf("create permission: %v", err)
		}
		if err := db.Model(&role).Association("PermissionList").Append(&perm); err != nil {
			t.Fatalf("bind permission: %v", err)
		}
	}
	user := createTestUser(t, db, 1, "user", 1)
	if err := db.Model(&user).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	return db, &AuthorizationService{db: db, rdb: rdb}, user.Id
}

// GetMyPermissions 返回纯权限码清单（011 裁决，无域前缀）。
func TestGetMyPermissionsReturnsPermissionCodes(t *testing.T) {
	_, service, accountId := setupPermissionCodeEnv(t)
	response, err := service.GetMyPermissions(auth.WithAccountId(context.Background(), accountId), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyPermissions() error = %v", err)
	}
	got := append([]string{}, response.PermissionCodeList...)
	slices.Sort(got)
	want := []string{"iam.permission.list", "iam.role.list"}
	if !slices.Equal(got, want) {
		t.Fatalf("PermissionCodeList = %v, want %v", response.PermissionCodeList, want)
	}
}

// 停用角色后其权限码即时失效。
func TestGetMyPermissionsExcludesDisabledRole(t *testing.T) {
	db, service, accountId := setupPermissionCodeEnv(t)
	if err := db.Model(model.Role{}).Where(model.RoleColumnId, 1).Update(model.RoleColumnStatus, model.StatusDisabled).Error; err != nil {
		t.Fatalf("disable role: %v", err)
	}
	response, err := service.GetMyPermissions(auth.WithAccountId(context.Background(), accountId), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyPermissions() error = %v", err)
	}
	if len(response.PermissionCodeList) != 0 {
		t.Fatalf("disabled role should yield empty permissions, got %v", response.PermissionCodeList)
	}
}

// CheckPermission 按权限码直接判定（011 裁决）。
func TestCheckPermissionMatchesCode(t *testing.T) {
	_, service, accountId := setupPermissionCodeEnv(t)
	ctx := auth.WithAccountId(context.Background(), accountId)

	resp, err := service.CheckPermission(ctx, &iam_v1.CheckPermissionRequest{
		PermissionCode: "iam.permission.list",
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if !resp.Ok {
		t.Fatal("CheckPermission() = false, want true for granted code")
	}

	// 未授权码
	resp, err = service.CheckPermission(ctx, &iam_v1.CheckPermissionRequest{
		PermissionCode: "iam.user.delete",
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if resp.Ok {
		t.Fatal("CheckPermission() = true, want false for ungranted code")
	}
}

// ── 管理员角色保护 ──

// 内置超管角色禁止删除（000014 起按 is_super_admin 标志判定，'admin' 码不再是全局键）。
func TestDeleteRoleRejectsAdminRole(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	service := NewAuthorizationService(db, nil)
	adminRole := createTestRole(t, db, 1, model.RoleAdmin, 1)
	if err := db.Model(&model.Role{}).Where(model.RoleColumnId, adminRole.Id).
		Update(model.RoleColumnIsSuperAdmin, true).Error; err != nil {
		t.Fatalf("set super admin flag: %v", err)
	}

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

// ── 权限码 CRUD ──

func TestCreatePermissionValidatesCodeFormat(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
	service := NewAuthorizationService(db, nil)
	ctx := appCtx(context.Background())

	cases := []struct {
		name    string
		code    string
		wantErr error
	}{
		{name: "缺少模块段", code: "iam.delete", wantErr: ErrInvalidPermissionCode},
		{name: "大写字母", code: "IAM.account.delete", wantErr: ErrInvalidPermissionCode},
		{name: "非法字符", code: "iam.account.delete!", wantErr: ErrInvalidPermissionCode},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{Name: "p", Code: tc.code})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreatePermission() error = %v, want %v", err, tc.wantErr)
			}
		})
	}

	created, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{
		Name: "合法权限", Code: "iam.account.delete",
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	if created.PermissionCode != "iam.account.delete" {
		t.Fatalf("created.PermissionCode = %q, want iam.account.delete", created.PermissionCode)
	}
}

func TestCreatePermissionCodeDuplicated(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
	service := NewAuthorizationService(db, nil)
	ctx := appCtx(context.Background())
	req := &iam_v1.CreatePermissionRequest{Name: "p", Code: "iam.dup.perm"}
	if _, err := service.CreatePermission(ctx, req); err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	if _, err := service.CreatePermission(ctx, req); !errors.Is(err, ErrPermissionCodeDuplicated) {
		t.Fatalf("CreatePermission() error = %v, want %v", err, ErrPermissionCodeDuplicated)
	}
}
