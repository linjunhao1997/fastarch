package service

import (
	"context"
	"errors"
	"testing"
	"time"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"
)

// bindScopeFixture 绑定隔离夹具：iam/sender 两应用、四个操作者（home=iam、
// home=sender、home=iam 非 admin、无 home）、两目标用户（home=iam id=10、
// home=sender id=20）、四个角色：iam-role/sender-role、内置超管 admin（归 IAM、
// is_super_admin=1）、sender 应用级 admin（同名码、非超管）。
type bindScopeFixture struct {
	db              *gorm.DB
	accounts        *AccountService
	authz           *AuthorizationService
	ctxIam          context.Context
	ctxSender       context.Context
	ctxIamNonAdmin  context.Context
	ctxNoHome       context.Context
	ctxAdmin        context.Context
	iamRole         model.Role
	otherRole       model.Role
	adminRole       model.Role
	senderAdminRole model.Role
}

func setupBindScopeFixture(t *testing.T) bindScopeFixture {
	t.Helper()
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
	const otherAppId = int64(901)
	if err := db.Create(&model.Application{
		Id: otherAppId, AppId: "901", Code: "sender", Name: "sender", Status: model.StatusEnabled,
		Version: 1, CreateTime: time.Now(), UpdateTime: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create other application: %v", err)
	}
	for _, spec := range []struct{ id, home int64 }{{1, testAppId}, {2, otherAppId}, {3, testAppId}, {4, 0}} {
		operator := createTestUser(t, db, spec.id, "bind-operator-"+cast.ToString(spec.id), 1)
		operator.HomeApplicationId = spec.home
		if err := db.Save(&operator).Error; err != nil {
			t.Fatalf("set operator home: %v", err)
		}
	}
	target := createTestUser(t, db, 10, "bind-target", 1)
	target.HomeApplicationId = testAppId
	if err := db.Save(&target).Error; err != nil {
		t.Fatalf("set target home: %v", err)
	}
	target2 := createTestUser(t, db, 20, "bind-target-sender", 1)
	target2.HomeApplicationId = otherAppId
	if err := db.Save(&target2).Error; err != nil {
		t.Fatalf("set sender target home: %v", err)
	}
	f := bindScopeFixture{
		db:             db,
		accounts:       NewAccountService(db, nil),
		authz:          NewAuthorizationService(db, nil),
		ctxIam:         auth.WithAccountId(context.Background(), 1),
		ctxSender:      auth.WithAccountId(context.Background(), 2),
		ctxIamNonAdmin: auth.WithAccountId(context.Background(), 3),
		ctxNoHome:      auth.WithAccountId(context.Background(), 4),
		ctxAdmin:       auth.WithAdmin(auth.WithAccountId(context.Background(), 99)),
	}
	f.iamRole = createTestRole(t, db, 11, "iam-role", 1)
	f.iamRole.ApplicationId = testAppId
	if err := db.Save(&f.iamRole).Error; err != nil {
		t.Fatalf("set iam role app: %v", err)
	}
	f.otherRole = createTestRole(t, db, 12, "sender-role", 1)
	f.otherRole.ApplicationId = otherAppId
	if err := db.Save(&f.otherRole).Error; err != nil {
		t.Fatalf("set sender role app: %v", err)
	}
	f.adminRole = createTestRole(t, db, 13, model.RoleAdmin, 1)
	f.adminRole.ApplicationId = testAppId
	f.adminRole.IsSuperAdmin = true
	if err := db.Save(&f.adminRole).Error; err != nil {
		t.Fatalf("set admin role app: %v", err)
	}
	// sender 应用级 'admin' 角色（同名码、非超管标志）：可绑可删，不授予平台超管
	f.senderAdminRole = createTestRole(t, db, 14, model.RoleAdmin, 1)
	f.senderAdminRole.ApplicationId = otherAppId
	if err := db.Save(&f.senderAdminRole).Error; err != nil {
		t.Fatalf("set sender admin role app: %v", err)
	}
	return f
}

// TestRoleBindHomeIsolation 验证绑定操作纳入数据边界（ADR 0006 补充裁决）：
// UpdateAccountRoles 待绑定角色逐个校验归属 home；非 admin 一律禁绑内置 admin
// 角色（登录按角色码计算 IsAdmin，绑定即提权），即使操作者 home 与 admin 角色
// 归属一致也拒绝；admin/系统调用旁路。
func TestRoleBindHomeIsolation(t *testing.T) {
	f := setupBindScopeFixture(t)

	// 跨 home 绑定 → 码按目标 home 解析不可达（000014 起应用内唯一）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIam, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{f.otherRole.Code},
	}); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("bind cross-home role error = %v, want %v", err, ErrRoleNotFound)
	}
	// 同 home 绑定 → 成功
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIam, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{f.iamRole.Code},
	}); err != nil {
		t.Fatalf("bind same-home role: %v", err)
	}
	// 非 admin 禁绑 admin 角色（home=IAM 与 admin 角色归属一致也拒绝）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIamNonAdmin, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{model.RoleAdmin},
	}); !errors.Is(err, ErrRoleAdminBindForbidden) {
		t.Fatalf("bind admin role error = %v, want %v", err, ErrRoleAdminBindForbidden)
	}
	// home 未设置 → fail-closed（目标用户边界先行拒绝）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxNoHome, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{f.iamRole.Code},
	}); !errors.Is(err, ErrAccountOutOfScope) {
		t.Fatalf("bind no-home error = %v, want %v", err, ErrAccountOutOfScope)
	}
	// admin 绑内置超管角色 → 成功（目标 home=IAM，码解析到内置角色）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxAdmin, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{model.RoleAdmin},
	}); err != nil {
		t.Fatalf("admin bind admin role: %v", err)
	}
	// sender 非 admin 绑 sender 应用级 'admin' 角色（同名码、非超管）→ 成功
	if _, err := f.accounts.UpdateAccountRoles(f.ctxSender, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, AddedRoleCodeList: []string{f.senderAdminRole.Code},
	}); err != nil {
		t.Fatalf("bind app-level admin role: %v", err)
	}
	// 系统调用（无认证上下文）→ 隔离旁路，但码同样按目标 home 解析：
	// 同 home 码可绑，跨 home 码不可达
	if _, err := f.accounts.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{f.iamRole.Code},
	}); err != nil {
		t.Fatalf("system bind same-home role: %v", err)
	}
	if _, err := f.accounts.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{f.otherRole.Code},
	}); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("system bind cross-home role error = %v, want %v", err, ErrRoleNotFound)
	}
}

