package service

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
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
	err = db.Migrator().DropTable(&model.Role{}, &model.Permission{}, &model.Application{})
	if err != nil {
		t.Fatalf("clean tables: %v", err)
	}
	err = db.AutoMigrate(&model.Application{}, &model.Role{}, &model.Permission{}, &model.Account{}, &model.RAccountApplication{})
	if err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func createTestApplication(t *testing.T, db *gorm.DB, id int64, name string) model.Application {
	t.Helper()
	now := time.Now()
	app := model.Application{
		Id:         id,
		Code:       "app-code",
		Name:       name,
		Disabled:   0,
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
				Id:          test.id,
				Description: "updated",
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
		withRef    bool
		wantErr    bool
		wantErrMsg string
	}{
		{name: "unreferenced application is deleted", id: 1, wantErr: false},
		{name: "referenced application is rejected", id: 2, withRef: true, wantErr: true, wantErrMsg: "应用已被引用，请先解除引用"},
		{name: "missing application is rejected", id: 999, wantErr: true, wantErrMsg: "record not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupApplicationTestDB(t)
			if test.id == 1 || test.id == 2 {
				createTestApplication(t, db, test.id, "应用")
			}
			if test.withRef {
				role := model.Role{Id: 10, ApplicationId: test.id}
				if err := db.Create(&role).Error; err != nil {
					t.Fatalf("create role reference: %v", err)
				}
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
			if test.id == 1 && !test.wantErr || test.id == 999 {
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

// ---- specs/008-account-app-context：应用删除时账号关联拦截（T009）----

func TestDeleteApplicationBlockedByAccountAssociation(t *testing.T) {
	db := setupApplicationTestDB(t)
	if err := db.AutoMigrate(&model.Account{}, &model.RAccountApplication{}); err != nil {
		t.Fatalf("migrate account tables: %v", err)
	}
	app := createTestApplication(t, db, 201, "被关联应用")
	now := time.Now()
	account := model.Account{
		Id:         1,
		Code:       "acc",
		Name:       "account",
		Email:      "acc@example.com",
		Status:     "enabled",
		Kind:       model.AccountKindIdUser,
		CreateTime: now,
		UpdateTime: now,
		Version:    1,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	association := model.RAccountApplication{AccountId: account.Id, ApplicationId: app.Id}
	if err := db.Create(&association).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	service := NewApplicationService(db)
	_, err := service.DeleteApplication(context.Background(), &iam_v1.DeleteApplicationRequest{Id: app.Id})
	if err != ErrApplicationReferenced {
		t.Fatalf("DeleteApplication() error = %v, want ErrApplicationReferenced", err)
	}

	var count int64
	if err := db.Model(&model.Application{}).Where(model.ApplicationColumnId, app.Id).Count(&count).Error; err != nil {
		t.Fatalf("count applications: %v", err)
	}
	if count != 1 {
		t.Fatalf("application was deleted despite account association")
	}
}
