package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"
)

// setupProfileTestDB 构造 GetMyProfile 测试库。
func setupProfileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupAccountTestDB(t)
}

// createTestUserWithPassword 创建带 bcrypt 密码摘要的测试用户。
func createTestUserWithPassword(t *testing.T, db *gorm.DB, id int64, username, password, status string) model.User {
	t.Helper()
	user := createTestUser(t, db, id, username, 1)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user.PasswordHash = string(hash)
	user.Status = status
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	return user
}

func TestGetMyProfile(t *testing.T) {
	db := setupProfileTestDB(t)
	createTestUser(t, db, 1, "one", 1)
	service := NewAuthenticationService(db, nil)

	resp, err := service.GetMyProfile(auth.WithAccountId(context.Background(), 1), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetMyProfile() error = %v", err)
	}
	// 011 模型：username/nickname/email/status；无应用归属
	if resp.Id != "1" || resp.Username != "one" || resp.Email != "" || resp.Nickname != "one" {
		t.Fatalf("GetMyProfile() basic info = %+v", resp)
	}
	if len(resp.Applications) != 0 {
		t.Fatalf("applications = %v, want empty", resp.Applications)
	}
}

func TestGetMyProfileWithoutSensitiveFields(t *testing.T) {
	db := setupProfileTestDB(t)
	createTestUser(t, db, 1, "one", 1)
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
	for _, banned := range []string{"password", "password_hash", "salt", `"ak"`, `"sk"`} {
		if strings.Contains(lower, banned) {
			t.Fatalf("profile response contains sensitive field %q: %s", banned, raw)
		}
	}
}

func TestGetMyProfileUnauthenticated(t *testing.T) {
	db := setupProfileTestDB(t)
	createTestUser(t, db, 1, "one", 1)
	service := NewAuthenticationService(db, nil)

	_, err := service.GetMyProfile(context.Background(), &emptypb.Empty{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetMyProfile() without auth error = %v, want Unauthenticated", err)
	}
}

// TestPasswordAuthDisabledAccountRejected 禁用账号即使密码正确也拒绝登录
// （账号状态表单项配套的服务端裁决，宪法 IV：安全决策在服务端）。
func TestPasswordAuthDisabledAccountRejected(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestUserWithPassword(t, db, 1, "disabled", "password123", constant.AccountStatusDisabled)

	service := NewAuthenticationService(db, rdb)
	ctx := grpc.NewContextWithServerTransportStream(auth.WithAdmin(context.Background()), stubServerTransportStream{})
	_, err := service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Identifier: "disabled",
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
	createTestUserWithPassword(t, db, 1, "enabled", "password123", constant.AccountStatusEnabled)

	service := NewAuthenticationService(db, rdb)
	ctx := grpc.NewContextWithServerTransportStream(auth.WithAdmin(context.Background()), stubServerTransportStream{})
	resp, err := service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Identifier: "enabled",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("enabled account login should pass: %v", err)
	}
	if resp.GetAccessToken() == "" {
		t.Fatal("expected non-empty access token")
	}
}

// 密码错误与用户不存在统一语义（不泄露账号存在性），均返回 ErrInvalidAccountPassword。
func TestPasswordAuthInvalidCredentials(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestUserWithPassword(t, db, 1, "enabled", "password123", constant.AccountStatusEnabled)

	service := NewAuthenticationService(db, rdb)
	ctx := grpc.NewContextWithServerTransportStream(auth.WithAdmin(context.Background()), stubServerTransportStream{})

	// 密码错误
	_, err := service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Identifier: "enabled",
		Password: "wrong-password",
	})
	if err != ErrInvalidAccountPassword {
		t.Fatalf("wrong password error = %v, want %v", err, ErrInvalidAccountPassword)
	}

	// 用户不存在：与密码错误同语义
	_, err = service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Identifier: "missing-user",
		Password: "password123",
	})
	if err != ErrInvalidAccountPassword {
		t.Fatalf("missing user error = %v, want %v", err, ErrInvalidAccountPassword)
	}
}
