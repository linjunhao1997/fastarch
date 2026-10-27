package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"iam-service/pkg/cipher"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
)

// newTestSnapshotEnv 构造 miniredis + 注入快照的测试环境。
func newTestSnapshotEnv(t *testing.T, snapshot map[int64][]string) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	for accountId, keys := range snapshot {
		data, err := json.Marshal(keys)
		if err != nil {
			t.Fatalf("marshal snapshot: %v", err)
		}
		if err := rdb.Set(context.Background(), constant.GetAccountPermissionKey(accountId), data, time.Minute).Err(); err != nil {
			t.Fatalf("seed snapshot: %v", err)
		}
	}
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	return mr, rdb
}

func testInterceptorContext(t *testing.T, accountId string, gatewayOrigin bool) (context.Context, *grpc.UnaryServerInfo) {
	t.Helper()
	token, err := GenerateJWT(ClaimsParam{UserId: accountId, ExpirationTime: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	if gatewayOrigin {
		md.Set(constant.MetadataGatewayOrigin, constant.GatewayOriginHTTP)
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}
	return ctx, info
}

func handlerOK(ctx context.Context, req any) (any, error) {
	return "ok", nil
}

// D3：网关来源请求校验 http: 前缀键。
func TestInterceptorGatewayOriginChecksHTTPKey(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, map[int64][]string{
		2: {constant.SnapshotKeyPrefixHTTP + "/iam.v1.AuthorizationService/ListPermissions"},
	})
	ctx, info := testInterceptorContext(t, "2", true)
	_, err := Interceptor(rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		t.Fatal("snapshot should hit cache, loader must not be called")
		return nil, nil
	}, nil)(ctx, nil, info, handlerOK)
	if err != nil {
		t.Fatalf("gateway-origin request with http key denied: %v", err)
	}
}

// D3：无网关标记（服务间直连）校验 rpc: 前缀键；http: 键不放大为服务间调用能力。
func TestInterceptorDirectGRPCChecksRPCKey(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, map[int64][]string{
		2: {constant.SnapshotKeyPrefixHTTP + "/iam.v1.AuthorizationService/ListPermissions"},
	})
	ctx, info := testInterceptorContext(t, "2", false)
	_, err := Interceptor(rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		t.Fatal("snapshot should hit cache, loader must not be called")
		return nil, nil
	}, nil)(ctx, nil, info, handlerOK)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("direct gRPC with only http key: got %v, want PermissionDenied", err)
	}
}

// D3：服务间直连拥有 rpc: 键时放行。
func TestInterceptorDirectGRPCAllowedWithRPCKey(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, map[int64][]string{
		2: {constant.SnapshotKeyPrefixRPC + "/iam.v1.AuthorizationService/ListPermissions"},
	})
	ctx, info := testInterceptorContext(t, "2", false)
	_, err := Interceptor(rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		return nil, nil
	}, nil)(ctx, nil, info, handlerOK)
	if err != nil {
		t.Fatalf("direct gRPC with rpc key denied: %v", err)
	}
}

// D6：登录接口免认证。
func TestInterceptorPasswordAuthBypassesAuth(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, nil)
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: iam_v1.AuthenticationService_PasswordAuth_FullMethodName}
	if _, err := Interceptor(rdb, nil, nil)(ctx, nil, info, handlerOK); err != nil {
		t.Fatalf("PasswordAuth should bypass auth: %v", err)
	}
}

// D6：自查询方法仅需认证（无权限键也可调用），否则零权限账号死锁。
func TestInterceptorAuthOnlyMethodsRequireNoPermission(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, nil)
	for _, method := range []string{
		iam_v1.AuthorizationService_GetMyPermissions_FullMethodName,
		iam_v1.AuthorizationService_CheckPermission_FullMethodName,
	} {
		ctx, _ := testInterceptorContext(t, "2", false)
		info := &grpc.UnaryServerInfo{FullMethod: method}
		if _, err := Interceptor(rdb, func(ctx context.Context, accountId int64) ([]string, error) {
			return []string{}, nil
		}, nil)(ctx, nil, info, handlerOK); err != nil {
			t.Fatalf("%s should only require auth: %v", method, err)
		}
	}
}

// 受保护方法无令牌 → Unauthenticated。
func TestInterceptorMissingTokenRejected(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, nil)
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}
	_, err := Interceptor(rdb, nil, nil)(ctx, nil, info, handlerOK)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token: got %v, want Unauthenticated", err)
	}
}

