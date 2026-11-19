package auth

import (
	"context"
	"github.com/spf13/cast"
)

const (
	Authorization        = "authorization"
	contextAccountId     = "accountId"
	contextTokenJti      = "auth.token_jti"
	contextApplicationId = "auth.application_id"
	// contextDataAppId 是当前请求的数据归属应用（从 x-fa-app-code 解析），
	// 与 contextApplicationId（宿主固定 appId，菜单/字典用）正交。
	contextDataAppId = "auth.data_app_id"
	// MetadataAppCode 是请求选择数据归属应用的头（网关小写透传进 metadata）。
	// 它只是选择信号，须经 validateAppCodeAdmitted 校验后才可用。
	MetadataAppCode = "x-fa-app-code"
)

// GetTokenJti 返回当前请求令牌唯一标识（app 上下文映射键）；无令牌上下文为空串。
func GetTokenJti(ctx context.Context) string {
	jti, _ := ctx.Value(contextTokenJti).(string)
	return jti
}

func WithTokenJti(ctx context.Context, jti string) context.Context {
	return context.WithValue(ctx, contextTokenJti, jti)
}

// GetApplicationId 返回宿主固定 appId（菜单/字典与管理域隔离用）。
// 由拦截器从服务配置注入，与令牌无关；未注入时为 0，需应用上下文的接口据此 fail-closed。
func GetApplicationId(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(contextApplicationId))
}

// WithApplicationId 注入宿主固定 appId（仅供拦截器与测试构造使用）。
func WithApplicationId(ctx context.Context, applicationId int64) context.Context {
	return context.WithValue(ctx, contextApplicationId, applicationId)
}

// GetDataAppId 返回当前请求的数据归属应用 id（由 appCode 校验通过后注入）；
// 未注入时为 0，需要数据隔离的服务据此 fail-closed。
func GetDataAppId(ctx context.Context) int64 {
	return cast.ToInt64(ctx.Value(contextDataAppId))
}

// WithDataAppId 注入当前请求的数据归属应用 id。
func WithDataAppId(ctx context.Context, appId int64) context.Context {
	return context.WithValue(ctx, contextDataAppId, appId)
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
