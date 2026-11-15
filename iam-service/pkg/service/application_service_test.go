package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"iam-service/pkg/model"
)

func setupApplicationTestDB(t *testing.T) *gorm.DB {
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
	if err := model.AutoMigrateModels(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func createTestApplication(t *testing.T, db *gorm.DB, id int64, name string) model.Application {
	t.Helper()
	now := time.Now()
	app := model.Application{
		Id:         id,
		AppId:      "app-code",
		Name:       name,
		Status:     model.StatusEnabled,
		Version:    1,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	return app
}

func TestUpdateApplication(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		wantErr bool
	}{
		{name: "existing id succeeds", id: 1, wantErr: false},
		{name: "missing id fails", id: 999, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupApplicationTestDB(t)
			app := createTestApplication(t, db, 1, "应用")
			if err := db.Save(&app).Error; err != nil {
				t.Fatalf("prepare application: %v", err)
			}
			time.Sleep(2 * time.Millisecond)
			updateTime := app.UpdateTime

			service := NewApplicationService(db)
			_, err := service.UpdateApplication(context.Background(), &iam_v1.UpdateApplicationRequest{
				Id:   test.id,
				Name: "updated",
			})
			if (err != nil) != test.wantErr {
				t.Fatalf("UpdateApplication() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr {
				if err != gorm.ErrRecordNotFound {
					t.Fatalf("UpdateApplication() error = %v, want %v", err, gorm.ErrRecordNotFound)
				}
				return
			}

			var updated model.Application
			if err := db.First(&updated, app.Id).Error; err != nil {
				t.Fatalf("get updated application: %v", err)
			}
			if !updated.UpdateTime.After(updateTime) {
				t.Fatalf("update_time was not changed: before=%v after=%v", updateTime, updated.UpdateTime)
			}
		})
	}
}

func TestDeleteApplication(t *testing.T) {
	tests := []struct {
		name       string
		id         int64
		wantErr    bool
		wantErrMsg string
	}{
		{name: "existing application is deleted", id: 1, wantErr: false},
		{name: "missing application is rejected", id: 999, wantErr: true, wantErrMsg: "record not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupApplicationTestDB(t)
			if test.id == 1 {
				createTestApplication(t, db, test.id, "应用")
			}

			service := NewApplicationService(db)
			_, err := service.DeleteApplication(context.Background(), &iam_v1.DeleteApplicationRequest{Id: test.id})
			if (err != nil) != test.wantErr {
				t.Fatalf("DeleteApplication() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr {
				if err == nil || err.Error() != test.wantErrMsg {
					t.Fatalf("DeleteApplication() error = %v, want %v", err, test.wantErrMsg)
				}
			}

			var count int64
			if err := db.Model(model.Application{}).Where(model.ApplicationColumnId, test.id).Count(&count).Error; err != nil {
				t.Fatalf("count application: %v", err)
			}
			wantCount := int64(1)
			if test.id == 1 || test.id == 999 {
				wantCount = 0
			}
			if count != wantCount {
				t.Fatalf("application count = %d, want %d", count, wantCount)
			}
		})
	}
}

func TestDeleteApplicationRepeated(t *testing.T) {
	db := setupApplicationTestDB(t)
	app := createTestApplication(t, db, 1, "应用")
	service := NewApplicationService(db)

	if _, err := service.DeleteApplication(context.Background(), &iam_v1.DeleteApplicationRequest{Id: app.Id}); err != nil {
		t.Fatalf("first DeleteApplication() error = %v", err)
	}
	if _, err := service.DeleteApplication(context.Background(), &iam_v1.DeleteApplicationRequest{Id: app.Id}); err != gorm.ErrRecordNotFound {
		t.Fatalf("second DeleteApplication() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

// 011 模型：app_id 服务端生成；secret 一次性返回且服务端仅存 sha256 摘要。
func TestCreateApplicationGeneratesAppIdAndSecret(t *testing.T) {
	db := setupApplicationTestDB(t)
	service := NewApplicationService(db)

	created, err := service.CreateApplication(context.Background(), &iam_v1.CreateApplicationRequest{
		Name: "新应用",
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if created.Application.AppId == "" {
		t.Fatal("AppId should be generated server-side")
	}
	if created.Application.Status != model.StatusEnabled {
		t.Fatalf("Status = %q, want enabled by default", created.Application.Status)
	}
	if len(created.Secret) != 64 {
		t.Fatalf("secret length = %d, want 64 (32 bytes hex)", len(created.Secret))
	}

	// 服务端只存 sha256 hex 摘要，Get 不回传 secret
	detail, err := service.GetApplication(context.Background(), &common_v1.GetRequest{
		Id: cast.ToInt64(created.Application.Id),
	})
	if err != nil {
		t.Fatalf("GetApplication() error = %v", err)
	}
	if detail.AppId != created.Application.AppId {
		t.Fatalf("GetApplication AppId = %q, want %q", detail.AppId, created.Application.AppId)
	}
	var stored model.Application
	if err := db.Where(model.ApplicationColumnId, cast.ToInt64(created.Application.Id)).First(&stored).Error; err != nil {
		t.Fatalf("get stored application: %v", err)
	}
	digest := sha256.Sum256([]byte(created.Secret))
	if stored.SecretHash != hex.EncodeToString(digest[:]) {
		t.Fatalf("SecretHash = %q, want sha256 hex of returned secret", stored.SecretHash)
	}
}
