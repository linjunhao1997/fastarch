package auth

import (
	"context"
	"encoding/json"
	"iam-service/pkg/constant"
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

func Interceptor(rdb *redis.Client) grpc.UnaryServerInterceptor {
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

		str, err := rdb.Get(ctx, constant.GetAccountPermissionKey(cast.ToInt64(accountId))).Result()
		if err != nil {
			return nil, err
		}

		var permCodes []string
		if err = json.Unmarshal([]byte(str), &permCodes); err != nil {
			return nil, err
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