// TestCreateAccountRoleBindIsolation 验证 CreateAccount 初始角色绑定与
// UpdateAccountRoles 同门槛（ADR 0006 补充）：跨 home 角色与 admin 角色均拒绝。
func TestCreateAccountRoleBindIsolation(t *testing.T) {
	f := setupBindScopeFixture(t)

	if _, err := f.accounts.CreateAccount(f.ctxIam, &iam_v1.CreateAccountRequest{
		Username: "cross-home", Password: "password123",
		HomeApplicationId: cast.ToString(testAppId),
		RoleIds:           []string{cast.ToString(f.otherRole.Id)},
	}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("create with cross-home role error = %v, want %v", err, ErrRoleOutOfScope)
	}
	if _, err := f.accounts.CreateAccount(f.ctxIamNonAdmin, &iam_v1.CreateAccountRequest{
		Username: "admin-bound", Password: "password123",
		HomeApplicationId: cast.ToString(testAppId),
		RoleIds:           []string{cast.ToString(f.adminRole.Id)},
	}); !errors.Is(err, ErrRoleAdminBindForbidden) {
		t.Fatalf("create with admin role error = %v, want %v", err, ErrRoleAdminBindForbidden)
	}
	// admin 创建带 admin 角色的用户 → 成功
	if _, err := f.accounts.CreateAccount(f.ctxAdmin, &iam_v1.CreateAccountRequest{
		Username: "admin-created", Password: "password123",
		HomeApplicationId: cast.ToString(testAppId),
		RoleIds:           []string{cast.ToString(f.adminRole.Id)},
	}); err != nil {
		t.Fatalf("admin create with admin role: %v", err)
	}
}

