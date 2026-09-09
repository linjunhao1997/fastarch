package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newResourceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Application{}, &Resource{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func TestResourceApplicationScopedKey(t *testing.T) {
	db := newResourceTestDB(t)
	first := Resource{Id: 1, ApplicationId: 1, Kind: PermissionKindMenu, Key: "account.list", Name: "账号列表"}
	secondSameApplication := Resource{Id: 2, ApplicationId: 1, Kind: PermissionKindMenu, Key: "account.list", Name: "重复资源"}
	secondApplication := Resource{Id: 3, ApplicationId: 2, Kind: PermissionKindMenu, Key: "account.list", Name: "其他应用资源"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first resource: %v", err)
	}
	if err := db.Create(&secondSameApplication).Error; err == nil {
		t.Fatal("duplicate application-scoped resource key was accepted")
	}
	if err := db.Create(&secondApplication).Error; err != nil {
		t.Fatalf("same key in another application was rejected: %v", err)
	}
}

func TestMigrateResourceTreeBackfillsApplication(t *testing.T) {
	db := newResourceTestDB(t)
	application := Application{Id: 10, Code: "iam-service", Name: "IAM"}
	if err := db.Create(&application).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	resource := Resource{Id: 1, Kind: PermissionKindMenu, Key: "legacy.key", Name: "Legacy"}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatalf("create legacy resource: %v", err)
	}
	if err := MigrateResourceTree(db); err != nil {
		t.Fatalf("MigrateResourceTree() error = %v", err)
	}
	var got Resource
	if err := db.Select(ResourceFields).Where(ResourceColumnId, resource.Id).First(&got).Error; err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if got.ApplicationId != application.Id {
		t.Fatalf("resource application id = %d, want %d", got.ApplicationId, application.Id)
	}
	if db.Migrator().HasIndex(&Resource{}, "idx_resource_key") {
		t.Fatal("legacy global key index still exists")
	}
}
