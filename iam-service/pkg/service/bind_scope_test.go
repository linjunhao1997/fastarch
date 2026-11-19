package service

import (
	"context"
	"errors"
	"testing"
	"time"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"
)

// bindScopeFixture 绑定隔离夹具：iam/sender 两应用、四个操作者（data scope=iam、
// data scope=sender、data scope=iam 非 admin、无 data scope）、两目标用户（接入 iam id=10、
// 接入 sender id=20）、四个角色：iam-role/sender-role、内置超管 admin（归 IAM、
// is_super_admin=1）、sender 应用级 admin（同名码、非超管）。
type bindScopeFixture struct {
	db              *gorm.DB
	accounts        *AccountService
	authz           *AuthorizationService
	ctxIam          context.Context
	ctxSender       context.Context
	ctxIamNonAdmin  context.Context
	ctxNoScope      context.Context
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
	// 数据范围由 dataAppId 注入 ctx（删除 home 后不再从用户行读取）。
	for _, spec := range []struct{ id, scope int64 }{{1, testAppId}, {2, otherAppId}, {3, testAppId}, {4, 0}} {
		createTestUser(t, db, spec.id, "bind-operator-"+cast.ToString(spec.id), 1)
	}
	// 目标用户归属由接入集合表达：10 接入 iam，20 接入 sender。
	createTestUser(t, db, 10, "bind-target", 1)
	createTestUser(t, db, 20, "bind-target-sender", 1)
	for _, spec := range []struct{ userId, appId int64 }{{10, testAppId}, {20, otherAppId}} {
		if err := db.Create(&model.RUserApplication{UserId: spec.userId, ApplicationId: spec.appId}).Error; err != nil {
			t.Fatalf("bind target application: %v", err)
		}
	}
	withScope := func(ctx context.Context, scope int64) context.Context {
		if scope > 0 {
			return appCtx(auth.WithDataAppId(ctx, scope))
		}
		return appCtx(ctx)
	}
	f := bindScopeFixture{
		db:             db,
		accounts:       NewAccountService(db, nil),
		authz:          NewAuthorizationService(db, nil),
		ctxIam:         withScope(auth.WithAccountId(context.Background(), 1), testAppId),
		ctxSender:      withScope(auth.WithAccountId(context.Background(), 2), otherAppId),
		ctxIamNonAdmin: withScope(auth.WithAccountId(context.Background(), 3), testAppId),
		ctxNoScope:     withScope(auth.WithAccountId(context.Background(), 4), 0),
		ctxAdmin:       appCtx(auth.WithAdmin(auth.WithAccountId(context.Background(), 99))),
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

// TestRoleBindScopeIsolation 验证绑定操作纳入数据边界（ADR 0006 补充、ADR-0009 改按 roleId）：
// UpdateAccountRoles 按全局唯一 roleId 加载角色，非 admin 逐个校验归属数据范围；
// 非 admin 一律禁绑内置超管角色（is_super_admin 标志，绑定即提权），admin/系统调用
// 旁路且可跨应用分配（ADR-0009 核心新能力）。
func TestRoleBindScopeIsolation(t *testing.T) {
	f := setupBindScopeFixture(t)

	// 非 admin 跨应用 roleId → ErrRoleOutOfScope（ADR-0009：不再按码解析，改按归属校验）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIam, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{cast.ToString(f.otherRole.Id)},
	}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("bind cross-scope role error = %v, want %v", err, ErrRoleOutOfScope)
	}
	// 同应用 roleId → 成功
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIam, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{cast.ToString(f.iamRole.Id)},
	}); err != nil {
		t.Fatalf("bind same-scope role: %v", err)
	}
	// 非 admin 禁绑内置超管角色（归属一致也拒绝）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIamNonAdmin, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{cast.ToString(f.adminRole.Id)},
	}); !errors.Is(err, ErrRoleAdminBindForbidden) {
		t.Fatalf("bind super admin role error = %v, want %v", err, ErrRoleAdminBindForbidden)
	}
	// 数据范围未注入 → fail-closed（目标用户边界先行拒绝）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxNoScope, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{cast.ToString(f.iamRole.Id)},
	}); !errors.Is(err, ErrAccountOutOfScope) {
		t.Fatalf("bind no-scope error = %v, want %v", err, ErrAccountOutOfScope)
	}
	// 不存在的 roleId → ErrRoleNotFound（fail-closed，不静默忽略）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIam, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{"999999"},
	}); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("bind missing role error = %v, want %v", err, ErrRoleNotFound)
	}
	// admin 绑内置超管角色 → 成功
	if _, err := f.accounts.UpdateAccountRoles(f.ctxAdmin, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{cast.ToString(f.adminRole.Id)},
	}); err != nil {
		t.Fatalf("admin bind super admin role: %v", err)
	}
	// admin 跨应用绑定 sender 角色 → 成功（ADR-0009 核心能力：超管不切数据范围亦可跨应用分配）
	if _, err := f.accounts.UpdateAccountRoles(f.ctxAdmin, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, AddedRoleIds: []string{cast.ToString(f.otherRole.Id)},
	}); err != nil {
		t.Fatalf("admin cross-app bind sender role: %v", err)
	}
	// sender 非 admin 绑 sender 应用级 'admin' 角色（同名码、非超管）→ 成功
	if _, err := f.accounts.UpdateAccountRoles(f.ctxSender, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, AddedRoleIds: []string{cast.ToString(f.senderAdminRole.Id)},
	}); err != nil {
		t.Fatalf("bind app-level admin role: %v", err)
	}
	// 系统调用（accountId=0）→ 隔离旁路，按 roleId 可绑任意应用角色
	if _, err := f.accounts.UpdateAccountRoles(appCtx(context.Background()), &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{cast.ToString(f.iamRole.Id)},
	}); err != nil {
		t.Fatalf("system bind role: %v", err)
	}
}

