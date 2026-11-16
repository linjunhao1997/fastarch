package service

import (
	"context"
	"testing"
	"time"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	contribmodel "github.com/linjunhao1997/contrib/model"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	menuSvc := NewMenuService(db, nil)
	dictSvc := NewDictService(db)
	authzSvc := NewAuthorizationService(db, nil)
	bare := auth.WithAccountId(context.Background(), 1)

	if _, err := menuSvc.ListMenuTree(bare, &emptypb.Empty{}); err != ErrApplicationContextRequired {
		t.Fatalf("ListMenuTree() error = %v, want %v", err, ErrApplicationContextRequired)
	}
	if _, err := menuSvc.ListMyMenuTree(bare, &emptypb.Empty{}); err != ErrApplicationContextRequired {
		t.Fatalf("ListMyMenuTree() error = %v, want %v", err, ErrApplicationContextRequired)
	}
	if _, err := dictSvc.ListDictTypes(bare, &iam_v1.ListDictTypesRequest{}); err != ErrApplicationContextRequired {
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
	svc := NewMenuService(db, nil)

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
	svc := NewMenuService(db, nil)

	_, err := svc.CreateMenu(ctxWithApp(testAppIdIam), &iam_v1.CreateMenuRequest{
		Name: "越权菜单", PermissionCode: "order.menu.list",
	})
	if err != ErrPermissionCodeSegmentMismatch {
		t.Fatalf("CreateMenu() error = %v, want %v", err, ErrPermissionCodeSegmentMismatch)
	}
	// 本应用服务段的码放行
	if _, err := svc.CreateMenu(ctxWithApp(testAppIdIam), &iam_v1.CreateMenuRequest{
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

	if _, err := svc.CreateDictType(ctxWithApp(testAppIdIam), &iam_v1.CreateDictTypeRequest{Code: "gender", Name: "性别"}); err != nil {
		t.Fatalf("CreateDictType(iam) error = %v", err)
	}
	// 另一应用可建同名 code（应用内唯一）
	if _, err := svc.CreateDictType(ctxWithApp(testAppIdOrder), &iam_v1.CreateDictTypeRequest{Code: "gender", Name: "性别"}); err != nil {
		t.Fatalf("CreateDictType(order) error = %v", err)
	}
	// 同应用重复 code 拒绝
	if _, err := svc.CreateDictType(ctxWithApp(testAppIdIam), &iam_v1.CreateDictTypeRequest{Code: "gender", Name: "性别2"}); err != ErrDictTypeCodeDuplicated {
		t.Fatalf("CreateDictType(iam dup) error = %v, want %v", err, ErrDictTypeCodeDuplicated)
	}
	// 列表只见本应用
	resp, err := svc.ListDictTypes(ctxWithApp(testAppIdIam), &iam_v1.ListDictTypesRequest{CurrentPage: 1, PageSize: 10})
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
	created, err := svc.CreateDictType(ctxWithApp(testAppIdOrder), &iam_v1.CreateDictTypeRequest{Code: "gender", Name: "性别"})
	if err != nil {
		t.Fatalf("CreateDictType(order) error = %v", err)
	}
	// 在 iam 应用上下文里给 order 的字典类型建条目 → 拒绝（跨应用）
	_, err = svc.CreateDictItem(ctxWithApp(testAppIdIam), &iam_v1.CreateDictItemRequest{
		TypeId: cast.ToInt64(created.Id), Label: "男", Value: "male",
	})
	if err == nil {
		t.Fatal("CreateDictItem() cross-app should fail")
	}
	// 本应用上下文放行
	if _, err := svc.CreateDictItem(ctxWithApp(testAppIdOrder), &iam_v1.CreateDictItemRequest{
		TypeId: cast.ToInt64(created.Id), Label: "男", Value: "male",
	}); err != nil {
		t.Fatalf("CreateDictItem() own-app error = %v", err)
	}
}

// ── 权限管理数据按应用隔离（授予仍全局）──────────────────────────────────

func TestListPermissionsScopedByApplication(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
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

// ── SelectApplication 准入与绑定 ───────────────────────────────────────

func TestSelectApplicationAccessAndBinding(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	rdb, mr := setupTestRedis(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	disabled := createTestApp(t, db, 902, "crm", model.StatusDisabled)
	user := createTestUser(t, db, 5, "scoped-user", 1)
	// 用户仅绑定 order 应用
	if err := db.Create(&model.RUserApplication{UserId: user.Id, ApplicationId: testAppIdOrder}).Error; err != nil {
		t.Fatalf("bind user app: %v", err)
	}
	svc := &AuthenticationService{db: db, rdb: rdb}

	userCtx := auth.WithTokenJti(auth.WithAccountId(context.Background(), user.Id), "jti-user")
	// 未准入应用拒绝
	if _, err := svc.SelectApplication(userCtx, &iam_v1.SelectApplicationRequest{ApplicationCode: "iam"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("SelectApplication(iam) error = %v, want PermissionDenied", err)
	}
	// 停用应用拒绝
	if _, err := svc.SelectApplication(userCtx, &iam_v1.SelectApplicationRequest{ApplicationCode: disabled.Code}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("SelectApplication(disabled) error = %v, want FailedPrecondition", err)
	}
	// 已准入应用绑定成功，Redis 写入映射
	if _, err := svc.SelectApplication(userCtx, &iam_v1.SelectApplicationRequest{ApplicationCode: "order"}); err != nil {
		t.Fatalf("SelectApplication(order) error = %v", err)
	}
	got, err := rdb.Get(context.Background(), "auth:app-ctx:jti-user").Int64()
	if err != nil || got != testAppIdOrder {
		t.Fatalf("app context binding = %d (err %v), want %d", got, err, testAppIdOrder)
	}
	if !mr.Exists("auth:app-ctx:jti-user") {
		t.Fatal("binding key missing in redis")
	}
	// admin 旁路：无绑定也可进入任意启用应用
	adminCtx := auth.WithTokenJti(auth.WithAdmin(auth.WithAccountId(context.Background(), 1)), "jti-admin")
	if _, err := svc.SelectApplication(adminCtx, &iam_v1.SelectApplicationRequest{ApplicationCode: "iam"}); err != nil {
		t.Fatalf("SelectApplication(admin) error = %v", err)
	}
	// 无 jti（机器令牌）不可切换
	if _, err := svc.SelectApplication(auth.WithAccountId(context.Background(), user.Id), &iam_v1.SelectApplicationRequest{ApplicationCode: "order"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("SelectApplication(no jti) error = %v, want FailedPrecondition", err)
	}
}

func TestListMyApplicationsScoping(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	createTestApp(t, db, 902, "crm", model.StatusDisabled)
	user := createTestUser(t, db, 5, "scoped-user", 1)
	if err := db.Create(&model.RUserApplication{UserId: user.Id, ApplicationId: testAppIdOrder}).Error; err != nil {
		t.Fatalf("bind user app: %v", err)
	}
	svc := &AuthenticationService{db: db, rdb: rdb}

	// 普通用户：仅准入的启用应用
	resp, err := svc.ListMyApplications(auth.WithAccountId(context.Background(), user.Id), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListMyApplications() error = %v", err)
	}
	if len(resp.Applications) != 1 || resp.Applications[0].Code != "order" {
		t.Fatalf("ListMyApplications() = %v, want [order]", resp.Applications)
	}
	// admin：全部启用应用（crm 停用不出现）
	adminResp, err := svc.ListMyApplications(auth.WithAdmin(auth.WithAccountId(context.Background(), 1)), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListMyApplications(admin) error = %v", err)
	}
	if len(adminResp.Applications) != 2 {
		t.Fatalf("ListMyApplications(admin) = %d apps, want 2 enabled", len(adminResp.Applications))
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
	ctx := auth.WithAccountId(context.Background(), 1)

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
	// 覆盖为单个（差集删除生效）
	_, err = svc.UpdateAccountApplications(ctx, &iam_v1.UpdateAccountApplicationsRequest{
		AccountId: user.Id, ApplicationIds: []string{"901"},
	})
	if err != nil {
		t.Fatalf("UpdateAccountApplications(shrink) error = %v", err)
	}
	got, _ = svc.GetAccount(ctx, &common_v1.GetRequest{Id: user.Id})
	if len(got.Applications) != 1 || got.Applications[0].Id != "901" {
		t.Fatalf("Applications = %v, want [901]", got.Applications)
	}
	// 清空
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
