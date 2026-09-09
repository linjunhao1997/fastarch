package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(Models...); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

// S1-S6 幂等性：同库重复执行 Seed 零新增行（T027）。
func TestSeedIdempotent(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #1 error = %v", err)
	}
	snapshots := []struct {
		label string
		model interface{}
	}{{"resource", &Resource{}}, {"permission", &Permission{}}, {"role", &Role{}}, {"binding", &PermissionResource{}}}
	counts := map[string]int64{}
	for _, s := range snapshots {
		var count int64
		if err := db.Model(s.model).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", s.label, err)
		}
		counts[s.label] = count
	}

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	for _, s := range snapshots {
		var count int64
		if err := db.Model(s.model).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", s.label, err)
		}
		if counts[s.label] != count {
			t.Fatalf("%s count changed after re-seed: %d -> %d", s.label, counts[s.label], count)
		}
	}
}

// S0' 遗留迁移：历史 code="admin" 角色迁移为 RoleAdmin，账号绑定保留，
// 重复的 RoleAdmin 角色被并入；迁移后内置权限全量授予迁移后角色（F4）。
func TestSeedMigratesLegacyAdminRole(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	legacyRole := Role{Id: 1, Name: "管理员", Code: RoleAdminLegacy, ApplicationId: 1}
	if err := db.Create(&legacyRole).Error; err != nil {
		t.Fatalf("create legacy role: %v", err)
	}
	dupRole := Role{Id: 2, Name: "管理员", Code: RoleAdmin, ApplicationId: 1}
	if err := db.Create(&dupRole).Error; err != nil {
		t.Fatalf("create duplicate admin role: %v", err)
	}
	account := Account{Id: 100, Email: "legacy-admin@example.com"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := db.Exec("INSERT INTO t_r_account_role (account_id, role_id) VALUES (?, ?)", account.Id, legacyRole.Id).Error; err != nil {
		t.Fatalf("bind account to legacy role: %v", err)
	}

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}

	// 历史角色编码已迁移，账号绑定保留
	var migrated Role
	if err := db.Where(RoleColumnCode, RoleAdmin).First(&migrated).Error; err != nil {
		t.Fatalf("get migrated admin role: %v", err)
	}
	if migrated.Id != legacyRole.Id {
		t.Fatalf("migrated admin role id = %d, want %d (legacy row renamed in place)", migrated.Id, legacyRole.Id)
	}
	var count int64
	if err := db.Raw("SELECT COUNT(1) FROM t_r_account_role WHERE account_id = ? AND role_id = ?", account.Id, legacyRole.Id).Scan(&count).Error; err != nil {
		t.Fatalf("count account binding: %v", err)
	}
	if count != 1 {
		t.Fatalf("account binding to migrated role = %d, want 1", count)
	}
	// 重复角色已被删除
	var dupCount int64
	if err := db.Model(Role{}).Where(RoleColumnId, dupRole.Id).Count(&dupCount).Error; err != nil {
		t.Fatalf("count duplicate role: %v", err)
	}
	if dupCount != 0 {
		t.Fatal("duplicate RoleAdmin role should be merged and removed")
	}
	// 内置权限已全量授予迁移后角色
	var grantCount int64
	if err := db.Raw("SELECT COUNT(1) FROM t_r_role_permission WHERE role_id = ?", legacyRole.Id).Scan(&grantCount).Error; err != nil {
		t.Fatalf("count grants: %v", err)
	}
	if grantCount != 3 {
		t.Fatalf("builtin permission grants to migrated admin = %d, want 3", grantCount)
	}

	// 幂等：再次 Seed 无变化
	before := map[string]int64{}
	for _, m := range []interface{}{&Role{}, &Permission{}, &PermissionResource{}} {
		var c int64
		if err := db.Model(m).Count(&c).Error; err != nil {
			t.Fatalf("count %T: %v", m, err)
		}
		before[fmt.Sprintf("%T", m)] = c
	}
	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	for _, m := range []interface{}{&Role{}, &Permission{}, &PermissionResource{}} {
		var c int64
		if err := db.Model(m).Count(&c).Error; err != nil {
			t.Fatalf("count %T: %v", m, err)
		}
		if before[fmt.Sprintf("%T", m)] != c {
			t.Fatalf("%T count changed after re-seed: %d -> %d", m, before[fmt.Sprintf("%T", m)], c)
		}
	}
}

// S4 管理员角色存在且 code=RoleAdmin。
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

// S5/S6 三个内置权限存在、kind 正确、全量绑定同类型资源并授予管理员角色。
func TestSeedBuiltinPermissions(t *testing.T) {
	db := setupSeedTestDB(t)
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}

	adminRole := Role{}
	if err := db.Where(RoleColumnCode, RoleAdmin).First(&adminRole).Error; err != nil {
		t.Fatalf("get admin role: %v", err)
	}

	for _, seed := range []struct {
		code string
		kind string
	}{
		{BuiltinPermissionMenuAll, PermissionKindMenu},
		{BuiltinPermissionHTTPAll, PermissionKindHTTPAPI},
		{BuiltinPermissionGRPCAll, PermissionKindGRPC},
	} {
		permission := Permission{}
		if err := db.Where(PermissionColumnCode, seed.code).First(&permission).Error; err != nil {
			t.Fatalf("get builtin permission %s: %v", seed.code, err)
		}
		if permission.Kind != seed.kind {
			t.Fatalf("builtin permission %s kind = %q, want %q", seed.code, permission.Kind, seed.kind)
		}

		var kindResourceCount, bindingCount, roleBindingCount int64
		if err := db.Model(Resource{}).Where(ResourceColumnKind, seed.kind).Count(&kindResourceCount).Error; err != nil {
			t.Fatalf("count resources: %v", err)
		}
		if err := db.Model(PermissionResource{}).
			Where(PermissionResourceColumnPermissionId, permission.Id).
			Count(&bindingCount).Error; err != nil {
			t.Fatalf("count bindings: %v", err)
		}
		if bindingCount != kindResourceCount {
			t.Fatalf("builtin permission %s bindings = %d, want %d (full kind resources)", seed.code, bindingCount, kindResourceCount)
		}
		if err := db.Raw(
			"SELECT COUNT(1) FROM t_r_role_permission WHERE role_id = ? AND permission_id = ?",
			adminRole.Id, permission.Id,
		).Scan(&roleBindingCount).Error; err != nil {
			t.Fatalf("count role binding: %v", err)
		}
		if roleBindingCount != 1 {
			t.Fatalf("builtin permission %s granted to admin %d times, want 1", seed.code, roleBindingCount)
		}
	}
}