// TestCreateAccountRoleBindIsolation 验证 CreateAccount 初始角色绑定与
// UpdateAccountRoles 同门槛（ADR 0006 补充）：跨应用 角色与 admin 角色均拒绝。
func TestCreateAccountRoleBindIsolation(t *testing.T) {
	f := setupBindScopeFixture(t)

	if _, err := f.accounts.CreateAccount(f.ctxIam, &iam_v1.CreateAccountRequest{
		Username: "cross-home", Password: "password123",
		ApplicationIds:   []string{cast.ToString(testAppId)},
		RoleIds:           []string{cast.ToString(f.otherRole.Id)},
	}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("create with cross-home role error = %v, want %v", err, ErrRoleOutOfScope)
	}
	if _, err := f.accounts.CreateAccount(f.ctxIamNonAdmin, &iam_v1.CreateAccountRequest{
		Username: "admin-bound", Password: "password123",
		ApplicationIds:   []string{cast.ToString(testAppId)},
		RoleIds:           []string{cast.ToString(f.adminRole.Id)},
	}); !errors.Is(err, ErrRoleAdminBindForbidden) {
		t.Fatalf("create with admin role error = %v, want %v", err, ErrRoleAdminBindForbidden)
	}
	// admin 创建带 admin 角色的用户 → 成功
	if _, err := f.accounts.CreateAccount(f.ctxAdmin, &iam_v1.CreateAccountRequest{
		Username: "admin-created", Password: "password123",
		ApplicationIds:   []string{cast.ToString(testAppId)},
		RoleIds:           []string{cast.ToString(f.adminRole.Id)},
	}); err != nil {
		t.Fatalf("admin create with admin role: %v", err)
	}
}