// 无匹配键 fail-closed（FR-010）。
func TestInterceptorFailClosedOnUnknownMethod(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, map[int64][]string{
		2: {constant.SnapshotKeyPrefixRPC + "/iam.v1.AccountService/ListAccounts"},
	})
	ctx, info := testInterceptorContext(t, "2", false)
	_, err := Interceptor(rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		return nil, nil
	}, nil)(ctx, nil, info, handlerOK)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unknown method: got %v, want PermissionDenied", err)
	}
}

// Redis 故障 fail-closed，不静默放行（宪法 X）。
func TestInterceptorFailClosedOnRedisFailure(t *testing.T) {
	mr, rdb := newTestSnapshotEnv(t, nil)
	mr.Close() // 模拟 Redis 故障
	ctx, info := testInterceptorContext(t, "2", false)
	_, err := Interceptor(rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		t.Fatal("loader should not be called when redis errors")
		return nil, nil
	}, nil)(ctx, nil, info, handlerOK)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("redis failure: got %v, want FailedPrecondition", err)
	}
}

// 缓存未命中走读穿回源（research.md D6-② 回归）。
func TestInterceptorCacheMissRebuildsSnapshot(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, nil)
	ctx, info := testInterceptorContext(t, "2", false)
	_, err := Interceptor(rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		return []string{constant.SnapshotKeyPrefixRPC + "/iam.v1.AuthorizationService/ListPermissions"}, nil
	}, nil)(ctx, nil, info, handlerOK)
	if err != nil {
		t.Fatalf("cache miss with loader grant should pass: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 签名验签分支（specs/009-ak-sk-auth T018）

// 测试用加密密钥（32 字节原始字符串，仅测试环境）。
const testAESKey = "0123456789abcdef0123456789abcdef"

// newTestSignatureStore 构造内存 SQLite + miniredis + AESGCM 的签名验签测试环境。
func newTestSignatureStore(t *testing.T) (*SignatureStore, *redis.Client, *miniredis.Miniredis, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AccessCredential{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	_, rdb := newTestSnapshotEnv(t, nil)
	aes, err := cipher.NewAESGCM([]byte(testAESKey))
	if err != nil {
		t.Fatalf("new aes: %v", err)
	}
	store := NewSignatureStore(db, rdb, aes, 300)
	return store, rdb, nil, "sqlite"
}

// seedTestCredential 在测试库中创建一条凭证，返回明文 SK。
func seedTestCredential(t *testing.T, store *SignatureStore, ak, sk, status string, expire *time.Time, accountId int64) {
	t.Helper()
	aes, err := cipher.NewAESGCM([]byte(testAESKey))
	if err != nil {
		t.Fatalf("new aes: %v", err)
	}
	cipherText, err := aes.Encrypt([]byte(sk))
	if err != nil {
		t.Fatalf("encrypt sk: %v", err)
	}
	credential := model.AccessCredential{
		Id:              accountId + 1000,
		AccessKey:       ak,
		SecretKeyCipher: cipherText,
		AccountId:       accountId,
		Status:          status,
		ExpireTime:      expire,
		Version:         1,
	}
	if err := store.db.Create(&credential).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
}

// signedMetadata 按签名规范构造已签名请求元数据。
func signedMetadata(ak, sk, method, uri, query, body string, timestamp, nonce string) metadata.MD {
	md := metadata.New(map[string]string{
		MetadataAccessKey:     ak,
		MetadataTimestamp:     timestamp,
		MetadataNonce:         nonce,
		MetadataOriginalMethod: method,
		MetadataOriginalURI:   uri,
		MetadataOriginalQuery: query,
		MetadataBodyDigest:    SHA256Hex([]byte(body)),
	})
	sts := BuildStringToSign(method, uri, CanonicalizeQuery(query), ak, timestamp, nonce, SHA256Hex([]byte(body)))
	md.Set(MetadataSignature, ComputeSignatureHex(sts, sk))
	md.Set(constant.MetadataGatewayOrigin, constant.GatewayOriginHTTP)
	return md
}

// signatureReason 解析验签失败错误中的 ErrorInfo.Reason。
func signatureReason(t *testing.T, err error) string {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error, got %v", err)
	}
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok {
			return info.Reason
		}
	}
	return ""
}

// currentTimestamp 返回当前秒级时间戳字符串（测试签名在时间窗口内）。
func currentTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// 签名分支成功路径：验签通过 → 绑定用户权限放行 → context 注入 accountId（FR-015）。
func TestInterceptorSignatureTrackAuthorized(t *testing.T) {
	store, _, _, _ := newTestSignatureStore(t)
	seedTestCredential(t, store, "FAKok0000000000000000001", "sk-plaintext", constant.AccessCredentialStatusEnabled, nil, 7)

	md := signedMetadata("FAKok0000000000000000001", "sk-plaintext", "POST", "/authentication/access-credentials", "", `{"remark":"x"}`, currentTimestamp(), "nonce-ok-1")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}

	var gotAccountId int64
	_, err := Interceptor(store.rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		gotAccountId = accountId
		return []string{constant.SnapshotKeyPrefixHTTP + "/iam.v1.AuthorizationService/ListPermissions"}, nil
	}, store)(ctx, nil, info, handlerOK)
	if err != nil {
		t.Fatalf("signature track request denied: %v", err)
	}
	if gotAccountId != 7 {
		t.Fatalf("context account id = %d, want 7 (bound user identity)", gotAccountId)
	}
}

