package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAccessCredentialTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(Models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestAccessCredentialAutoMigrate(t *testing.T) {
	db := setupAccessCredentialTestDB(t)
	now := time.Now()
	credential := AccessCredential{
		Id:              1,
		AccessKey:       "FAKtest000000000000000001",
		SecretKeyCipher: "cipher-text",
		AccountId:       100,
		Remark:          "ci",
		Status:          "enabled",
		CreateTime:      now,
		UpdateTime:      now,
		CreatedBy:       1,
		UpdatedBy:       1,
		Version:         1,
	}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	var got AccessCredential
	if err := db.Select(AccessCredentialFields).Where(AccessCredentialColumnId, credential.Id).First(&got).Error; err != nil {
		t.Fatalf("get credential: %v", err)
	}
	if got.AccessKey != credential.AccessKey || got.Status != "enabled" {
		t.Fatalf("unexpected row: %+v", got)
	}
}

func TestAccessCredentialUniqueAccessKey(t *testing.T) {
	db := setupAccessCredentialTestDB(t)
	now := time.Now()
	first := AccessCredential{
		Id: 1, AccessKey: "FAKdup", SecretKeyCipher: "c", AccountId: 100,
		Status: "enabled", CreateTime: now, UpdateTime: now, Version: 1,
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first: %v", err)
	}
	duplicate := AccessCredential{
		Id: 2, AccessKey: "FAKdup", SecretKeyCipher: "c", AccountId: 101,
		Status: "enabled", CreateTime: now, UpdateTime: now, Version: 1,
	}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate access_key must violate unique index")
	}
}
