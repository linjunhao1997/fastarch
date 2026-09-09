package auth

import (
	"context"
	"encoding/json"
	"errors"
	"iam-service/pkg/constant"
	"log/slog"
	"net/http"
	"strings"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	cookieHeaderKey = "cookie"
	bearerPrefix    = "Bearer "
)

var notAuthMethod = map[string]struct{}{
	iam_v1.AuthenticationService_PasswordAuth_FullMethodName: {},
}

// authOnlyMethods 仅需认证（self-scoped）、不需要权限判定的方法：
// GetMyPermissions 是权限数据来源，若其自身需要权限，零权限账号将死锁（research.md D6）。
var authOnlyMethods = map[string]struct{}{
	iam_v1.AuthorizationService_GetMyPermissions_FullMethodName: {},
	iam_v1.AuthorizationService_CheckPermission_FullMethodName:  {},
	// specs/008-account-app-context：自描述接口，仅需登录态（无需具体资源权限）
	iam_v1.AuthenticationService_GetMyProfile_FullMethodName: {},
	iam_v1.AuthorizationService_ListMyMenuTree_FullMethodName: {},
}

const contextIsAdmin = "auth.is_admin"

// Interceptor 构造业务鉴权拦截器。load 为权限快照读穿回源函数（缓存未命中时
// 从 DB 重建快照，research.md D6-②），由 server 组装时注入。
//
// 双执行域判定（specs/005-typed-permission-model research.md D3）：
//   - metadata 携带网关标记（gwlib 注入 x-gateway-origin: http）→ 校验 "http:<full_method>" 键；
//   - 无标记（服务间直连 gRPC）→ 校验 "rpc:<full_method>" 键。
func Interceptor(rdb *redis.Client, load PermissionLoader) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if _, ok := notAuthMethod[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
		}

		tokenString := extractTokenFromMetadata(md)

		accountId, err := GetAccountIdFromJwt(tokenString)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "parse token failed: %v", err)
		}

		isAdmin, err := IsAdminFromJwt(tokenString)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "parse token failed: %v", err)
		}

		newCtx := context.WithValue(ctx, contextAccountId, accountId)
		newCtx = context.WithValue(newCtx, contextIsAdmin, isAdmin)

		if isAdmin {
			return handler(newCtx, req)
		}

		// 仅需认证的 self-scoped 方法：不加载快照、不做权限判定
		if _, ok := authOnlyMethods[info.FullMethod]; ok {
			return handler(newCtx, req)
		}

		// 网关来源 → http 域键；服务间直连 → rpc 域键
		requiredKey := constant.SnapshotKeyPrefixRPC + info.FullMethod
		if originValues := md.Get(constant.MetadataGatewayOrigin); len(originValues) > 0 && originValues[0] == constant.GatewayOriginHTTP {
			requiredKey = constant.SnapshotKeyPrefixHTTP + info.FullMethod
		}

		accountIdInt := cast.ToInt64(accountId)
		str, err := rdb.Get(ctx, constant.GetAccountPermissionKey(accountIdInt)).Result()
		var permKeys []string
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				// Redis 其他故障 fail-closed：明确报错、不静默放行、不崩溃级联
				// （research.md D12；底层错误仅入日志不透出客户端）
				slog.Error("get permission snapshot from redis failed",
					"account_id", accountIdInt, "method", info.FullMethod, "error", err)
				return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
			}
			// 缓存未命中：读穿回源，从 DB 重建快照后继续校验（research.md D6-②）
			permKeys, err = LoadPermissionSnapshot(ctx, rdb, accountIdInt, load)
			if err != nil {
				slog.Error("rebuild permission snapshot from db failed",
					"account_id", accountId, "method", info.FullMethod, "error", err)
				return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
			}
		} else if err = json.Unmarshal([]byte(str), &permKeys); err != nil {
			// 快照格式不符（如旧格式残留）视为未命中：走读穿回源重建（research.md D8），不静默放行
			slog.Warn("unmarshal permission snapshot failed, rebuilding from db",
				"account_id", accountId, "method", info.FullMethod, "error", err)
			permKeys, err = LoadPermissionSnapshot(ctx, rdb, accountIdInt, load)
			if err != nil {
				slog.Error("rebuild permission snapshot from db failed",
					"account_id", accountId, "method", info.FullMethod, "error", err)
				return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
			}
		}

		for _, key := range permKeys {
			if key == requiredKey {
				return handler(newCtx, req)
			}
		}

		// fail-closed：无匹配键一律拒绝（FR-010）
		slog.Warn("permission denied",
			"account_id", accountId, "method", info.FullMethod, "required_key", requiredKey)
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}

}

func extractTokenFromMetadata(md metadata.MD) string {
	authValues := md.Get(Authorization)
	if len(authValues) > 0 {
		authHeader := authValues[0]
		if strings.HasPrefix(authHeader, bearerPrefix) {
			return strings.TrimPrefix(authHeader, bearerPrefix)
		} else {
			return strings.TrimSpace(authHeader)
		}
	}

	cookieValues := md.Get(cookieHeaderKey)
	if len(cookieValues) > 0 {
		cookieHeader := cookieValues[0]
		cookies, err := http.ParseCookie(cookieHeader)
		if err != nil {
			return ""
		}
		for _, cookie := range cookies {
			if cookie.Name == Authorization {
				return cookie.Value
			}
		}
	}

	return ""
}
