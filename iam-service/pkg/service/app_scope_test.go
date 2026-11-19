package service

import (
	"context"
	"testing"
	"time"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	contribmodel "github.com/linjunhao1997/contrib/model"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	common_dict_v1 "github.com/linjunhao1997/idl/pb/common/dict"
	common_menu_v1 "github.com/linjunhao1997/idl/pb/common/menu"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

// 多应用数据隔离回归（裁决 #8）：应用上下文由 token 承载（ctx 注入），
// 菜单/字典/权限管理数据按应用隔离；未选应用 fail-closed。

const (
	testAppIdIam   = int64(900)
	testAppIdOrder = int64(901)
)

func createTestApp(t *testing.T, db *gorm.DB, id int64, code string, status string) model.Application {
	t.Helper()
	app := model.Application{
		Id: id, AppId: "app-" + code, Code: code, Name: code + " 应用",
		Status: status, Version: 1, CreateTime: time.Now(), UpdateTime: time.Now(),
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app %s: %v", code, err)
	}
	return app
}

func ctxWithApp(appId int64) context.Context {
	return auth.WithApplicationId(auth.WithAccountId(context.Background(), 1), appId)
}

func createTestMenu(t *testing.T, db *gorm.DB, id int64, appId int64, name string, code string) {
	t.Helper()
	menu := contribmodel.Menu{
		Id: id, ApplicationId: appId, Name: name, Path: "/" + name,
		PermissionCode: code, Status: model.StatusEnabled, Version: 1,
		CreateTime: time.Now(), UpdateTime: time.Now(),
	}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatalf("create menu %s: %v", name, err)
	}
}

// ── 应用上下文缺失 fail-closed ──────────────────────────────────────────

func TestAppScopedRpcRequiresApplicationContext(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	menuSvc := NewMenuService(db)
	dictSvc := NewDictService(db)
	authzSvc := NewAuthorizationService(db, nil)
	bare := auth.WithAccountId(context.Background(), 1)

	if _, err := menuSvc.ListMenuTree(bare, &emptypb.Empty{}); err != ErrApplicationContextRequired {
		t.Fatalf("ListMenuTree() error = %v, want %v", err, ErrApplicationContextRequired)
	}
	if _, err := menuSvc.ListMyMenuTree(bare, &emptypb.Empty{}); err != ErrApplicationContextRequired {
		t.Fatalf("ListMyMenuTree() error = %v, want %v", err, ErrApplicationContextRequired)
	}
	if _, err := dictSvc.ListDictTypes(bare, &common_dict_v1.ListDictTypesRequest{}); err != ErrApplicationContextRequired {
		t.Fatalf("ListDictTypes() error = %v, want %v", err, ErrApplicationContextRequired)
	}
	if _, err := authzSvc.ListPermissions(bare, &iam_v1.ListPermissionsRequest{}); err != ErrApplicationContextRequired {
		t.Fatalf("ListPermissions() error = %v, want %v", err, ErrApplicationContextRequired)
	}
}

// ── 菜单按应用隔离 ──────────────────────────────────────────────────────

func TestMenuTreeScopedByApplication(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	createTestMenu(t, db, 1, testAppIdIam, "iam-menu", "")
	createTestMenu(t, db, 2, testAppIdOrder, "order-menu", "")
	svc := NewMenuService(db)

	resp, err := svc.ListMenuTree(ctxWithApp(testAppIdIam), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListMenuTree() error = %v", err)
	}
	if len(resp.Roots) != 1 || resp.Roots[0].Menu.Name != "iam-menu" {
		t.Fatalf("ListMenuTree() roots = %v, want only iam-menu", resp.Roots)
	}

	// 管理员的 my-menu-tree 同样限定当前应用
	adminCtx := auth.WithAdmin(ctxWithApp(testAppIdOrder))
	myResp, err := svc.ListMyMenuTree(adminCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListMyMenuTree() error = %v", err)
	}
	if len(myResp.Roots) != 1 || myResp.Roots[0].Menu.Name != "order-menu" {
		t.Fatalf("ListMyMenuTree() roots = %v, want only order-menu", myResp.Roots)
	}
}

