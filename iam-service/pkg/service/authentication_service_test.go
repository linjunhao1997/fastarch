package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
	"iam-service/pkg/cipher"
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
	// 011 模型：username/nickname/email/status；无应用归属列表（登录锁定应用，ADR 0007）
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
// 无 metadata 时缺省目标应用为 iam（ADR 0007）。
func TestPasswordAuthEnabledAccountAllowed(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	user := createTestUserWithPassword(t, db, 1, "enabled", "password123", constant.AccountStatusEnabled)
	// 非 admin：所属应用隐式准入，登录缺省 iam 应用需 home 命中
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
	// 缺省 iam 应用已锁定进 claim
	if token.ApplicationId != testAppIdIam {
		t.Fatalf("issued token appId = %d, want %d (iam)", token.ApplicationId, testAppIdIam)
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

// TestPasswordAuthIgnoresCallerApplicationCode 调用方自报 application_code 不参与目标应用选择：
// 无人机 AK/SK 时一律锁定 iam；机机代登录的目标应用由 AK/SK 服务账号推导（另测）。
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
	if token.ApplicationId != testAppIdIam {
		t.Fatalf("issued token appId = %d, want %d (caller code must be ignored)", token.ApplicationId, testAppIdIam)
	}

	// 未准入 iam 的用户即使自报已准入应用的 code 也无法登录
	createTestUserWithPassword(t, db, 2, "no-home", "password123", constant.AccountStatusEnabled)
	_, err = service.PasswordAuth(
		metadata.NewIncomingContext(baseCtx, metadata.Pairs("application_code", "order")),
		&iam_v1.PasswordAuthRequest{Identifier: "no-home", Password: "password123"},
	)
	if err == nil {
		t.Fatal("login with spoofed code to un-admitted app should fail")
	}
}

// TestPasswordAuthServiceAccountDerivesApplication 机机代登录：目标应用由 AK/SK
// 服务账号所属应用推导；用户仍需属于该应用（准入校验不因机机账号而旁路）。
func TestPasswordAuthServiceAccountDerivesApplication(t *testing.T) {
	db := setupProfileTestDB(t)
	rdb, _ := setupTestRedis(t)
	if err := db.AutoMigrate(&model.AccessCredential{}); err != nil {
		t.Fatalf("migrate access credential: %v", err)
	}
	createTestApp(t, db, testAppIdIam, "iam", model.StatusEnabled)
	createTestApp(t, db, testAppIdOrder, "order", model.StatusEnabled)

	human := createTestUserWithPassword(t, db, 20, "order-user", "password123", constant.AccountStatusEnabled)
	human.HomeApplicationId = testAppIdOrder
	if err := db.Save(&human).Error; err != nil {
		t.Fatalf("set human home: %v", err)
	}
	serviceAccount := createTestUser(t, db, 21, "svc-order", 1)
	serviceAccount.HomeApplicationId = testAppIdOrder
	if err := db.Save(&serviceAccount).Error; err != nil {
		t.Fatalf("set service account home: %v", err)
	}

	aes, err := cipher.NewAESGCMFromEncodedKey("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("new aes: %v", err)
	}
	secretCipher, err := aes.Encrypt([]byte("svc-secret"))
	if err != nil {
		t.Fatalf("encrypt sk: %v", err)
	}
	credential := model.AccessCredential{
		Id:              1,
		AccessKey:       "FAKSVCORDER00000000000000001",
		SecretKeyCipher: secretCipher,
		UserId:          21,
		Status:          constant.AccessCredentialStatusEnabled,
		Version:         1,
	}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatalf("seed credential: %v", err)
	}

	store := auth.NewSignatureStore(db, rdb, aes, 300)
	service := NewAuthenticationServiceWithSignature(db, rdb, store)
	baseCtx := grpc.NewContextWithServerTransportStream(context.Background(), stubServerTransportStream{})

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce-service-app-1"
	stringToSign := strings.Join([]string{
		"POST", "/authentication/password-auth", "", credential.AccessKey, timestamp, nonce, auth.EmptyBodyDigestHex,
	}, "\n")
	mac := hmac.New(sha256.New, []byte("svc-secret"))
	mac.Write([]byte(stringToSign))
	md := metadata.Pairs(
		auth.MetadataAccessKey, credential.AccessKey,
		auth.MetadataTimestamp, timestamp,
		auth.MetadataNonce, nonce,
		auth.MetadataSignature, hex.EncodeToString(mac.Sum(nil)),
		auth.MetadataOriginalMethod, "POST",
		auth.MetadataOriginalURI, "/authentication/password-auth",
		auth.MetadataBodyDigest, auth.EmptyBodyDigestHex,
	)
	serviceCtx := metadata.NewIncomingContext(baseCtx, md)

	resp, err := service.PasswordAuth(serviceCtx, &iam_v1.PasswordAuthRequest{Identifier: "order-user", Password: "password123"})
	if err != nil {
		t.Fatalf("PasswordAuth via service account error = %v", err)
	}
	token, err := auth.ParseClaims(resp.GetAccessToken())
	if err != nil {
		t.Fatalf("parse issued token: %v", err)
	}
	if token.ApplicationId != testAppIdOrder {
		t.Fatalf("issued token appId = %d, want %d (service account home)", token.ApplicationId, testAppIdOrder)
	}

	// 用户不属于服务账号所属应用时仍应被准入校验拒绝（换新 nonce 避免重放误判）
	createTestUserWithPassword(t, db, 22, "iam-user-2", "password123", constant.AccountStatusEnabled)
	nonce2 := "nonce-service-app-2"
	stringToSign2 := strings.Join([]string{
		"POST", "/authentication/password-auth", "", credential.AccessKey, timestamp, nonce2, auth.EmptyBodyDigestHex,
	}, "\n")
	mac2 := hmac.New(sha256.New, []byte("svc-secret"))
	mac2.Write([]byte(stringToSign2))
	md2 := metadata.Pairs(
		auth.MetadataAccessKey, credential.AccessKey,
		auth.MetadataTimestamp, timestamp,
		auth.MetadataNonce, nonce2,
		auth.MetadataSignature, hex.EncodeToString(mac2.Sum(nil)),
		auth.MetadataOriginalMethod, "POST",
		auth.MetadataOriginalURI, "/authentication/password-auth",
		auth.MetadataBodyDigest, auth.EmptyBodyDigestHex,
	)
	serviceCtx2 := metadata.NewIncomingContext(baseCtx, md2)
	_, err = service.PasswordAuth(serviceCtx2, &iam_v1.PasswordAuthRequest{Identifier: "iam-user-2", Password: "password123"})
	if err == nil {
		t.Fatal("user outside service application must be rejected")
	}
}
