package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"iam-service/pkg/constant"

	"github.com/redis/go-redis/v9"
)

// SetAppContext 记录令牌 jti 当前映射的应用 id（切换应用即覆写，不重签令牌）。
// TTL 由调用方按令牌有效期传入；Redis 故障返回错误，由上层决定 fail-closed。
func SetAppContext(ctx context.Context, rdb *redis.Client, jti string, applicationId int64, ttl time.Duration) error {
	if jti == "" {
		return errors.New("empty jti")
	}
	return rdb.Set(ctx, constant.GetAppContextKey(jti), applicationId, ttl).Err()
}

// GetAppContext 读取令牌 jti 当前映射的应用 id。
// 返回 (id, true, nil) 命中；(0, false, nil) 未绑定（redis.Nil）；
// (0, false, err) Redis 故障（调用方 fail-closed）。
func GetAppContext(ctx context.Context, rdb *redis.Client, jti string) (int64, bool, error) {
	if jti == "" {
		return 0, false, nil
	}
	id, err := rdb.Get(ctx, constant.GetAppContextKey(jti)).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

// ClearAppContext 删除令牌 jti 的应用映射（登出/令牌失效时清理，可选）。
func ClearAppContext(ctx context.Context, rdb *redis.Client, jti string) error {
	if jti == "" {
		return nil
	}
	return rdb.Del(ctx, constant.GetAppContextKey(jti)).Err()
}

// withResolvedApplication 解析当前请求的目标应用并注入 ctx。
// 优先级：Redis app:ctx:{jti}（人用登录，切换不重签）→ claim.ApplicationId
// （外部/机器令牌签发即锁定）→ 0（未选应用；需要应用上下文的服务自行
// fail-closed）。Redis 故障不阻断请求：回落 claim 并记日志，需要应用
// 上下文的接口以 0 判定拒绝，安全语义仍是 fail-closed。
func withResolvedApplication(ctx context.Context, rdb *redis.Client, jti string, claimApplicationId int64) context.Context {
	if rdb != nil {
		appId, bound, err := GetAppContext(ctx, rdb, jti)
		if err != nil {
			slog.Error("resolve application context from redis failed",
				"jti", jti, "error", err)
			return WithApplicationId(ctx, claimApplicationId)
		}
		if bound {
			return WithApplicationId(ctx, appId)
		}
	}
	return WithApplicationId(ctx, claimApplicationId)
}
