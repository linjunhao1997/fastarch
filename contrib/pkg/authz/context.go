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
	// contextDataAppId 是 IAM 校验 appCode 后回传的数据归属应用 id（业务数据隔离用），
	// 与 contextApplicationId（宿主控制台固定值，菜单/字典用）正交。
	contextDataAppId   = "auth.data_app_id"
	contextDataAppCode = "auth.data_app_code"
	// MetadataAppCode 是请求选择数据归属应用的头（网关小写透传）。
	MetadataAppCode = "x-fa-app-code"
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

// WithDataAppId 注入 IAM 回传的数据归属应用 id。
func WithDataAppId(ctx context.Context, appId int64) context.Context {
	return context.WithValue(ctx, contextDataAppId, appId)
}

// GetDataAppId 返回 IAM 校验后的数据归属应用 id；未注入时为 0（业务服务据此回落宿主默认应用）。
func GetDataAppId(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(contextDataAppId))
}

// WithDataAppCode 注入请求选择的归属应用 code（已由 IAM 校验，仅供展示回填）。
func WithDataAppCode(ctx context.Context, code string) context.Context {
	return context.WithValue(ctx, contextDataAppCode, code)
}

// GetDataAppCode 返回请求选择的归属应用 code；未注入时为空。
func GetDataAppCode(ctx context.Context) string {
	code, _ := ctx.Value(contextDataAppCode).(string)
	return code
}

// GetTokenJti 返回当前请求令牌 jti，无则空串。
func GetTokenJti(ctx context.Context) string {
	jti, _ := ctx.Value(contextTokenJti).(string)
	return jti
}