// FR-015：验签通过但接口未授权给绑定用户 → PERMISSION_DENIED（而非 Unauthenticated）。
func TestInterceptorSignatureTrackPermissionDenied(t *testing.T) {
	store, _, _, _ := newTestSignatureStore(t)
	seedTestCredential(t, store, "FAKok0000000000000000002", "sk-plaintext", constant.AccessCredentialStatusEnabled, nil, 7)

	md := signedMetadata("FAKok0000000000000000002", "sk-plaintext", "POST", "/authentication/access-credentials", "", `{"remark":"x"}`, currentTimestamp(), "nonce-ok-2")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}

	_, err := Interceptor(store.rdb, func(ctx context.Context, accountId int64) ([]string, error) {
		return []string{}, nil // 零权限
	}, store)(ctx, nil, info, handlerOK)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unauthorized interface: got %v, want PermissionDenied", err)
	}
}

// 7 类验签失败语义（规范第 4 节校验顺序）。
func TestInterceptorSignatureTrackFailureSemantics(t *testing.T) {
	tests := []struct {
		name       string
		seedStatus string
		expireIn   time.Duration // <0 = 已过期；0 = 永不过期
		mutate     func(md metadata.MD, sk string)
		wantCode   codes.Code
		wantReason string
	}{
		{
			name:       "missing header",
			seedStatus: constant.AccessCredentialStatusEnabled,
			mutate: func(md metadata.MD, sk string) {
				md.Delete(MetadataSignature)
			},
			wantCode:   codes.Unauthenticated,
			wantReason: "SIGNATURE_MISSING",
		},
		{
			name:       "unknown ak",
			seedStatus: constant.AccessCredentialStatusEnabled,
			mutate: func(md metadata.MD, sk string) {
				md.Set(MetadataAccessKey, "FAKnotexist000000000000")
			},
			wantCode:   codes.Unauthenticated,
			wantReason: "KEY_UNKNOWN",
		},
		{
			name:       "disabled key",
			seedStatus: constant.AccessCredentialStatusDisabled,
			mutate:     func(md metadata.MD, sk string) {},
			wantCode:   codes.Unauthenticated,
			wantReason: "KEY_DISABLED",
		},
		{
			name:       "expired key",
			seedStatus: constant.AccessCredentialStatusEnabled,
			expireIn:   -time.Hour,
			mutate:     func(md metadata.MD, sk string) {},
			wantCode:   codes.Unauthenticated,
			wantReason: "KEY_EXPIRED",
		},
		{
			name:       "timestamp expired",
			seedStatus: constant.AccessCredentialStatusEnabled,
			mutate: func(md metadata.MD, sk string) {
				md.Set(MetadataTimestamp, "1700000000")
			},
			wantCode:   codes.Unauthenticated,
			wantReason: "TIMESTAMP_EXPIRED",
		},
		{
			name:       "nonce replay",
			seedStatus: constant.AccessCredentialStatusEnabled,
			mutate: func(md metadata.MD, sk string) {
				// 同 AK+nonce 已占位（由首个成功请求写入，测试内预置）
			},
			wantCode:   codes.Unauthenticated,
			wantReason: "NONCE_REPLAY",
		},
		{
			name:       "signature mismatch",
			seedStatus: constant.AccessCredentialStatusEnabled,
			mutate: func(md metadata.MD, sk string) {
				sig := md.Get(MetadataSignature)[0]
				md.Set(MetadataSignature, "0"+sig[1:]) // 篡改一位
			},
			wantCode:   codes.Unauthenticated,
			wantReason: "SIGNATURE_MISMATCH",
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, _, _, _ := newTestSignatureStore(t)
			ak := "FAKfail00000000000000000" + string(rune('a'+i))
			var expire *time.Time
			if test.expireIn != 0 {
				t := time.Now().Add(test.expireIn).Round(0)
				expire = &t
				test.expireIn = 0
			}
			seedTestCredential(t, store, ak, "sk-plaintext", test.seedStatus, expire, 7)

			md := signedMetadata(ak, "sk-plaintext", "POST", "/authentication/access-credentials", "", `{"remark":"x"}`, currentTimestamp(), "nonce-fail")
			test.mutate(md, "sk-plaintext")

			// NONCE_REPLAY 用例：先以同 AK+nonce 预占位
			if test.wantReason == "NONCE_REPLAY" {
				if err := store.rdb.Set(context.Background(), "aksk:nonce:"+ak+":nonce-fail", "1", time.Minute).Err(); err != nil {
					t.Fatalf("seed nonce: %v", err)
				}
			}

			ctx := metadata.NewIncomingContext(context.Background(), md)
			info := &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}
			_, err := Interceptor(store.rdb, func(ctx context.Context, accountId int64) ([]string, error) {
				t.Fatal("verify-failed request must not reach handler")
				return nil, nil
			}, store)(ctx, nil, info, handlerOK)

			if status.Code(err) != test.wantCode {
				t.Fatalf("code = %v, want %v", status.Code(err), test.wantCode)
			}
			if reason := signatureReason(t, err); reason != test.wantReason {
				t.Fatalf("reason = %q, want %q", reason, test.wantReason)
			}
		})
	}
}