// TestUpdateRolePermissionBindScopeIsolation 验证角色挂码纳入数据边界（ADR 0006 补充）：
// 非 admin 仅可把 数据范围一致的码挂到角色上，防止经挂码自授跨应用能力；
// 数据范围未注入 fail-closed；admin 旁路。
func TestUpdateRolePermissionBindScopeIsolation(t *testing.T) {
	f := setupBindScopeFixture(t)
	homePerm := createTestPermission(t, f.db, 21, "iam.role.bind", 1)
	otherPerm := createTestPermission(t, f.db, 22, "sender.record.send", 1)
	otherPerm.ApplicationId = f.otherRole.ApplicationId
	if err := f.db.Save(&otherPerm).Error; err != nil {
		t.Fatalf("set other permission app: %v", err)
	}

	// 跨应用 挂码 → ErrPermissionOutOfScope（角色本身 home 一致）
	if _, err := f.authz.UpdateRole(f.ctxIam, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version,
		AddedPermissionCodeList: []string{otherPerm.Code},
	}); !errors.Is(err, ErrPermissionOutOfScope) {
		t.Fatalf("bind cross-home permission error = %v, want %v", err, ErrPermissionOutOfScope)
	}
	// 数据范围码 → 成功，版本推进
	if _, err := f.authz.UpdateRole(f.ctxIam, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version,
		AddedPermissionCodeList: []string{homePerm.Code},
	}); err != nil {
		t.Fatalf("bind same-home permission: %v", err)
	}
	// 数据范围未注入 → fail-closed（角色自身边界先行拒绝，挂码校验不可达）
	if _, err := f.authz.UpdateRole(f.ctxNoScope, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version + 1,
		AddedPermissionCodeList: []string{homePerm.Code},
	}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("bind no-home error = %v, want %v", err, ErrRoleOutOfScope)
	}
	// admin 挂跨应用 码 → 成功
	if _, err := f.authz.UpdateRole(f.ctxAdmin, &iam_v1.UpdateRoleRequest{
		Id: f.iamRole.Id, Version: f.iamRole.Version + 1,
		AddedPermissionCodeList: []string{otherPerm.Code},
	}); err != nil {
		t.Fatalf("admin bind cross-home permission: %v", err)
	}
}

// TestRoleCodeAppScopedUniqueness 验证角色码应用内唯一（000014）：
// 跨应用同名码可各自创建（复合唯一索引），同应用重复拒绝；非 admin 创建强制归属数据范围。
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
	// 非 admin 同应用重复 → 拒绝（归属强制数据范围）
	if _, err := f.authz.CreateRole(f.ctxIam, &iam_v1.CreateRoleRequest{Name: "ops", RoleCode: "ops"}); !errors.Is(err, ErrRoleCodeDuplicated) {
		t.Fatalf("duplicate in home error = %v, want %v", err, ErrRoleCodeDuplicated)
	}
	// sender 非 admin 建同名码 → 归属强制 sender，撞 sender 自己的 ops（而非 iam 的）
	if _, err := f.authz.CreateRole(f.ctxSender, &iam_v1.CreateRoleRequest{Name: "ops", RoleCode: "ops"}); !errors.Is(err, ErrRoleCodeDuplicated) {
		t.Fatalf("sender duplicate error = %v, want %v", err, ErrRoleCodeDuplicated)
	}
}