func TestCreateMenuRejectsForeignSegmentCode(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	svc := NewMenuService(db)

	_, err := svc.CreateMenu(ctxWithApp(testAppIdIam), &common_menu_v1.CreateMenuRequest{
		Name: "越权菜单", PermissionCode: "order.menu.list",
	})
	if err != ErrPermissionCodeSegmentMismatch {
		t.Fatalf("CreateMenu() error = %v, want %v", err, ErrPermissionCodeSegmentMismatch)
	}
	// 本应用服务段的码放行
	if _, err := svc.CreateMenu(ctxWithApp(testAppIdIam), &common_menu_v1.CreateMenuRequest{
		Name: "用户管理", PermissionCode: "iam.user.list",
	}); err != nil {
		t.Fatalf("CreateMenu() with own-segment code error = %v", err)
	}
}

// ── 字典按应用隔离：code 应用内唯一 ─────────────────────────────────────

func TestDictTypeCodeUniquePerApp(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	svc := NewDictService(db)

	if _, err := svc.CreateDictType(ctxWithApp(testAppIdIam), &common_dict_v1.CreateDictTypeRequest{Code: "gender", Name: "性别"}); err != nil {
		t.Fatalf("CreateDictType(iam) error = %v", err)
	}
	// 另一应用可建同名 code（应用内唯一）
	if _, err := svc.CreateDictType(ctxWithApp(testAppIdOrder), &common_dict_v1.CreateDictTypeRequest{Code: "gender", Name: "性别"}); err != nil {
		t.Fatalf("CreateDictType(order) error = %v", err)
	}
	// 同应用重复 code 拒绝
	if _, err := svc.CreateDictType(ctxWithApp(testAppIdIam), &common_dict_v1.CreateDictTypeRequest{Code: "gender", Name: "性别2"}); err != ErrDictTypeCodeDuplicated {
		t.Fatalf("CreateDictType(iam dup) error = %v, want %v", err, ErrDictTypeCodeDuplicated)
	}
	// 列表只见本应用
	resp, err := svc.ListDictTypes(ctxWithApp(testAppIdIam), &common_dict_v1.ListDictTypesRequest{CurrentPage: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListDictTypes() error = %v", err)
	}
	if len(resp.DictTypes) != 1 {
		t.Fatalf("ListDictTypes() = %d types, want 1 (iam only)", len(resp.DictTypes))
	}
}

func TestDictItemScopedByTypeApplication(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	svc := NewDictService(db)
	created, err := svc.CreateDictType(ctxWithApp(testAppIdOrder), &common_dict_v1.CreateDictTypeRequest{Code: "gender", Name: "性别"})
	if err != nil {
		t.Fatalf("CreateDictType(order) error = %v", err)
	}
	// 在 iam 应用上下文里给 order 的字典类型建条目 → 拒绝（跨应用）
	_, err = svc.CreateDictItem(ctxWithApp(testAppIdIam), &common_dict_v1.CreateDictItemRequest{
		TypeId: cast.ToInt64(created.Id), Label: "男", Value: "male",
	})
	if err == nil {
		t.Fatal("CreateDictItem() cross-app should fail")
	}
	// 本应用上下文放行
	if _, err := svc.CreateDictItem(ctxWithApp(testAppIdOrder), &common_dict_v1.CreateDictItemRequest{
		TypeId: cast.ToInt64(created.Id), Label: "男", Value: "male",
	}); err != nil {
		t.Fatalf("CreateDictItem() own-app error = %v", err)
	}
}

// ── 权限管理数据按所属应用隔离（授予仍全局）──────────────────────────────

func TestListPermissionsScopedByApplication(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	operator := createTestUser(t, db, 1, "permission-list-operator", 1)
	operator.HomeApplicationId = testAppIdIam
	if err := db.Save(&operator).Error; err != nil {
		t.Fatalf("set operator home: %v", err)
	}
	iamPerm := createTestPermission(t, db, 1, "iam.user.list", 1)
	orderPerm := model.Permission{
		Id: 2, Name: "order", Code: "order.list.all", ApplicationId: testAppIdOrder,
		Status: model.StatusEnabled, Type: "api", Version: 1,
		CreateTime: time.Now(), UpdateTime: time.Now(),
	}
	if err := db.Create(&orderPerm).Error; err != nil {
		t.Fatalf("create order permission: %v", err)
	}
	_ = iamPerm
	svc := NewAuthorizationService(db, nil)

	resp, err := svc.ListPermissions(ctxWithApp(testAppIdIam), &iam_v1.ListPermissionsRequest{CurrentPage: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListPermissions() error = %v", err)
	}
	if len(resp.Permissions) != 1 || resp.Permissions[0].PermissionCode != "iam.user.list" {
		t.Fatalf("ListPermissions(iam) = %v, want only iam.user.list", resp.Permissions)
	}
}

// ── 应用服务段编码唯一 ─────────────────────────────────────────────────

func TestCreateApplicationCodeDuplicated(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	svc := NewApplicationService(db)
	if _, err := svc.CreateApplication(context.Background(), &iam_v1.CreateApplicationRequest{Name: "订单", Code: "order"}); err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if _, err := svc.CreateApplication(context.Background(), &iam_v1.CreateApplicationRequest{Name: "订单2", Code: "order"}); err != ErrApplicationCodeDuplicated {
		t.Fatalf("CreateApplication(dup) error = %v, want %v", err, ErrApplicationCodeDuplicated)
	}
}


// ── 用户应用准入分配（全量覆盖）─────────────────────────────────────────

func TestUpdateAccountApplications(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	user := createTestUser(t, db, 5, "scoped-user", 1)
	svc := &AccountService{db: db, rdb: rdb}
	// 分配准入/所属为 admin 视角（数据边界 ADR 0006：非 admin 受 home 隔离）
	ctx := auth.WithAdmin(auth.WithAccountId(context.Background(), 1))

	// 初始为空
	got, err := svc.GetAccount(ctx, &common_v1.GetRequest{Id: user.Id})
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if len(got.Applications) != 0 {
		t.Fatalf("initial Applications = %v, want empty", got.Applications)
	}
	// 全量覆盖分配两个应用
	_, err = svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: user.Id, ApplicationIds: []string{"900", "901"},
	})
	if err != nil {
		t.Fatalf("UpdateAccountApplications() error = %v", err)
	}
	got, _ = svc.GetAccount(ctx, &common_v1.GetRequest{Id: user.Id})
	if len(got.Applications) != 2 {
		t.Fatalf("Applications = %v, want 2 apps", got.Applications)
	}
	// 所属应用必须属于准入集合
	_, err = svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: user.Id, ApplicationIds: []string{"900", "901"}, HomeApplicationId: "900",
	})
	if err != nil {
		t.Fatalf("UpdateAccountApplications(home) error = %v", err)
	}
	got, _ = svc.GetAccount(ctx, &common_v1.GetRequest{Id: user.Id})
	if got.HomeApplicationId != "900" {
		t.Fatalf("HomeApplicationId = %q, want 900", got.HomeApplicationId)
	}
	// 非准入应用不能设为所属应用
	_, err = svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: user.Id, ApplicationIds: []string{"900", "901"}, HomeApplicationId: "999",
	})
	if err != ErrHomeApplicationInvalid {
		t.Fatalf("UpdateAccountApplications(bad home) error = %v, want %v", err, ErrHomeApplicationInvalid)
	}
	// 所属应用无需出现在准入集合（隐式准入）
	_, err = svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: user.Id, ApplicationIds: []string{"901"}, HomeApplicationId: "900",
	})
	if err != nil {
		t.Fatalf("UpdateAccountApplications(implicit home) error = %v", err)
	}
	got, _ = svc.GetAccount(ctx, &common_v1.GetRequest{Id: user.Id})
	if len(got.Applications) != 1 || got.Applications[0].Id != "901" {
		t.Fatalf("Applications = %v, want [901]", got.Applications)
	}
	if got.HomeApplicationId != "900" {
		t.Fatalf("HomeApplicationId = %q, want 900 (implicit)", got.HomeApplicationId)
	}
	// 清空准入不影响所属应用
	_, err = svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: user.Id, ApplicationIds: []string{},
	})
	if err != nil {
		t.Fatalf("UpdateAccountApplications(clear) error = %v", err)
	}
	got, _ = svc.GetAccount(ctx, &common_v1.GetRequest{Id: user.Id})
	if len(got.Applications) != 0 {
		t.Fatalf("Applications = %v, want empty after clear", got.Applications)
	}
	// 不存在的应用拒绝
	_, err = svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: user.Id, ApplicationIds: []string{"12345"},
	})
	if err != ErrApplicationNotFoundForScope {
		t.Fatalf("UpdateAccountApplications(missing) error = %v, want %v", err, ErrApplicationNotFoundForScope)
	}
}

