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
	// 个人信息 + 可接入应用列表（统一登录后供控制台数据范围切换使用）
	if resp.Id != "1" || resp.Username != "one" || resp.Email != "" || resp.Nickname != "one" {
		t.Fatalf("GetMyProfile() basic info = %+v", resp)
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
		Password:   "password123",
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
// 统一登录（ADR 0008）：令牌只承载身份与接入集合，不携带应用上下文。
func TestPasswordAuthEnabledAccountAllowed(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	user := createTestUserWithPassword(t, db, 1, "enabled", "password123", constant.AccountStatusEnabled)
	// home 是隐式准入，写入接入集合快照（不再作为登录目标应用）
	user.HomeApplicationId = testAppIdIam
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("set user home: %v", err)
	}

	service := NewAuthenticationService(db, rdb)
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), stubServerTransportStream{})
	resp, err := service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Identifier: "enabled",
		Password:   "password123",
	})
	if err != nil {
		t.Fatalf("enabled account login should pass: %v", err)
	}
	if resp.GetAccessToken() == "" {
		t.Fatal("expected non-empty access token")
	}
	token, err := auth.ParseClaims(resp.GetAccessToken())
	if err != nil {
		t.Fatalf("parse issued token: %v", err)
	}
	// 令牌不再携带应用上下文：admittedAppIds 应包含 home
	if len(token.AdmittedAppIds) == 0 {
		t.Fatal("issued token should carry admitted app ids")
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
		Password:   "wrong-password",
	})
	if err != ErrInvalidAccountPassword {
		t.Fatalf("wrong password error = %v, want %v", err, ErrInvalidAccountPassword)
	}

	// 用户不存在：与密码错误同语义
	_, err = service.PasswordAuth(ctx, &iam_v1.PasswordAuthRequest{
		Identifier: "missing-user",
		Password:   "password123",
	})
	if err != ErrInvalidAccountPassword {
		t.Fatalf("missing user error = %v, want %v", err, ErrInvalidAccountPassword)
	}
}

// TestPasswordAuthIgnoresCallerApplicationCode 调用方自报 application_code 不参与登录：
// 统一登录不接受目标应用参数，令牌只写身份与接入集合（home 隐式准入 + 显式绑定）。
func TestPasswordAuthIgnoresCallerApplicationCode(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)
	user := createTestUserWithPassword(t, db, 1, "iam-user", "password123", constant.AccountStatusEnabled)
	user.HomeApplicationId = testAppIdIam
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("set user home: %v", err)
	}
	// 即使用户也准入 order，自报 code 仍不能改变登录目标
	if err := db.Create(&model.RUserApplication{UserId: user.Id, ApplicationId: testAppIdOrder}).Error; err != nil {
		t.Fatalf("bind user order: %v", err)
	}

	service := NewAuthenticationService(db, rdb)
	baseCtx := grpc.NewContextWithServerTransportStream(context.Background(), stubServerTransportStream{})
	spoofCtx := metadata.NewIncomingContext(baseCtx, metadata.Pairs("application_code", "order"))

	resp, err := service.PasswordAuth(spoofCtx, &iam_v1.PasswordAuthRequest{Identifier: "iam-user", Password: "password123"})
	if err != nil {
		t.Fatalf("PasswordAuth(spoofed code) error = %v", err)
	}
	token, err := auth.ParseClaims(resp.GetAccessToken())
	if err != nil {
		t.Fatalf("parse issued token: %v", err)
	}
	// 自报 code 不改变接入集合：只含 home（iam）与显式绑定的 order
	if len(token.AdmittedAppIds) != 2 {
		t.Fatalf("admittedAppIds = %v, want home + explicit binding", token.AdmittedAppIds)
	}
	seenApps := make(map[int64]bool)
	for _, id := range token.AdmittedAppIds {
		seenApps[id] = true
	}
	if !seenApps[testAppIdIam] || !seenApps[testAppIdOrder] {
		t.Fatalf("admittedAppIds = %v, want home (%d) + binding (%d)", token.AdmittedAppIds, testAppIdIam, testAppIdOrder)
	}

	// home 指向不存在应用的用户：接入集合解析失败，登录拒绝（fail-closed）
	createTestUserWithPassword(t, db, 2, "no-home", "password123", constant.AccountStatusEnabled)
	_, err = service.PasswordAuth(
		metadata.NewIncomingContext(baseCtx, metadata.Pairs("application_code", "order")),
		&iam_v1.PasswordAuthRequest{Identifier: "no-home", Password: "password123"},
	)
	if err == nil {
		t.Fatal("login with spoofed code to un-admitted app should fail")
	}
}

// TestPasswordAuthUnifiedLoginHomeScope 统一登录：只接入业务应用（如 sender/order）的用户
// 无需准入 iam 即可登录；AdmittedAppIds 携带 home + 显式绑定的接入集合，令牌不含应用上下文。
// 控制台选择后由前端决定，登录不再锁定任何控制台。
func TestPasswordAuthUnifiedLoginHomeScope(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)

	user := createTestUserWithPassword(t, db, 20, "order-user", "password123", constant.AccountStatusEnabled)
	user.HomeApplicationId = testAppIdOrder
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("set user home: %v", err)
	}
	// 额外显式接入 iam：接入集合应同时包含 home 与显式绑定。
	if err := db.Create(&model.RUserApplication{UserId: user.Id, ApplicationId: testAppIdIam}).Error; err != nil {
		t.Fatalf("bind user iam: %v", err)
	}

	service := NewAuthenticationService(db, rdb)
	baseCtx := grpc.NewContextWithServerTransportStream(context.Background(), stubServerTransportStream{})
	// 调用方自报 code 与伪造 AK 头都不参与目标应用决策。
	spoofCtx := metadata.NewIncomingContext(baseCtx, metadata.Pairs(
		"application_code", "iam", auth.MetadataAccessKey, "fake-key",
	))
	resp, err := service.PasswordAuth(spoofCtx, &iam_v1.PasswordAuthRequest{Identifier: "order-user", Password: "password123"})
	if err != nil {
		t.Fatalf("unified PasswordAuth error = %v", err)
	}
	token, err := auth.ParseClaims(resp.GetAccessToken())
	if err != nil {
		t.Fatalf("parse issued token: %v", err)
	}
	seen := make(map[int64]bool, len(token.AdmittedAppIds))
	for _, id := range token.AdmittedAppIds {
		seen[id] = true
	}
	if !seen[testAppIdOrder] || !seen[testAppIdIam] {
		t.Fatalf("admittedAppIds = %v, want home + explicit binding", token.AdmittedAppIds)
	}
}
