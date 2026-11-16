package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	contribmodel "github.com/linjunhao1997/contrib/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func setupSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateModels(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

// 幂等性：同库重复执行 Seed 零新增行。
func TestSeedIdempotent(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #1 error = %v", err)
	}
	count := func() (int64, int64, int64, int64, int64) {
		var users, roles, permissions, grants, menus int64
		if err := db.Model(&User{}).Count(&users).Error; err != nil {
			t.Fatalf("count user: %v", err)
		}
		if err := db.Model(&Role{}).Count(&roles).Error; err != nil {
			t.Fatalf("count role: %v", err)
		}
		if err := db.Model(&Permission{}).Count(&permissions).Error; err != nil {
			t.Fatalf("count permission: %v", err)
		}
		if err := db.Raw("SELECT COUNT(1) FROM t_r_role_permission").Scan(&grants).Error; err != nil {
			t.Fatalf("count grants: %v", err)
		}
		if err := db.Model(&contribmodel.Menu{}).Count(&menus).Error; err != nil {
			t.Fatalf("count menu: %v", err)
		}
		return users, roles, permissions, grants, menus
	}
	u1, r1, p1, g1, m1 := count()

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	u2, r2, p2, g2, m2 := count()
	if u1 != u2 || r1 != r2 || p1 != p2 || g1 != g2 || m1 != m2 {
		t.Fatalf("counts changed after re-seed: user %d->%d, role %d->%d, permission %d->%d, grant %d->%d, menu %d->%d",
			u1, u2, r1, r2, p1, p2, g1, g2, m1, m2)
	}
}

// 种子管理员：admin 用户存在、密码 admin123 可经 bcrypt 校验、绑定 admin 角色。
func TestSeedAdminUser(t *testing.T) {
	db := setupSeedTestDB(t)
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}

	var user User
	if err := db.Where(UserColumnUsername, SeedAdminUsername).First(&user).Error; err != nil {
		t.Fatalf("get admin user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("admin123")); err != nil {
		t.Fatalf("seed admin password mismatch: %v", err)
	}

	var adminRole Role
	if err := db.Where(RoleColumnCode, RoleAdmin).First(&adminRole).Error; err != nil {
		t.Fatalf("get admin role: %v", err)
	}
	var bindings int64
	if err := db.Model(&RUserRole{}).
		Where(RUserRoleColumnUserId, user.Id).
		Where(RUserRoleColumnRoleId, adminRole.Id).
		Count(&bindings).Error; err != nil {
		t.Fatalf("count binding: %v", err)
	}
	if bindings != 1 {
		t.Fatalf("admin user bindings = %d, want 1", bindings)
	}
}

// 管理员角色存在且 code=RoleAdmin。
func TestSeedAdminRole(t *testing.T) {
	db := setupSeedTestDB(t)
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}
	var count int64
	if err := db.Model(Role{}).Where(RoleColumnCode, RoleAdmin).Count(&count).Error; err != nil {
		t.Fatalf("count admin role: %v", err)
	}
	if count != 1 {
		t.Fatalf("admin role count = %d, want 1", count)
	}
}

// 权限码词汇表：全部 PERMISSION_CODES 建权限行且全量授予 admin 角色（011 裁决）。
func TestSeedVocabularyPermissions(t *testing.T) {
	db := setupSeedTestDB(t)
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}

	adminRole := Role{}
	if err := db.Where(RoleColumnCode, RoleAdmin).First(&adminRole).Error; err != nil {
		t.Fatalf("get admin role: %v", err)
	}

	for _, code := range PERMISSION_CODES {
		var permission Permission
		if err := db.Where(PermissionColumnCode, code).First(&permission).Error; err != nil {
			t.Fatalf("vocabulary permission %s missing: %v", code, err)
		}
		var granted int64
		if err := db.Raw(
			"SELECT COUNT(1) FROM t_r_role_permission WHERE role_id = ? AND permission_id = ?",
			adminRole.Id, permission.Id,
		).Scan(&granted).Error; err != nil {
			t.Fatalf("count grant for %s: %v", code, err)
		}
		if granted != 1 {
			t.Fatalf("vocabulary permission %s granted %d times to admin, want 1", code, granted)
		}
	}
}

// 菜单树种子：MENU_SEEDS 全部存在（固定 id）、展示码合法且在词汇表内。
// 菜单为纯展示配置（纯码门槛），不再绑定 admin 角色。
func TestSeedMenus(t *testing.T) {
	db := setupSeedTestDB(t)
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}

	for _, seed := range MENU_SEEDS {
		var menu contribmodel.Menu
		if err := db.Where(contribmodel.MenuColumnId, seed.Id).First(&menu).Error; err != nil {
			t.Fatalf("menu %s (id=%d) missing: %v", seed.Name, seed.Id, err)
		}
		if menu.Name != seed.Name || menu.Path != seed.Path {
			t.Fatalf("menu id=%d name/path = %s/%s, want %s/%s", seed.Id, menu.Name, menu.Path, seed.Name, seed.Path)
		}
		if len(seed.PermissionCode) > 0 && !IsValidPermissionCode(menu.PermissionCode) {
			t.Fatalf("menu id=%d invalid permission code %q", seed.Id, menu.PermissionCode)
		}
	}

	// 幂等：重复 Seed 菜单零新增
	var before, after int64
	if err := db.Model(contribmodel.Menu{}).Count(&before).Error; err != nil {
		t.Fatalf("count menus: %v", err)
	}
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	if err := db.Model(contribmodel.Menu{}).Count(&after).Error; err != nil {
		t.Fatalf("count menus after re-seed: %v", err)
	}
	if before != after {
		t.Fatalf("menu count changed after re-seed: %d -> %d", before, after)
	}
}
