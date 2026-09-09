package auth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"iam-service/pkg/constant"

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
	})(ctx, nil, info, handlerOK)
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
	})(ctx, nil, info, handlerOK)
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
	})(ctx, nil, info, handlerOK)
	if err != nil {
		t.Fatalf("direct gRPC with rpc key denied: %v", err)
	}
}

// D6：登录接口免认证。
func TestInterceptorPasswordAuthBypassesAuth(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, nil)
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: iam_v1.AuthenticationService_PasswordAuth_FullMethodName}
	if _, err := Interceptor(rdb, nil)(ctx, nil, info, handlerOK); err != nil {
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
		})(ctx, nil, info, handlerOK); err != nil {
			t.Fatalf("%s should only require auth: %v", method, err)
		}
	}
}

// 受保护方法无令牌 → Unauthenticated。
func TestInterceptorMissingTokenRejected(t *testing.T) {
	_, rdb := newTestSnapshotEnv(t, nil)
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/iam.v1.AuthorizationService/ListPermissions"}
	_, err := Interceptor(rdb, nil)(ctx, nil, info, handlerOK)
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
	})(ctx, nil, info, handlerOK)
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
	})(ctx, nil, info, handlerOK)
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
	})(ctx, nil, info, handlerOK)
	if err != nil {
		t.Fatalf("cache miss with loader grant should pass: %v", err)
	}
}
