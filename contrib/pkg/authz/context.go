package authz

import (
	"context"

	"github.com/spf13/cast"
)

// context key 值与 iam 迁移前一致，便于跨服务共享 context 语义（如日志/trace 透传）。
const (
	contextAccountId     = "accountId"
	contextApplicationId = "auth.application_id"
	contextTokenJti      = "auth.token_jti"
)

// WithIdentity 把校验后的身份写入 ctx。注意：这里的身份来自无验签解析的 JWT，
// 只在 CheckPermission 通过后才应调用本函数（保证身份已被 IAM 背书）。
func WithIdentity(ctx context.Context, c *Claims, applicationId int64) context.Context {
	ctx = context.WithValue(ctx, contextAccountId, c.AccountId)
	ctx = context.WithValue(ctx, contextApplicationId, applicationId)
	ctx = context.WithValue(ctx, contextTokenJti, c.Jti)
	return ctx
}

// WithAccountId 单独注入账号 id（审计字段钩子用）。
func WithAccountId(ctx context.Context, accountId int64) context.Context {
	return context.WithValue(ctx, contextAccountId, accountId)
}

// GetAccountId 返回当前请求账号 id；未注入时为 0。
func GetAccountId(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(contextAccountId))
}

// GetApplicationId 返回当前请求应用 id（宿主配死的固定值）。
func GetApplicationId(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(contextApplicationId))
}

// GetTokenJti 返回当前请求令牌 jti，无则空串。
func GetTokenJti(ctx context.Context) string {
	jti, _ := ctx.Value(contextTokenJti).(string)
	return jti
}
