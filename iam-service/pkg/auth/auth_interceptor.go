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

// Interceptor 构造业务鉴权拦截器。load 为权限快照读穿回源函数（缓存未命中时
// 从 DB 重建快照，research.md D6-②），由 server 组装时注入。
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

		if isAdmin {
			return handler(newCtx, req)
		}

		accountIdInt := cast.ToInt64(accountId)
		str, err := rdb.Get(ctx, constant.GetAccountPermissionKey(accountIdInt)).Result()
		var permCodes []string
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				// Redis 其他故障 fail-closed：明确报错、不静默放行、不崩溃级联
				// （research.md D12；底层错误仅入日志不透出客户端）
				slog.Error("get permission snapshot from redis failed",
					"account_id", accountIdInt, "method", info.FullMethod, "error", err)
				return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
			}
			// 缓存未命中：读穿回源，从 DB 重建快照后继续校验（research.md D6-②）
			permCodes, err = LoadPermissionSnapshot(ctx, rdb, accountIdInt, load)
			if err != nil {
				slog.Error("rebuild permission snapshot from db failed",
					"account_id", accountId, "method", info.FullMethod, "error", err)
				return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
			}
		} else if err = json.Unmarshal([]byte(str), &permCodes); err != nil {
			slog.Error("unmarshal permission snapshot failed",
				"account_id", accountId, "method", info.FullMethod, "error", err)
			return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
		}

		for _, perm := range permCodes {
			if perm == constant.ApplicationCode+":"+info.FullMethod {
				return handler(newCtx, req)
			}
		}

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