// TestUpdateAccountRolesByIdResolution 验证绑定按全局唯一 roleId 精确落库（ADR-0009）：
// 两应用各有同名 'ops' 角色，按 id 绑定各自精确命中，无同名歧义；超管可跨应用
// 给 sender 用户绑 sender 角色，而非 admin 只能绑本数据范围角色。
func TestUpdateAccountRolesByIdResolution(t *testing.T) {
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

	// iam 操作者按 id 给 iam 用户绑 iam ops → 精确命中 iam ops
	if _, err := f.accounts.UpdateAccountRoles(f.ctxIam, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 10, AddedRoleIds: []string{cast.ToString(iamOps.Id)},
	}); err != nil {
		t.Fatalf("bind iam ops by id: %v", err)
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
	// sender 非 admin 按 id 给 sender 用户绑 sender ops → 精确命中 sender ops
	if _, err := f.accounts.UpdateAccountRoles(f.ctxSender, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, AddedRoleIds: []string{cast.ToString(senderOps.Id)},
	}); err != nil {
		t.Fatalf("bind sender ops by id: %v", err)
	}
	if err := f.db.Model(&model.RUserRole{}).
		Where(model.RUserRoleColumnUserId+" = ? AND "+model.RUserRoleColumnRoleId+" = ?", 20, senderOps.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count sender binding: %v", err)
	}
	if count != 1 {
		t.Fatalf("sender ops binding count = %d, want 1", count)
	}
	// 超管跨应用按 id 给 sender 用户绑 iam ops（同名另一应用）→ 成功且精确命中 iam ops
	if _, err := f.accounts.UpdateAccountRoles(f.ctxAdmin, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, AddedRoleIds: []string{cast.ToString(iamOps.Id)},
	}); err != nil {
		t.Fatalf("admin cross-app bind iam ops to sender user: %v", err)
	}
	if err := f.db.Model(&model.RUserRole{}).
		Where(model.RUserRoleColumnUserId+" = ? AND "+model.RUserRoleColumnRoleId+" = ?", 20, iamOps.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count cross-app binding: %v", err)
	}
	if count != 1 {
		t.Fatalf("admin cross-app iam ops binding count = %d, want 1", count)
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
		createTestUserWithPassword(t, f.db, spec.userId, spec.username, "password123", model.StatusEnabled)
		// 接入集合：登录 fail-closed 门槛要求非 admin 至少接入一个应用
		if err := f.db.Create(&model.RUserApplication{UserId: spec.userId, ApplicationId: testAppId}).Error; err != nil {
			t.Fatalf("bind admitted app for login: %v", err)
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

// TestRoleBindingRemovedScopeAndReadIsolation 验证 ADR-0009 读写两侧对称隔离：
// ① 写侧 fail-closed：sender 操作者解绑 iam 角色（越界 roleId）被拒，旧实现的
//    removed 裸码越权删 iam 角色路径被堵死；
// ② 读侧隔离：账号详情返回的角色列表按操作者数据范围过滤——sender 操作者看不到
//    目标账号的 iam 角色，超管可见全部。
func TestRoleBindingRemovedScopeAndReadIsolation(t *testing.T) {
	f := setupBindScopeFixture(t)
	// 夹具：目标用户 20（sender）已绑 sender role + iam role（模拟超管配的跨应用角色）
	for _, roleId := range []int64{f.otherRole.Id, f.iamRole.Id} {
		if err := f.db.Create(&model.RUserRole{UserId: 20, RoleId: roleId}).Error; err != nil {
			t.Fatalf("seed user role binding: %v", err)
		}
	}

	// ① sender 操作者解绑 iam 角色（越界 roleId）→ ErrRoleOutOfScope，绑定仍在
	if _, err := f.accounts.UpdateAccountRoles(f.ctxSender, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, RemovedRoleIds: []string{cast.ToString(f.iamRole.Id)},
	}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("sender remove iam role error = %v, want %v", err, ErrRoleOutOfScope)
	}
	var count int64
	if err := f.db.Model(&model.RUserRole{}).
		Where(model.RUserRoleColumnUserId+" = ? AND "+model.RUserRoleColumnRoleId+" = ?", 20, f.iamRole.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count iam binding after rejected remove: %v", err)
	}
	if count != 1 {
		t.Fatalf("iam role must remain bound after rejected remove, count = %d", count)
	}

	// sender 操作者解绑本应用角色 → 成功
	if _, err := f.accounts.UpdateAccountRoles(f.ctxSender, &iam_v1.UpdateAccountRolesRequest{
		AccountId: 20, RemovedRoleIds: []string{cast.ToString(f.otherRole.Id)},
	}); err != nil {
		t.Fatalf("sender remove sender role: %v", err)
	}

	// ② 读侧：重新补回两条绑定后分别以 sender / 超管视角读取
	if err := f.db.Create(&model.RUserRole{UserId: 20, RoleId: f.otherRole.Id}).Error; err != nil {
		t.Fatalf("reseed sender binding: %v", err)
	}
	senderView, err := f.accounts.GetAccount(f.ctxSender, &common_v1.GetRequest{Id: 20})
	if err != nil {
		t.Fatalf("sender GetAccount: %v", err)
	}
	for _, id := range senderView.RoleIds {
		if id == cast.ToString(f.iamRole.Id) {
			t.Fatalf("sender view must not expose iam role id in RoleIds: %v", senderView.RoleIds)
		}
	}
	for _, code := range senderView.RoleCodeList {
		if code == f.iamRole.Code {
			t.Fatalf("sender view must not expose iam role code: %v", senderView.RoleCodeList)
		}
	}
	adminView, err := f.accounts.GetAccount(f.ctxAdmin, &common_v1.GetRequest{Id: 20})
	if err != nil {
		t.Fatalf("admin GetAccount: %v", err)
	}
	foundIam := false
	for _, id := range adminView.RoleIds {
		if id == cast.ToString(f.iamRole.Id) {
			foundIam = true
		}
	}
	if !foundIam {
		t.Fatalf("admin view must expose iam role id, got %v", adminView.RoleIds)
	}
}
