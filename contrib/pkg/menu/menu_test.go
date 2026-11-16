package menu

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/linjunhao1997/contrib/model"
	common_menu_v1 "github.com/linjunhao1997/idl/pb/common/menu"
	"gorm.io/gorm"
)

// 角色-菜单绑定的应用隔离回归测试：UpdateRoleMenus / ListRoleMenus
// 必须限定当前应用上下文——跨应用菜单按不存在处理，差集计算不得
// 误删其他应用的绑定（关联表无应用列，归属经菜单行过滤）。

type menuTestCtxKey struct{}

func setupMenuTestDB(t *testing.T) *gorm.DB {
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
	for _, m := range []interface{}{&model.Menu{}, &model.RRoleMenu{}} {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("migrate sqlite: %v", err)
		}
	}
	return db
}

func newTestMenuService(t *testing.T, db *gorm.DB) *MenuService {
	t.Helper()
	svc, err := NewMenuService(Config{
		DB: db,
		ApplicationIdFromContext: func(ctx context.Context) (int64, error) {
			appId, _ := ctx.Value(menuTestCtxKey{}).(int64)
			if appId == 0 {
				return 0, errors.New("application context required")
			}
			return appId, nil
		},
		AccountIdFromContext: func(ctx context.Context) int64 { return 1 },
		IsAdminFromContext:   func(ctx context.Context) bool { return true },
		RoleIdsForAccount: func(ctx context.Context, accountId int64) ([]int64, error) {
			return []int64{1}, nil
		},
		RoleExists: func(ctx context.Context, roleId int64) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatalf("build menu service: %v", err)
	}
	return svc
}

func menuCtx(appId int64) context.Context {
	return context.WithValue(context.Background(), menuTestCtxKey{}, appId)
}

func createTestMenu(t *testing.T, db *gorm.DB, id, appId int64) {
	t.Helper()
	now := time.Now()
	menu := model.Menu{
		Id:            id,
		ApplicationId: appId,
		Name:          "菜单",
		Status:        model.StatusEnabled,
		Version:       1,
		CreateTime:    now,
		UpdateTime:    now,
	}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatalf("create menu: %v", err)
	}
}

func bindRoleMenu(t *testing.T, db *gorm.DB, roleId, menuId int64) {
	t.Helper()
	if err := db.Create(&model.RRoleMenu{RoleId: roleId, MenuId: menuId}).Error; err != nil {
		t.Fatalf("bind role menu: %v", err)
	}
}

func boundMenuIds(t *testing.T, db *gorm.DB, roleId int64) []int64 {
	t.Helper()
	var ids []int64
	err := db.Model(model.RRoleMenu{}).
		Where(model.RRoleMenuColumnRoleId, roleId).
		Pluck(model.RRoleMenuColumnMenuId, &ids).Error
	if err != nil {
		t.Fatalf("pluck bound menu ids: %v", err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func equalIds(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 目标集合混入其他应用菜单 → 整体拒绝（按不存在处理），不写入任何绑定。
func TestUpdateRoleMenusRejectsCrossAppMenus(t *testing.T) {
	db := setupMenuTestDB(t)
	svc := newTestMenuService(t, db)
	createTestMenu(t, db, 10, 1)
	createTestMenu(t, db, 20, 2)

	_, err := svc.UpdateRoleMenus(menuCtx(1), &common_menu_v1.UpdateRoleMenusRequest{
		RoleId:  1,
		MenuIds: []string{"10", "20"},
	})
	if !errors.Is(err, ErrMenuNotFound) {
		t.Fatalf("UpdateRoleMenus() error = %v, want ErrMenuNotFound", err)
	}
	if ids := boundMenuIds(t, db, 1); len(ids) != 0 {
		t.Fatalf("bindings written on rejected request: %v", ids)
	}
}

// 核心回归：当前应用内全量覆盖时，其他应用的既有绑定不得被差集误删。
func TestUpdateRoleMenusPreservesOtherAppBindings(t *testing.T) {
	db := setupMenuTestDB(t)
	svc := newTestMenuService(t, db)
	createTestMenu(t, db, 10, 1)
	createTestMenu(t, db, 20, 2)
	bindRoleMenu(t, db, 1, 20) // 其他应用的既有绑定

	_, err := svc.UpdateRoleMenus(menuCtx(1), &common_menu_v1.UpdateRoleMenusRequest{
		RoleId:  1,
		MenuIds: []string{"10"},
	})
	if err != nil {
		t.Fatalf("UpdateRoleMenus() error = %v", err)
	}
	if ids := boundMenuIds(t, db, 1); !equalIds(ids, []int64{10, 20}) {
		t.Fatalf("bindings = %v, want [10 20]（应用 2 的绑定须保留）", ids)
	}
}

// 应用内替换语义不变：旧绑定删除、新绑定写入，跨应用绑定仍保留。
func TestUpdateRoleMenusReplacesWithinApp(t *testing.T) {
	db := setupMenuTestDB(t)
	svc := newTestMenuService(t, db)
	createTestMenu(t, db, 10, 1)
	createTestMenu(t, db, 11, 1)
	createTestMenu(t, db, 20, 2)
	bindRoleMenu(t, db, 1, 10)
	bindRoleMenu(t, db, 1, 20)

	_, err := svc.UpdateRoleMenus(menuCtx(1), &common_menu_v1.UpdateRoleMenusRequest{
		RoleId:  1,
		MenuIds: []string{"11"},
	})
	if err != nil {
		t.Fatalf("UpdateRoleMenus() error = %v", err)
	}
	if ids := boundMenuIds(t, db, 1); !equalIds(ids, []int64{11, 20}) {
		t.Fatalf("bindings = %v, want [11 20]", ids)
	}
}

// 回显只返回当前应用的绑定（编辑角色页按应用展示菜单树）。
func TestListRoleMenusScopedToApplication(t *testing.T) {
	db := setupMenuTestDB(t)
	svc := newTestMenuService(t, db)
	createTestMenu(t, db, 10, 1)
	createTestMenu(t, db, 20, 2)
	bindRoleMenu(t, db, 1, 10)
	bindRoleMenu(t, db, 1, 20)

	resp, err := svc.ListRoleMenus(menuCtx(1), &common_menu_v1.ListRoleMenusRequest{RoleId: 1})
	if err != nil {
		t.Fatalf("ListRoleMenus() error = %v", err)
	}
	if len(resp.MenuIds) != 1 || resp.MenuIds[0] != "10" {
		t.Fatalf("MenuIds = %v, want [10]", resp.MenuIds)
	}
}

// 未选应用 → fail-closed，与其余菜单接口一致。
func TestRoleMenuMethodsRequireApplicationContext(t *testing.T) {
	db := setupMenuTestDB(t)
	svc := newTestMenuService(t, db)
	createTestMenu(t, db, 10, 1)

	if _, err := svc.UpdateRoleMenus(context.Background(), &common_menu_v1.UpdateRoleMenusRequest{
		RoleId:  1,
		MenuIds: []string{"10"},
	}); err == nil {
		t.Fatal("UpdateRoleMenus() without application context: want error, got nil")
	}
	if _, err := svc.ListRoleMenus(context.Background(), &common_menu_v1.ListRoleMenusRequest{RoleId: 1}); err == nil {
		t.Fatal("ListRoleMenus() without application context: want error, got nil")
	}
}
