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
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	cookieHeaderKey = "cookie"
	bearerPrefix    = "Bearer "
	errorDomain     = "iam-service"
	traceIDHeader   = "trace-id"
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
// 从 DB 重建快照，research.md D6-②），store 为 AK/SK 签名验签器
// （specs/009-ak-sk-auth T017，测试 JWT 双轨回归场景可传 nil）。
//
// 双轨身份（FR-010a）：
//   - metadata 含 x-fa-access-key → 签名验签分支（SignatureStore.Authenticate），
//     通过后以绑定用户身份进入统一权限判定（FR-015，签名不授予额外权限、无管理员旁路）；
//   - 无该头 → 既有 JWT 分支完全不变。
//
// 双执行域判定（specs/005-typed-permission-model research.md D3）：
//   - metadata 携带网关标记（gwlib 注入 x-gateway-origin: http）→ 校验 "http:<full_method>" 键；
//   - 无标记（服务间直连 gRPC）→ 校验 "rpc:<full_method>" 键。
func Interceptor(rdb *redis.Client, load PermissionLoader, store *SignatureStore) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if _, ok := notAuthMethod[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
		}

		// 签名验签分支（specs/009-ak-sk-auth）：请求携带 AK 头即走 AK/SK 轨道
		if len(md.Get(MetadataAccessKey)) > 0 {
			return authenticateBySignature(ctx, rdb, load, store, md, info, handler, req)
		}

		// JWT 轨道（既有逻辑，FR-010a 双轨之另一轨）
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

		requiredKey := requiredPermissionKey(md, info.FullMethod)

		if err := authorizeBySnapshot(ctx, rdb, load, cast.ToInt64(accountId), requiredKey, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// authenticateBySignature 签名验签分支：验签通过后以绑定用户身份复用统一权限判定。
func authenticateBySignature(ctx context.Context, rdb *redis.Client, load PermissionLoader, store *SignatureStore, md metadata.MD, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler, req any) (any, error) {
	if store == nil {
		return nil, status.Error(codes.Unauthenticated, "signature authentication is not configured")
	}

	accountId, err := store.Authenticate(ctx, md)
	if err != nil {
		var sigErr *SignatureError
		if errors.As(err, &sigErr) {
			// 验签失败结构化日志（FR-014a：AK、失败 reason、来源、traceId），监控告警消费
			slog.Warn("signature verify failed",
				"access_key", firstMetadataValue(md, MetadataAccessKey),
				"reason", sigErr.Code,
				"origin", signatureOrigin(md),
				"trace_id", traceIDFromMetadata(md),
				"method", info.FullMethod)
			return nil, signatureStatusError(sigErr)
		}
		// 内部故障（Redis/DB/解密异常）fail-closed，不透出底层错误
		slog.Error("signature authenticate internal failure",
			"access_key", firstMetadataValue(md, MetadataAccessKey),
			"trace_id", traceIDFromMetadata(md),
			"method", info.FullMethod, "error", err)
		return nil, status.Error(codes.Unauthenticated, "signature authentication failed")
	}

	newCtx := WithAccountId(ctx, accountId)

	// 仅需认证的 self-scoped 方法：不加载快照、不做权限判定
	if _, ok := authOnlyMethods[info.FullMethod]; ok {
		return handler(newCtx, req)
	}

	// 签名鉴权不授予任何额外权限（FR-015）：以绑定用户的权限快照判定，无管理员旁路
	requiredKey := requiredPermissionKey(md, info.FullMethod)
	if err := authorizeBySnapshot(ctx, rdb, load, accountId, requiredKey, info.FullMethod); err != nil {
		return nil, err
	}
	return handler(newCtx, req)
}

// requiredPermissionKey 计算执行域权限键（D3）：网关来源 → http: 键；服务间直连 → rpc: 键。
func requiredPermissionKey(md metadata.MD, fullMethod string) string {
	if originValues := md.Get(constant.MetadataGatewayOrigin); len(originValues) > 0 && originValues[0] == constant.GatewayOriginHTTP {
		return constant.SnapshotKeyPrefixHTTP + fullMethod
	}
	return constant.SnapshotKeyPrefixRPC + fullMethod
}

// authorizeBySnapshot 权限快照判定（两轨共用）：
// Redis 快照命中即比对；未命中/格式不符走读穿回源重建；Redis 故障 fail-closed。
func authorizeBySnapshot(ctx context.Context, rdb *redis.Client, load PermissionLoader, accountId int64, requiredKey, fullMethod string) error {
	str, err := rdb.Get(ctx, constant.GetAccountPermissionKey(accountId)).Result()
	var permKeys []string
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			// Redis 其他故障 fail-closed：明确报错、不静默放行、不崩溃级联
			// （research.md D12；底层错误仅入日志不透出客户端）
			slog.Error("get permission snapshot from redis failed",
				"account_id", accountId, "method", fullMethod, "error", err)
			return status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
		}
		// 缓存未命中：读穿回源，从 DB 重建快照后继续校验（research.md D6-②）
		permKeys, err = LoadPermissionSnapshot(ctx, rdb, accountId, load)
		if err != nil {
			slog.Error("rebuild permission snapshot from db failed",
				"account_id", accountId, "method", fullMethod, "error", err)
			return status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
		}
	} else if err = json.Unmarshal([]byte(str), &permKeys); err != nil {
		// 快照格式不符（如旧格式残留）视为未命中：走读穿回源重建（research.md D8），不静默放行
		slog.Warn("unmarshal permission snapshot failed, rebuilding from db",
			"account_id", accountId, "method", fullMethod, "error", err)
		permKeys, err = LoadPermissionSnapshot(ctx, rdb, accountId, load)
		if err != nil {
			slog.Error("rebuild permission snapshot from db failed",
				"account_id", accountId, "method", fullMethod, "error", err)
			return status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
		}
	}

	for _, key := range permKeys {
		if key == requiredKey {
			return nil
		}
	}

	// fail-closed：无匹配键一律拒绝（FR-010）
	slog.Warn("permission denied",
		"account_id", accountId, "method", fullMethod, "required_key", requiredKey)
	return status.Error(codes.PermissionDenied, "permission denied")
}

// signatureStatusError 把语义化验签错误转成携带 ErrorInfo（reason）的 gRPC 状态。
func signatureStatusError(err *SignatureError) error {
	st := status.New(err.GRPCCode(), err.Error())
	withDetails, detailErr := st.WithDetails(&errdetails.ErrorInfo{
		Reason: err.BusinessCode(),
		Domain: errorDomain,
	})
	if detailErr != nil {
		return st.Err()
	}
	return withDetails.Err()
}

func signatureOrigin(md metadata.MD) string {
	if originValues := md.Get(constant.MetadataGatewayOrigin); len(originValues) > 0 {
		return originValues[0]
	}
	return "grpc"
}

func traceIDFromMetadata(md metadata.MD) string {
	values := md.Get(traceIDHeader)
	if len(values) == 0 {
		return ""
	}
	return values[0]
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