// ── 数据边界（ADR 0006）：所属应用 = 数据边界 ─────────────────────────────

func TestCreateAccountHomeRequired(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	disabled := createTestApp(t, db, 902, "crm", model.StatusDisabled)
	svc := &AccountService{db: db}
	ctx := auth.WithAdmin(auth.WithAccountId(context.Background(), 1))

	// 缺 home 拒绝
	if _, err := svc.CreateAccount(ctx, &iam_v1.CreateAccountRequest{
		Username: "u1", Password: "password123",
	}); err != ErrHomeApplicationRequired {
		t.Fatalf("CreateAccount(no home) error = %v, want %v", err, ErrHomeApplicationRequired)
	}
	// 停用 home 拒绝
	if _, err := svc.CreateAccount(ctx, &iam_v1.CreateAccountRequest{
		Username: "u1", Password: "password123", HomeApplicationId: cast.ToString(disabled.Id),
	}); err != ErrHomeApplicationInvalid {
		t.Fatalf("CreateAccount(disabled home) error = %v, want %v", err, ErrHomeApplicationInvalid)
	}
	// 不存在 home 拒绝
	if _, err := svc.CreateAccount(ctx, &iam_v1.CreateAccountRequest{
		Username: "u1", Password: "password123", HomeApplicationId: "999",
	}); err != ErrHomeApplicationInvalid {
		t.Fatalf("CreateAccount(missing home) error = %v, want %v", err, ErrHomeApplicationInvalid)
	}
	// 不存在准入应用拒绝
	if _, err := svc.CreateAccount(ctx, &iam_v1.CreateAccountRequest{
		Username: "u2", Password: "password123", HomeApplicationId: "900", ApplicationIds: []string{"12345"},
	}); err != ErrApplicationNotFoundForScope {
		t.Fatalf("CreateAccount(missing admitted) error = %v, want %v", err, ErrApplicationNotFoundForScope)
	}
	// 成功：application_ids 含 home 自动剔除（隐式准入）+ 去重，响应回填 home+准入
	created, err := svc.CreateAccount(ctx, &iam_v1.CreateAccountRequest{
		Username: "u1", Password: "password123", HomeApplicationId: "900",
		ApplicationIds: []string{"900", "901", "901"},
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.HomeApplicationId != "900" {
		t.Fatalf("HomeApplicationId = %q, want 900", created.HomeApplicationId)
	}
	if created.HomeApplication == nil || created.HomeApplication.Code != "iam" {
		t.Fatalf("HomeApplication = %v, want iam summary", created.HomeApplication)
	}
	if len(created.Applications) != 1 || created.Applications[0].Id != "901" {
		t.Fatalf("Applications = %v, want [901] (home excluded)", created.Applications)
	}
	var bindings []model.RUserApplication
	if err := db.Where(model.RUserApplicationColumnUserId, cast.ToInt64(created.Id)).Find(&bindings).Error; err != nil {
		t.Fatalf("query bindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].ApplicationId != testAppIdOrder {
		t.Fatalf("bindings = %v, want single order binding (home implicit)", bindings)
	}
}

func TestCreateAccountNonAdminHomeLocked(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	operator := createTestUser(t, db, 1, "op", 1)
	operator.HomeApplicationId = testAppIdIam
	if err := db.Save(&operator).Error; err != nil {
		t.Fatalf("save operator home: %v", err)
	}
	createTestUser(t, db, 2, "op-nohome", 1)
	svc := &AccountService{db: db}

	// 跨 home 创建拒绝
	ctx := auth.WithAccountId(context.Background(), 1)
	if _, err := svc.CreateAccount(ctx, &iam_v1.CreateAccountRequest{
		Username: "x1", Password: "password123", HomeApplicationId: "901",
	}); err != ErrAccountOutOfScope {
		t.Fatalf("CreateAccount(cross home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	// 同 home 创建放行
	if _, err := svc.CreateAccount(ctx, &iam_v1.CreateAccountRequest{
		Username: "x2", Password: "password123", HomeApplicationId: "900",
	}); err != nil {
		t.Fatalf("CreateAccount(own home) error = %v", err)
	}
	// 操作者未设置 home 拒绝（fail-closed）
	ctx0 := auth.WithAccountId(context.Background(), 2)
	if _, err := svc.CreateAccount(ctx0, &iam_v1.CreateAccountRequest{
		Username: "x3", Password: "password123", HomeApplicationId: "900",
	}); err != ErrAccountOutOfScope {
		t.Fatalf("CreateAccount(operator no home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
}

func TestListAccountsHomeScoped(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	operator := createTestUser(t, db, 1, "op", 1)
	operator.HomeApplicationId = testAppIdIam
	if err := db.Save(&operator).Error; err != nil {
		t.Fatalf("save operator home: %v", err)
	}
	createTestUser(t, db, 2, "op-nohome", 1)
	for _, spec := range []struct {
		id   int64
		name string
		home int64
	}{
		{3, "iam-a", testAppIdIam},
		{4, "iam-b", testAppIdIam},
		{5, "order-a", testAppIdOrder},
	} {
		u := createTestUser(t, db, spec.id, spec.name, 1)
		u.HomeApplicationId = spec.home
		if err := db.Save(&u).Error; err != nil {
			t.Fatalf("save user home: %v", err)
		}
	}
	svc := &AccountService{db: db}

	// 非 admin 仅见所属应用的用户（含操作者自身）
	resp, err := svc.ListAccounts(auth.WithAccountId(context.Background(), 1), &iam_v1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts(scoped) error = %v", err)
	}
	if len(resp.Accounts) != 3 {
		t.Fatalf("ListAccounts(scoped) = %d accounts, want 3 (home=iam)", len(resp.Accounts))
	}
	// admin 全量
	adminResp, err := svc.ListAccounts(auth.WithAdmin(auth.WithAccountId(context.Background(), 1)), &iam_v1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts(admin) error = %v", err)
	}
	if len(adminResp.Accounts) != 5 {
		t.Fatalf("ListAccounts(admin) = %d accounts, want 5", len(adminResp.Accounts))
	}
	// 未设置 home 的非 admin fail-closed 空列表
	emptyResp, err := svc.ListAccounts(auth.WithAccountId(context.Background(), 2), &iam_v1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts(no home) error = %v", err)
	}
	if len(emptyResp.Accounts) != 0 {
		t.Fatalf("ListAccounts(no home) = %d accounts, want 0", len(emptyResp.Accounts))
	}
}

func TestAccountOperationsCrossHomeRejected(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	operator := createTestUser(t, db, 1, "op", 1)
	operator.HomeApplicationId = testAppIdIam
	if err := db.Save(&operator).Error; err != nil {
		t.Fatalf("save operator home: %v", err)
	}
	foreign := createTestUser(t, db, 5, "foreign", 1)
	foreign.HomeApplicationId = testAppIdOrder
	if err := db.Save(&foreign).Error; err != nil {
		t.Fatalf("save foreign home: %v", err)
	}
	same := createTestUser(t, db, 6, "same", 1)
	same.HomeApplicationId = testAppIdIam
	if err := db.Save(&same).Error; err != nil {
		t.Fatalf("save same home: %v", err)
	}
	svc := &AccountService{db: db}
	ctx := auth.WithAccountId(context.Background(), 1)

	if _, err := svc.GetAccount(ctx, &common_v1.GetRequest{Id: foreign.Id}); err != ErrAccountOutOfScope {
		t.Fatalf("GetAccount(cross home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	if _, err := svc.UpdateAccount(ctx, &iam_v1.UpdateAccountRequest{Id: foreign.Id, Nickname: "x", Version: 1}); err != ErrAccountOutOfScope {
		t.Fatalf("UpdateAccount(cross home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	if _, err := svc.DeleteAccount(ctx, &iam_v1.DeleteAccountRequest{Id: foreign.Id}); err != ErrAccountOutOfScope {
		t.Fatalf("DeleteAccount(cross home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	if _, err := svc.ResetAccountPassword(ctx, &iam_v1.ResetAccountPasswordRequest{Id: foreign.Id, NewPassword: "password123"}); err != ErrAccountOutOfScope {
		t.Fatalf("ResetAccountPassword(cross home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	if _, err := svc.UpdateAccountRoles(ctx, &iam_v1.UpdateAccountRolesRequest{AccountId: foreign.Id, AddedRoleCodeList: []string{"nope"}}); err != ErrAccountOutOfScope {
		t.Fatalf("UpdateAccountRoles(cross home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	if _, err := svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{AccountId: foreign.Id, ApplicationIds: []string{"900"}}); err != ErrAccountOutOfScope {
		t.Fatalf("UpdateAccountApplications(cross home) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	// 同 home 放行
	if _, err := svc.GetAccount(ctx, &common_v1.GetRequest{Id: same.Id}); err != nil {
		t.Fatalf("GetAccount(same home) error = %v", err)
	}
	// admin 旁路
	adminCtx := auth.WithAdmin(auth.WithAccountId(context.Background(), 1))
	if _, err := svc.GetAccount(adminCtx, &common_v1.GetRequest{Id: foreign.Id}); err != nil {
		t.Fatalf("GetAccount(admin bypass) error = %v", err)
	}
}

func TestUpdateAccountApplicationsNonAdminHomeChangeRejected(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	operator := createTestUser(t, db, 1, "op", 1)
	operator.HomeApplicationId = testAppIdIam
	if err := db.Save(&operator).Error; err != nil {
		t.Fatalf("save operator home: %v", err)
	}
	target := createTestUser(t, db, 5, "target", 1)
	target.HomeApplicationId = testAppIdIam
	if err := db.Save(&target).Error; err != nil {
		t.Fatalf("save target home: %v", err)
	}
	svc := &AccountService{db: db}
	ctx := auth.WithAccountId(context.Background(), 1)

	// 非 admin 显式改 home 拒绝（数据边界不可移动）
	if _, err := svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: target.Id, ApplicationIds: []string{}, HomeApplicationId: "901",
	}); err != ErrAccountOutOfScope {
		t.Fatalf("UpdateAccountApplications(home change) error = %v, want %v", err, ErrAccountOutOfScope)
	}
	// 空串保留旧值 + 准入变更放行
	if _, err := svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: target.Id, ApplicationIds: []string{"901"},
	}); err != nil {
		t.Fatalf("UpdateAccountApplications(admitted only) error = %v", err)
	}
	got, err := svc.GetAccount(ctx, &common_v1.GetRequest{Id: target.Id})
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if got.HomeApplicationId != "900" {
		t.Fatalf("HomeApplicationId = %q, want 900 (unchanged)", got.HomeApplicationId)
	}
	if len(got.Applications) != 1 || got.Applications[0].Id != "901" {
		t.Fatalf("Applications = %v, want [901]", got.Applications)
	}
}