// TestUpdateRolePermissionBindIsolation 验证角色挂码纳入数据边界（ADR 0006 补充）：
// 非 admin 仅可把 home 一致的码挂到角色上，防止经挂码自授跨应用能力；
// home 未设置 fail-closed；admin 旁路。
func TestUpdateRolePermissionBindIsolation(t *testing.T) {
	f := setupBindScopeFixture(t)
	homePerm := createTestPermission(t, f.db, 21, "iam.role.bind", 1)
	otherPerm := createTestPermission(t, f.db, 22, "sender.record.send", 1)
	otherPerm.ApplicationId = f.otherRole.ApplicationId
	if err := f.db.Save(&otherPerm).Error; err != nil {
		t.Fatalf("set other permission app: %v", err)
	}

	// 跨 home 挂码 → ErrPermissionOutOfScope（角色本身 home 一致）
	if _, err := f.authz.UpdateRole(f.ctxIam, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version,
		AddedPermissionCodeList: []string{otherPerm.Code},
	}); !errors.Is(err, ErrPermissionOutOfScope) {
		t.Fatalf("bind cross-home permission error = %v, want %v", err, ErrPermissionOutOfScope)
	}
	// home 码 → 成功，版本推进
	if _, err := f.authz.UpdateRole(f.ctxIam, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version,
		AddedPermissionCodeList: []string{homePerm.Code},
	}); err != nil {
		t.Fatalf("bind same-home permission: %v", err)
	}
	// home 未设置 → fail-closed（角色自身边界先行拒绝，挂码校验不可达）
	if _, err := f.authz.UpdateRole(f.ctxNoHome, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version + 1,
		AddedPermissionCodeList: []string{homePerm.Code},
	}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("bind no-home error = %v, want %v", err, ErrRoleOutOfScope)
	}
	// admin 挂跨 home 码 → 成功
	if _, err := f.authz.UpdateRole(f.ctxAdmin, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version + 1,
		AddedPermissionCodeList: []string{otherPerm.Code},
	}); err != nil {
		t.Fatalf("admin bind cross-home permission: %v", err)
	}
}

// TestRoleCodeAppScopedUniqueness 验证角色码应用内唯一（000014）：
// 跨应用同名码可各自创建（复合唯一索引），同应用重复拒绝；非 admin 创建强制归属 home。
func TestRoleCodeAppScopedUniqueness(t *testing.T) {
	f := setupBindScopeFixture(t)

	first, err := f.authz.CreateRole(f.ctxAdmin, &iam_v1.CreateRoleRequest{Name: "ops", RoleCode: "ops", ApplicationId: testAppId})
	if err != nil {
		t.Fatalf("create ops in iam: %v", err)
	}
	second, err := f.authz.CreateRole(f.ctxAdmin, &iam_v1.CreateRoleRequest{Name: "ops", RoleCode: "ops", ApplicationId: f.otherRole.ApplicationId})
	if err != nil {
		t.Fatalf("create ops in sender (cross-app same code): %v", err)
	}
	if first.ApplicationId != testAppId || second.ApplicationId != f.otherRole.ApplicationId {
		t.Fatalf("application ids = %d/%d, want %d/%d", first.ApplicationId, second.ApplicationId, testAppId, f.otherRole.ApplicationId)
	}
	// 非 admin 同应用重复 → 拒绝（归属强制 home）
	if _, err := f.authz.CreateRole(f.ctxIam, &iam_v1.CreateRoleRequest{Name: "ops", RoleCode: "ops"}); !errors.Is(err, ErrRoleCodeDuplicated) {
		t.Fatalf("duplicate in home error = %v, want %v", err, ErrRoleCodeDuplicated)
	}
	// sender 非 admin 建同名码 → 归属强制 sender，撞 sender 自己的 ops（而非 iam 的）
	if _, err := f.authz.CreateRole(f.ctxSender, &iam_v1.CreateRoleRequest{Name: "ops", RoleCode: "ops"}); !errors.Is(err, ErrRoleCodeDuplicated) {
		t.Fatalf("sender duplicate error = %v, want %v", err, ErrRoleCodeDuplicated)
	}
}

