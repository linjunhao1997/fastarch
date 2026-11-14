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
	first := Resource{Id: 1, ApplicationId: 1, Kind: ComponentKindMenu, Key: "account.list", Name: "账号列表"}
	secondSameApplication := Resource{Id: 2, ApplicationId: 1, Kind: ComponentKindMenu, Key: "account.list", Name: "重复资源"}
	secondApplication := Resource{Id: 3, ApplicationId: 2, Kind: ComponentKindMenu, Key: "account.list", Name: "其他应用资源"}
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

// TestMigrateResourceTreeBackfillsApplication 已随运行时 Go 迁移函数移除：
// 资源归属回填由 pkg/migration 000004_seed ⑥ 在真实 MySQL 上承担。

