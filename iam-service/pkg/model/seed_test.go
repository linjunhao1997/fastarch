package model

import (
	"context"
	"slices"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"iam-service/pkg/constant"
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

// 幂等性：同库重复执行 Seed 零新增行。
func TestSeedIdempotent(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #1 error = %v", err)
	}
	snapshots := []struct {
		label string
		model interface{}
	}{{"resource", &Resource{}}, {"permission", &Permission{}}, {"role", &Role{}}, {"role_permission", struct{}{}}}
	countResource := func() (int64, int64, int64, int64) {
		var resources, permissions, roles, grants int64
		if err := db.Model(&Resource{}).Count(&resources).Error; err != nil {
			t.Fatalf("count resource: %v", err)
		}
		if err := db.Model(&Permission{}).Count(&permissions).Error; err != nil {
			t.Fatalf("count permission: %v", err)
		}
		if err := db.Model(&Role{}).Count(&roles).Error; err != nil {
			t.Fatalf("count role: %v", err)
		}
		if err := db.Raw("SELECT COUNT(1) FROM t_r_role_permission").Scan(&grants).Error; err != nil {
			t.Fatalf("count grants: %v", err)
		}
		return resources, permissions, roles, grants
	}
	r1, p1, ro1, g1 := countResource()

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	r2, p2, ro2, g2 := countResource()
	if r1 != r2 || p1 != p2 || ro1 != ro2 || g1 != g2 {
		t.Fatalf("counts changed after re-seed: resource %d->%d, permission %d->%d, role %d->%d, grant %d->%d",
			r1, r2, p1, p2, ro1, ro2, g1, g2)
	}
	_ = snapshots
}

// TestMigrateLegacyAdminRole 已随运行时 Go 迁移移除：遗留 admin 角色改名/合并
// 由 pkg/migration 000004_seed 在真实 MySQL 上承担。此处仅保留遗留角色共存的幂等验证。
func TestSeedMigratesLegacyAdminRole(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()

	legacyRole := Role{Id: 1, Name: "管理员", Code: RoleAdminLegacy, ApplicationId: 1}
	if err := db.Create(&legacyRole).Error; err != nil {
		t.Fatalf("create legacy role: %v", err)
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

	// 遗留角色行原地保留（改名/合并由 pkg/migration 000004 承担，夹具不做迁移），
	// 新建管理员角色与遗留角色并存，遗留角色的账号绑定不受影响
	var legacy Role
	if err := db.Where(RoleColumnId, legacyRole.Id).First(&legacy).Error; err != nil {
		t.Fatalf("get legacy role: %v", err)
	}
	if legacy.Code != RoleAdminLegacy {
		t.Fatalf("legacy role code = %q, want unchanged %q", legacy.Code, RoleAdminLegacy)
	}
	var count int64
	if err := db.Raw("SELECT COUNT(1) FROM t_r_account_role WHERE account_id = ? AND role_id = ?", account.Id, legacyRole.Id).Scan(&count).Error; err != nil {
		t.Fatalf("count account binding: %v", err)
	}
	if count != 1 {
		t.Fatalf("account binding to legacy role = %d, want 1", count)
	}

	// 幂等：再次 Seed 无变化
	var rolesBefore, permissionsBefore int64
	if err := db.Model(&Role{}).Count(&rolesBefore).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if err := db.Model(&Permission{}).Count(&permissionsBefore).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	var rolesAfter, permissionsAfter int64
	if err := db.Model(&Role{}).Count(&rolesAfter).Error; err != nil {
		t.Fatalf("count roles after re-seed: %v", err)
	}
	if err := db.Model(&Permission{}).Count(&permissionsAfter).Error; err != nil {
		t.Fatalf("count permissions after re-seed: %v", err)
	}
	if rolesBefore != rolesAfter || permissionsBefore != permissionsAfter {
		t.Fatalf("counts changed after re-seed: role %d->%d, permission %d->%d",
			rolesBefore, rolesAfter, permissionsBefore, permissionsAfter)
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

// 权限码词汇表：全部 PERMISSION_CODES 建权限行且全量授予管理员角色（011 裁决）。
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

// 组件资源种子：COMPONENT_SEEDS 全部存在（固定 id）、kind 合法、绑定码合法（与迁移 SQL 镜像）。
func TestSeedComponents(t *testing.T) {
	db := setupSeedTestDB(t)
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}

	app := Application{}
	if err := db.Where(ApplicationColumnCode, constant.ApplicationCode).First(&app).Error; err != nil {
		t.Fatalf("get builtin application: %v", err)
	}

	for _, seed := range COMPONENT_SEEDS {
		var resource Resource
		if err := db.Where(ResourceColumnId, seed.Id).First(&resource).Error; err != nil {
			t.Fatalf("component %s (id=%d) missing: %v", seed.Key, seed.Id, err)
		}
		if resource.Key != seed.Key || resource.Kind != seed.Kind {
			t.Fatalf("component id=%d key/kind = %s/%s, want %s/%s", seed.Id, resource.Key, resource.Kind, seed.Key, seed.Kind)
		}
		if resource.ApplicationId != app.Id {
			t.Fatalf("component id=%d application_id = %d, want %d", seed.Id, resource.ApplicationId, app.Id)
		}
		if !IsValidComponentKind(resource.Kind) {
			t.Fatalf("component id=%d invalid kind %q", seed.Id, resource.Kind)
		}
		for _, code := range resource.PermissionCodes {
			if !IsValidPermissionCode(code) {
				t.Fatalf("component id=%d has invalid permission code %q", seed.Id, code)
			}
			if !slices.Contains(PERMISSION_CODES, code) {
				t.Fatalf("component id=%d binds code %q outside vocabulary", seed.Id, code)
			}
		}
	}

	// 幂等：重复 Seed 资源零新增
	var before, after int64
	if err := db.Model(Resource{}).Count(&before).Error; err != nil {
		t.Fatalf("count resources: %v", err)
	}
	if err := Seed(context.Background(), db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	if err := db.Model(Resource{}).Count(&after).Error; err != nil {
		t.Fatalf("count resources after re-seed: %v", err)
	}
	if before != after {
		t.Fatalf("resource count changed after re-seed: %d -> %d", before, after)
	}
}

// 账号-应用关系：绑定启用角色的账号自动获得应用可见性。
func TestSeedAccountApplications(t *testing.T) {
	db := setupSeedTestDB(t)
	ctx := context.Background()
	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}

	app := Application{}
	if err := db.Where(ApplicationColumnCode, constant.ApplicationCode).First(&app).Error; err != nil {
		t.Fatalf("get builtin application: %v", err)
	}
	adminRole := Role{}
	if err := db.Where(RoleColumnCode, RoleAdmin).First(&adminRole).Error; err != nil {
		t.Fatalf("get admin role: %v", err)
	}
	account := Account{Id: 200, Email: "bound@example.com"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := db.Exec("INSERT INTO t_r_account_role (account_id, role_id) VALUES (?, ?)", account.Id, adminRole.Id).Error; err != nil {
		t.Fatalf("bind role: %v", err)
	}

	if err := Seed(ctx, db); err != nil {
		t.Fatalf("Seed() #2 error = %v", err)
	}
	var count int64
	if err := db.Model(&RAccountApplication{}).
		Where(RAccountApplicationColumnAccountId, account.Id).
		Where(RAccountApplicationColumnApplicationId, app.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count account application: %v", err)
	}
	if count != 1 {
		t.Fatalf("account application rows = %d, want 1", count)
	}
}
