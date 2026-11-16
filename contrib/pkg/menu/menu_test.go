package menu

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/linjunhao1997/contrib/model"
	common_menu_v1 "github.com/linjunhao1997/idl/pb/common/menu"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

// 纯码门槛裁决回归：my-menu-tree 返回当前应用全量启用菜单，服务端不做角色过滤
// （可见性由前端按 permission_code 与用户快照码过滤）；停用菜单排除、跨应用隔离。

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
	if err := db.AutoMigrate(&model.Menu{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
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
	})
	if err != nil {
		t.Fatalf("build menu service: %v", err)
	}
	return svc
}

func menuCtx(appId int64) context.Context {
	return context.WithValue(context.Background(), menuTestCtxKey{}, appId)
}

func createTestMenu(t *testing.T, db *gorm.DB, id, appId int64, status string) {
	t.Helper()
	now := time.Now()
	menu := model.Menu{
		Id:            id,
		ApplicationId: appId,
		Name:          "菜单",
		Status:        status,
		Version:       1,
		CreateTime:    now,
		UpdateTime:    now,
	}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatalf("create menu: %v", err)
	}
}

func collectTreeIds(nodes []*common_menu_v1.MenuTreeNode, out *[]string) {
	for _, n := range nodes {
		*out = append(*out, n.Menu.Id)
		collectTreeIds(n.Children, out)
	}
}

// 返回当前应用全量启用菜单：含普通用户也可见（无角色过滤），停用排除，跨应用隔离。
func TestListMyMenuTreeReturnsAllEnabledMenusOfApp(t *testing.T) {
	db := setupMenuTestDB(t)
	svc := newTestMenuService(t, db)
	createTestMenu(t, db, 10, 1, model.StatusEnabled)
	createTestMenu(t, db, 11, 1, model.StatusEnabled)
	createTestMenu(t, db, 12, 1, model.StatusDisabled)
	createTestMenu(t, db, 20, 2, model.StatusEnabled)

	resp, err := svc.ListMyMenuTree(menuCtx(1), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListMyMenuTree() error = %v", err)
	}
	var ids []string
	collectTreeIds(resp.Roots, &ids)
	if len(ids) != 2 {
		t.Fatalf("menu ids = %v, want 2 enabled menus of app 1 (10,11)", ids)
	}
	seen := map[string]bool{ids[0]: true, ids[1]: true}
	if !seen["10"] || !seen["11"] {
		t.Fatalf("menu ids = %v, want {10,11}（停用 12 与跨应用 20 须排除）", ids)
	}
}

// 未选应用 → fail-closed。
func TestListMyMenuTreeRequiresApplicationContext(t *testing.T) {
	db := setupMenuTestDB(t)
	svc := newTestMenuService(t, db)
	if _, err := svc.ListMyMenuTree(context.Background(), &emptypb.Empty{}); err == nil {
		t.Fatal("ListMyMenuTree() without application context: want error, got nil")
	}
}
