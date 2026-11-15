package auth

import (
	"context"
	"github.com/spf13/cast"
)

const (
	Authorization         = "authorization"
	contextAccountId      = "accountId"
	contextTokenJti       = "auth.token_jti"
	contextApplicationId  = "auth.application_id"
)

// GetTokenJti 返回当前请求令牌唯一标识（app 上下文映射键）；无令牌上下文为空串。
func GetTokenJti(ctx context.Context) string {
	jti, _ := ctx.Value(contextTokenJti).(string)
	return jti
}

func WithTokenJti(ctx context.Context, jti string) context.Context {
	return context.WithValue(ctx, contextTokenJti, jti)
}

// GetApplicationId 返回当前请求解析出的目标应用 id（多应用数据隔离分区键）；
// 未绑定应用时为 0，需应用上下文的接口据此 fail-closed。
func GetApplicationId(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(contextApplicationId))
}

func WithApplicationId(ctx context.Context, applicationId int64) context.Context {
	return context.WithValue(ctx, contextApplicationId, applicationId)
}

func GetAccountId(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(contextAccountId))
}

func WithAccountId(ctx context.Context, accountId int64) context.Context {
	return context.WithValue(ctx, contextAccountId, accountId)
}

func WithAdmin(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextIsAdmin, true)
}

func IsAdminFromContext(ctx context.Context) bool {
	isAdmin, ok := ctx.Value(contextIsAdmin).(bool)
	return ok && isAdmin
}
