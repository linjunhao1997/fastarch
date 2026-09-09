package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"
)

// setupProfileTestDB 构造 GetMyProfile 测试库（specs/008-account-app-context T016）。
func setupProfileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupAccountTestDB(t)
	return db
}

func createProfileApplication(t *testing.T, db *gorm.DB, id int64, name, statusCode string) model.Application {
	t.Helper()
	now := time.Now()
	app := model.Application{
		Id:         id,
		Code:       cast.ToString(id),
		Name:       name,
		Disabled:   cast.ToInt(statusCode != constant.ApplicationStatusEnabled),
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	return app
}

func bindAccountApplication(t *testing.T, db *gorm.DB, accountId, applicationId int64) {
	t.Helper()
	if err := db.Create(&model.RAccountApplication{AccountId: accountId, ApplicationId: applicationId}).Error; err != nil {
		t.Fatalf("bind account application: %v", err)
	}
}

func TestGetMyProfile(t *testing.T) {
	db := setupProfileTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	createProfileApplication(t, db, 101, "应用A", constant.ApplicationStatusEnabled)
	createProfileApplication(t, db, 102, "应用B停用", constant.ApplicationStatusDisabled)
	bindAccountApplication(t, db, 1, 101)
	bindAccountApplication(t, db, 1, 102)
	service := NewAuthenticationService(db, nil)

	resp, err := service.GetMyProfile(auth.WithAccountId(context.Background(), 1), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyProfile() error = %v", err)
	}
	if resp.Id != "1" || resp.Name != "account" || resp.Email != "one@example.com" {
		t.Fatalf("GetMyProfile() basic info = %+v", resp)
	}
	if len(resp.Applications) != 2 {
		t.Fatalf("applications = %d, want 2（含停用应用）", len(resp.Applications))
	}
	// 含停用应用（前端置灰不可选）
	var hasDisabled bool
	for _, app := range resp.Applications {
		if app.Id == "102" && app.Status == constant.ApplicationStatusDisabled {
			hasDisabled = true
		}
	}
	if !hasDisabled {
		t.Fatalf("disabled application missing from profile: %+v", resp.Applications)
	}
}

func TestGetMyProfileNoApplications(t *testing.T) {
	db := setupProfileTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	service := NewAuthenticationService(db, nil)

	resp, err := service.GetMyProfile(auth.WithAccountId(context.Background(), 1), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyProfile() error = %v", err)
	}
	if len(resp.Applications) != 0 {
		t.Fatalf("applications = %v, want empty", resp.Applications)
	}
}

func TestGetMyProfileWithoutSensitiveFields(t *testing.T) {
	db := setupProfileTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	service := NewAuthenticationService(db, nil)

	resp, err := service.GetMyProfile(auth.WithAccountId(context.Background(), 1), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyProfile() error = %v", err)
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	lower := strings.ToLower(string(raw))
	for _, banned := range []string{"password", "salt", `"ak"`, `"sk"`} {
		if strings.Contains(lower, banned) {
			t.Fatalf("profile response contains sensitive field %q: %s", banned, raw)
		}
	}
}

func TestGetMyProfileUnauthenticated(t *testing.T) {
	db := setupProfileTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	service := NewAuthenticationService(db, nil)

	_, err := service.GetMyProfile(context.Background(), &emptypb.Empty{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetMyProfile() without auth error = %v, want Unauthenticated", err)
	}
}

// TestPasswordAuthDisabledAccountRejected 禁用账号即使密码正确也拒绝登录
//（账号状态表单项配套的服务端裁决，宪法 IV：安全决策在服务端）。
func TestPasswordAuthDisabledAccountRejected(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	account := createTestAccount(t, db, 1, "disabled@example.com", 1)
	account.Status = constant.AccountStatusDisabled
	account.Salt = "salt"
	account.Password = helper.HashPasswordWithSalt("password123", "salt")
	if err := db.Save(&account).Error; err != nil {
		t.Fatalf("save account: %v", err)
	}

	service := NewAuthenticationService(db, rdb)
	ctx := grpc.NewContextWithServerTransportStream(auth.WithAdmin(context.Background()), stubServerTransportStream{})
	_, err := service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Email:    "disabled@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected disabled account login to be rejected")
	}
	// 业务错误码经统一拦截器映射；单测直接断言错误语义
	if !strings.Contains(err.Error(), "账号已禁用") {
		t.Fatalf("expected account disabled error, got %v", err)
	}
}

// stubServerTransportStream 使 PasswordAuth 的 SetHeader 在单测上下文可用。
type stubServerTransportStream struct {
	grpc.ServerTransportStream
}

func (stubServerTransportStream) SetHeader(metadata.MD) error  { return nil }
func (stubServerTransportStream) SendHeader(metadata.MD) error { return nil }
func (stubServerTransportStream) SetTrailer(metadata.MD) error { return nil }

// TestPasswordAuthEnabledAccountAllowed 启用账号正常登录（防误伤）。
func TestPasswordAuthEnabledAccountAllowed(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	account := createTestAccount(t, db, 1, "enabled@example.com", 1)
	account.Status = constant.AccountStatusEnabled
	account.Salt = "salt"
	account.Password = helper.HashPasswordWithSalt("password123", "salt")
	if err := db.Save(&account).Error; err != nil {
		t.Fatalf("save account: %v", err)
	}

	service := NewAuthenticationService(db, rdb)
	ctx := grpc.NewContextWithServerTransportStream(auth.WithAdmin(context.Background()), stubServerTransportStream{})
	resp, err := service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Email:    "enabled@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("enabled account login should pass: %v", err)
	}
	if resp.GetAccessToken() == "" {
		t.Fatal("expected non-empty access token")
	}
}
