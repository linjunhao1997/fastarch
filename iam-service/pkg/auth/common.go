package auth

import (
	"context"
	"github.com/spf13/cast"
)

const (
	Authorization    = "authorization"
	contextAccountId = "accountId"
)

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
