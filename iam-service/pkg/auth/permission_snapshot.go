package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"iam-service/pkg/constant"
)

// PermissionLoader 从 DB 重建账号权限快照的回源函数。
// 由 service 层注入实现（依赖 DB），避免 auth ↔ service 循环依赖（research.md D8 v2）。
type PermissionLoader func(ctx context.Context, accountId int64) ([]string, error)

// LoadPermissionSnapshot 读穿回源：调用 load 从 DB 重建账号权限快照并写回 Redis，
// TTL 取 constant.AccountPermissionSnapshotTTL（research.md D6-②/D6-③）。
// 写回失败仅记日志不返回错误——快照为派生数据，下次未命中可再次重建。
func LoadPermissionSnapshot(ctx context.Context, rdb *redis.Client, accountId int64, load PermissionLoader) ([]string, error) {
	codes, err := load(ctx, accountId)
	if err != nil {
		return nil, fmt.Errorf("rebuild permission snapshot from db: %w", err)
	}

	data, err := json.Marshal(codes)
	if err != nil {
		return nil, err
	}
	if err := rdb.Set(ctx, constant.GetAccountPermissionKey(accountId), data, constant.AccountPermissionSnapshotTTL).Err(); err != nil {
		slog.Warn("write back permission snapshot failed", "account_id", accountId, "error", err)
	}
	return codes, nil
}
