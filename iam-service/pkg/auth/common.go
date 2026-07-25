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
