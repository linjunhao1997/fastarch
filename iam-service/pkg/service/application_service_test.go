package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	contribmodel "github.com/linjunhao1997/contrib/model"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
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

// TestApplicationWriteAdminOnly 验证应用域写操作硬门槛（ADR 0006 补充裁决）：
// 非 admin 即使持有应用管理码也在服务层拒绝（码可经角色绑定自授，码门槛不构成
// 租户边界）；admin 放行；读接口不受硬门槛影响（码门槛在拦截器层）。
func TestApplicationWriteAdminOnly(t *testing.T) {
	db := setupApplicationTestDB(t)
	createTestApplication(t, db, 1, "应用")
	service := NewApplicationService(db)
	ctxNonAdmin := auth.WithAccountId(context.Background(), 7)
	ctxAdmin := auth.WithAdmin(auth.WithAccountId(context.Background(), 99))

	if _, err := service.CreateApplication(ctxNonAdmin, &iam_v1.CreateApplicationRequest{Code: "sneaky", Name: "sneaky"}); !errors.Is(err, ErrApplicationAdminRequired) {
		t.Fatalf("non-admin create error = %v, want %v", err, ErrApplicationAdminRequired)
	}
	if _, err := service.UpdateApplication(ctxNonAdmin, &iam_v1.UpdateApplicationRequest{Id: 1, Name: "hacked"}); !errors.Is(err, ErrApplicationAdminRequired) {
		t.Fatalf("non-admin update error = %v, want %v", err, ErrApplicationAdminRequired)
	}
	if _, err := service.DeleteApplication(ctxNonAdmin, &iam_v1.DeleteApplicationRequest{Id: 1}); !errors.Is(err, ErrApplicationAdminRequired) {
		t.Fatalf("non-admin delete error = %v, want %v", err, ErrApplicationAdminRequired)
	}
	// admin 放行
	if _, err := service.CreateApplication(ctxAdmin, &iam_v1.CreateApplicationRequest{Code: "admin-created", Name: "admin"}); err != nil {
		t.Fatalf("admin create: %v", err)
	}
	// 读接口不受硬门槛影响
	if _, err := service.GetApplication(ctxNonAdmin, &common_v1.GetRequest{Id: 1}); err != nil {
		t.Fatalf("non-admin get: %v", err)
	}
	if _, err := service.ListApplications(ctxNonAdmin, &iam_v1.ListApplicationsRequest{CurrentPage: 1, PageSize: 10}); err != nil {
		t.Fatalf("non-admin list: %v", err)
	}
}

// TestDeleteApplicationReferenced 验证应用删除引用校验（ADR 0006 补充裁决）：
// 角色/权限/菜单/字典归属、用户 home、准入绑定任一引用即禁删；全部解除后可删。
func TestDeleteApplicationReferenced(t *testing.T) {
	db := setupApplicationTestDB(t)
	createTestApplication(t, db, 1, "应用")
	service := NewApplicationService(db)
	ctx := context.Background() // 系统调用旁路 admin 门槛，聚焦引用校验
	req := &iam_v1.DeleteApplicationRequest{Id: 1}

	assertReferenced := func(name string) {
		t.Helper()
		if _, err := service.DeleteApplication(ctx, req); !errors.Is(err, ErrApplicationReferenced) {
			t.Fatalf("%s reference: delete error = %v, want %v", name, err, ErrApplicationReferenced)
		}
	}

	// 角色归属引用
	role := createTestRole(t, db, 11, "ref-role", 1)
	role.ApplicationId = 1
	if err := db.Save(&role).Error; err != nil {
		t.Fatalf("set role app: %v", err)
	}
	assertReferenced("role")
	if err := db.Where(model.RoleColumnId, role.Id).Delete(&model.Role{}).Error; err != nil {
		t.Fatalf("cleanup role: %v", err)
	}

	// 权限归属引用
	permission := createTestPermission(t, db, 12, "iam.ref.perm", 1)
	permission.ApplicationId = 1
	if err := db.Save(&permission).Error; err != nil {
		t.Fatalf("set permission app: %v", err)
	}
	assertReferenced("permission")
	if err := db.Where(model.PermissionColumnId, permission.Id).Delete(&model.Permission{}).Error; err != nil {
		t.Fatalf("cleanup permission: %v", err)
	}

	// 菜单归属引用
	menu := contribmodel.Menu{Id: 13, ApplicationId: 1, Name: "ref-menu", Path: "/ref"}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatalf("create menu: %v", err)
	}
	assertReferenced("menu")
	if err := db.Where("id", menu.Id).Delete(&contribmodel.Menu{}).Error; err != nil {
		t.Fatalf("cleanup menu: %v", err)
	}

	// 字典归属引用
	dictType := contribmodel.DictType{Id: 14, ApplicationId: 1, Code: "ref-dict", Name: "ref"}
	if err := db.Create(&dictType).Error; err != nil {
		t.Fatalf("create dict type: %v", err)
	}
	assertReferenced("dict")
	if err := db.Where("id", dictType.Id).Delete(&contribmodel.DictType{}).Error; err != nil {
		t.Fatalf("cleanup dict type: %v", err)
	}

	// 用户 home 引用
	user := createTestUser(t, db, 15, "ref-user", 1)
	user.HomeApplicationId = 1
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("set user home: %v", err)
	}
	assertReferenced("user home")
	// home 清零后仅剩准入绑定引用
	if err := db.Model(&model.User{}).Where(model.UserColumnId, user.Id).
		Update(model.UserColumnHomeApplicationId, 0).Error; err != nil {
		t.Fatalf("clear user home: %v", err)
	}
	binding := model.RUserApplication{UserId: user.Id, ApplicationId: 1}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("create admission binding: %v", err)
	}
	assertReferenced("admission binding")
	if err := db.Where("user_id", user.Id).Delete(&model.RUserApplication{}).Error; err != nil {
		t.Fatalf("cleanup binding: %v", err)
	}

	// 全部引用解除 → 删除成功
	if _, err := service.DeleteApplication(ctx, req); err != nil {
		t.Fatalf("delete unreferenced application: %v", err)
	}
}