// NONCE_REPLAY 实际链路：同一请求重放被拒，换新 nonce 重签放行。
func TestInterceptorSignatureTrackReplayRealFlow(t *testing.T) {
	store, _, _, _ := newTestSignatureStore(t)
	seedTestCredential(t, store, "FAKreplay0000000000000001", "sk-plaintext", constant.AccessCredentialStatusEnabled, nil, 7)

	build := func(nonce string) (context.Context, *grpc.UnaryServerInfo) {
		md := signedMetadata("FAKreplay0000000000000001", "sk-plaintext", "GET", "/authentication/access-credentials", "status=enabled", "", currentTimestamp(), nonce)
		return metadata.NewIncomingContext(context.Background(), md), &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}
	}
	loader := func(ctx context.Context, accountId int64) ([]string, error) {
		return []string{constant.SnapshotKeyPrefixHTTP + "/iam.v1.AuthorizationService/ListPermissions"}, nil
	}

	// 首次请求放行
	ctx1, info := build("nonce-first")
	if _, err := Interceptor(store.rdb, loader, store)(ctx1, nil, info, handlerOK); err != nil {
		t.Fatalf("first request should pass: %v", err)
	}
	// 原样重放被拒
	ctx2, info := build("nonce-first")
	_, err := Interceptor(store.rdb, loader, store)(ctx2, nil, info, handlerOK)
	if reason := signatureReason(t, err); reason != "NONCE_REPLAY" {
		t.Fatalf("replayed request reason = %q, want NONCE_REPLAY", reason)
	}
	// 换新 nonce 放行
	ctx3, info := build("nonce-second")
	if _, err := Interceptor(store.rdb, loader, store)(ctx3, nil, info, handlerOK); err != nil {
		t.Fatalf("new nonce request should pass: %v", err)
	}
}

// store 为 nil 时签名请求 fail-closed（Unauthenticated），JWT 轨道不受影响（FR-010a 回归）。
func TestInterceptorSignatureNilStoreFailClosed(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, nil)
	md := metadata.New(map[string]string{
		MetadataAccessKey: "FAKanything",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}
	_, err := Interceptor(rdb, nil, nil)(ctx, nil, info, handlerOK)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("signature request with nil store: got %v, want Unauthenticated", err)
	}
	if !errors.Is(err, err) {
		t.Fatal("sanity")
	}
}
