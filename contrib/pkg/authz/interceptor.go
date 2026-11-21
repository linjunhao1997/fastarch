package authz

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authorizationHeader = "authorization"
	cookieHeader        = "cookie"
	bearerPrefix        = "Bearer "
)

// Config 参数化判权拦截器，由宿主（如 sender）注入自身配置。
type Config struct {
	// ApplicationId 宿主应用的固定 id（如 sender=2），注入 ctx 供菜单/字典数据隔离用。
	// 不用 token 里的 applicationId——那是 IAM 多应用控制台才需要的运行时切换语义。
	ApplicationId int64
	// AuthOnlyMethods 仅需登录态、不判权的方法（如 ListMyMenuTree）。
	// 注意：这些方法跳过了 IAM 判权，其 ctx 身份无 IAM 背书，不得据此做写操作的归责。
	AuthOnlyMethods map[string]struct{}
	// MethodPerms full method → 所需权限码。返回空/ nil 一律拒绝（fail-closed）。
	MethodPerms func(fullMethod string) []string
	// ErrorDomain 写入拒绝日志的域标识（如 "sender-service"）。
	ErrorDomain string
}

// Interceptor 构造"判权委托 IAM"的 unary 拦截器。流程：
//  1. 免判权方法（AuthOnlyMethods）→ 解析身份注入 ctx（无 IAM 背书）后放行
//  2. 其余方法 → 透传 JWT 调 IAM CheckPermission（admin 无本地旁路）
//  3. ok → 解析身份注入 ctx、放行；否则/调用失败/未登记码 → 拒绝（fail-closed）
//
// 身份字段只用于审计；授权唯一依据是 IAM 的 ok。
func Interceptor(client *Client, cfg Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := cfg.AuthOnlyMethods[info.FullMethod]; ok {
			claims, _ := claimsFromContext(ctx) // 解析失败也要放行 self-scoped 方法，身份记为 0
			return handler(withIdentitySafe(ctx, claims, cfg.ApplicationId), req)
		}

		required := cfg.MethodPerms(info.FullMethod)
		if len(required) == 0 {
			slog.Warn("permission denied: method not registered",
				"domain", cfg.ErrorDomain, "method", info.FullMethod)
			return nil, status.Error(codes.PermissionDenied, "permission denied")
		}

		ok, dataAppId, err := client.CheckPermission(ctx, required[0])
		if err != nil {
			slog.Error("iam check permission failed (fail-closed)",
				"domain", cfg.ErrorDomain, "method", info.FullMethod, "error", err)
			switch status.Code(err) {
			case codes.Unauthenticated:
				return nil, status.Error(codes.Unauthenticated, "未登录或登录已过期")
			case codes.PermissionDenied:
				return nil, status.Error(codes.PermissionDenied, "permission denied")
			default:
				return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
			}
		}
		if !ok {
			slog.Warn("permission denied by iam",
				"domain", cfg.ErrorDomain, "method", info.FullMethod, "required", required[0])
			return nil, status.Error(codes.PermissionDenied, "permission denied")
		}

		// IAM 已背书：解析身份仅供审计，注入后放行。dataAppId 为 IAM 校验后的
		// 数据归属应用，注入 ctx 供业务服务隔离（menu/dict 仍用 cfg.ApplicationId）。
		claims, _ := claimsFromContext(ctx)
		newCtx := withIdentitySafe(ctx, claims, cfg.ApplicationId)
		if dataAppId > 0 {
			newCtx = WithDataAppId(newCtx, dataAppId)
		}
		// appCode 已随 metadata 透传由 IAM 校验通过，回读供展示摘要回填。
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(MetadataAppCode); len(values) > 0 {
				newCtx = WithDataAppCode(newCtx, values[0])
			}
		}
		return handler(newCtx, req)
	}
}

// claimsFromContext 从入站 metadata 取 token 并无验签解析。
func claimsFromContext(ctx context.Context) (*Claims, error) {
	token := extractToken(ctx)
	return ParseUnverified(token)
}

// withIdentitySafe 有身份则注入，无身份退化为仅注入 appId（审计字段记 0）。
func withIdentitySafe(ctx context.Context, claims *Claims, applicationId int64) context.Context {
	ctx = context.WithValue(ctx, contextApplicationId, applicationId)
	if claims != nil {
		ctx = context.WithValue(ctx, contextAccountId, claims.AccountId)
		ctx = context.WithValue(ctx, contextTokenJti, claims.Jti)
	}
	return ctx
}

// extractToken 从入站 metadata 的 authorization 头或 cookie 取 JWT，与 iam 一致。
func extractToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if values := md.Get(authorizationHeader); len(values) > 0 {
		header := values[0]
		if strings.HasPrefix(header, bearerPrefix) {
			return strings.TrimPrefix(header, bearerPrefix)
		}
		return strings.TrimSpace(header)
	}
	if values := md.Get(cookieHeader); len(values) > 0 {
		cookies, err := http.ParseCookie(values[0])
		if err != nil {
			return ""
		}
		for _, cookie := range cookies {
			if cookie.Name == authorizationHeader {
				return cookie.Value
			}
		}
	}
	return ""
}
