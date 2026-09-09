package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE t_r_role_permission (
		id INTEGER PRIMARY KEY,
		role_id INTEGER NOT NULL,
		permission_id INTEGER NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create sqlite table: %v", err)
	}
	return db
}

func TestDedupRolePermissions(t *testing.T) {
	db := newMigrationTestDB(t)
	rows := []RRolePermission{
		{Id: 1, RoleId: 10, PermissionId: 20},
		{Id: 5, RoleId: 10, PermissionId: 20},
		{Id: 7, RoleId: 11, PermissionId: 20},
		{Id: 8, RoleId: 11, PermissionId: 20},
		{Id: 3, RoleId: 12, PermissionId: 21},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed duplicates: %v", err)
	}
	if err := DedupRolePermissions(db); err != nil {
		t.Fatalf("dedup: %v", err)
	}
	var count int64
	if err := db.Model(&RRolePermission{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("row count after dedup = %d, want 3", count)
	}
	var ids []int64
	if err := db.Model(&RRolePermission{}).Order("id").Pluck("id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 3 || ids[2] != 7 {
		t.Fatalf("retained ids = %v, want [1 3 7]", ids)
	}
	if err := DedupRolePermissions(db); err != nil {
		t.Fatalf("repeat dedup: %v", err)
	}
}

func TestDropLegacyForeignKeysSQLiteNoop(t *testing.T) {
	db := newMigrationTestDB(t)
	if err := DropLegacyForeignKeys(db); err != nil {
		t.Fatalf("sqlite foreign-key migration must be no-op: %v", err)
	}
}

func TestModelsContainsRolePermissionBinding(t *testing.T) {
	found := false
	for _, target := range Models {
		if rolePermission, ok := target.(RRolePermission); ok && rolePermission.TableName() == "t_r_role_permission" {
			found = true
		}
	}
	if !found {
		t.Fatal("RRolePermission is not registered for migration")
	}
}