// TestUpdateAccountRolesCodeResolution 验证同名码按目标用户 home 解析（000014）：
// 两应用各有 'ops' 角色时，绑定落库指向目标 home 应用的角色，无歧义。
func TestUpdateAccountRolesCodeResolution(t *testing.T) {
	f := setupBindScopeFixture(t)
	iamOps := createTestRole(t, f.db, 15, "ops", 1)
	iamOps.ApplicationId = testAppId
	if err := f.db.Save(&iamOps).Error; err != nil {
		t.Fatalf("set iam ops app: %v", err)
	}
	senderOps := createTestRole(t, f.db, 16, "ops", 1)
	senderOps.ApplicationId = f.otherRole.ApplicationId
	if err := f.db.Save(&senderOps).Error; err != nil {
		t.Fatalf("set sender ops app: %v", err)
	}

	// iam 操作者给 iam 用户绑 'ops' → 解析 iam 的 ops
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIam, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleCodeList: []string{"ops"},
	}); err != nil {
		t.Fatalf("bind ops for iam target: %v", err)
	}
	var count int64
	if err := f.db.Model(&model.RUserRole{}).
		Where(model.RUserRoleColumnUserId+" = ? AND "+model.RUserRoleColumnRoleId+" = ?", 10, iamOps.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count binding: %v", err)
	}
	if count != 1 {
		t.Fatalf("iam ops binding count = %d, want 1", count)
	}
	if err := f.db.Model(&model.RUserRole{}).
		Where(model.RUserRoleColumnUserId+" = ? AND "+model.RUserRoleColumnRoleId+" = ?", 10, senderOps.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count wrong binding: %v", err)
	}
	if count != 0 {
		t.Fatalf("sender ops must not be bound to iam target, count = %d", count)
	}
	// sender 操作者给 sender 用户绑 'ops' → 解析 sender 的 ops
	if _, err := f.accounts.UpdateAccountRoles(f.ctxSender, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, AddedRoleCodeList: []string{"ops"},
	}); err != nil {
		t.Fatalf("bind ops for sender target: %v", err)
	}
	if err := f.db.Model(&model.RUserRole{}).
		Where(model.RUserRoleColumnUserId+" = ? AND "+model.RUserRoleColumnRoleId+" = ?", 20, senderOps.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count sender binding: %v", err)
	}
	if count != 1 {
		t.Fatalf("sender ops binding count = %d, want 1", count)
	}
}

// TestLoginIsAdminBySuperFlag 验证登录超管判定按 is_super_admin 标志（000014）：
// 应用级 'admin' 码角色不授予平台超管；标志角色无论码为何均授予。
func TestLoginIsAdminBySuperFlag(t *testing.T) {
	f := setupBindScopeFixture(t)
	rdb, _ := setupTestRedis(t)
	svc := NewAuthenticationService(f.db, rdb)
	// 码非 'admin' 但标志为真的角色：判定只认标志
	flagRole := createTestRole(t, f.db, 17, "superuser", 1)
	flagRole.ApplicationId = testAppId
	flagRole.IsSuperAdmin = true
	if err := f.db.Save(&flagRole).Error; err != nil {
		t.Fatalf("set flag role: %v", err)
	}

	for _, spec := range []struct {
		userId    int64
		username  string
		roleId    int64
		wantAdmin bool
	}{
		{30, "app-admin-user", f.senderAdminRole.Id, false},
		{31, "super-admin-user", f.adminRole.Id, true},
		{32, "flag-role-user", flagRole.Id, true},
	} {
		user := createTestUserWithPassword(t, f.db, spec.userId, spec.username, "password123", model.StatusEnabled)
		user.HomeApplicationId = testAppId
		if err := f.db.Save(&user).Error; err != nil {
			t.Fatalf("set login user home: %v", err)
		}
		// 夹具级直接写关联表（绕过服务层绑定校验，聚焦登录判定本身）
		if err := f.db.Create(&model.RUserRole{UserId: spec.userId, RoleId: spec.roleId}).Error; err != nil {
			t.Fatalf("bind role for login: %v", err)
		}
		ctx := grpc.NewContextWithServerTransportStream(context.Background(), stubServerTransportStream{})
		resp, err := svc.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
			Identifier: spec.username, Password: "password123",
		})
		if err != nil {
			t.Fatalf("%s login: %v", spec.username, err)
		}
		claims, err := auth.ParseClaims(resp.AccessToken)
		if err != nil {
			t.Fatalf("%s parse claims: %v", spec.username, err)
		}
		if claims.IsAdmin != spec.wantAdmin {
			t.Fatalf("%s IsAdmin = %v, want %v", spec.username, claims.IsAdmin, spec.wantAdmin)
		}
	}
}

// TestDeleteRoleAppLevelAdmin 验证删除保护按超管标志（000014）：
// 应用级 'admin' 码角色可由本应用非 admin 删除；内置超管角色 admin 也禁删。
func TestDeleteRoleAppLevelAdmin(t *testing.T) {
	f := setupBindScopeFixture(t)

	if _, err := f.authz.DeleteRole(f.ctxSender, &iam_v1.DeleteRoleRequest{Id: f.senderAdminRole.Id}); err != nil {
		t.Fatalf("delete app-level admin role: %v", err)
	}
	if _, err := f.authz.DeleteRole(f.ctxAdmin, &iam_v1.DeleteRoleRequest{Id: f.adminRole.Id}); !errors.Is(err, ErrRoleAdminProtected) {
		t.Fatalf("delete super admin role error = %v, want %v", err, ErrRoleAdminProtected)
	}
}
