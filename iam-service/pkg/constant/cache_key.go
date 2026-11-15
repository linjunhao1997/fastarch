package constant

import (
	"time"

	"github.com/spf13/cast"
)

// AccountPermissionSnapshotTTL 是账号权限快照（account:<id>:permissions）的缓存 TTL。
// 登录写入与读穿回源写回共用该常量；短 TTL 作为"失效-重建竞态"的时间上界（research.md D6-③）。
const AccountPermissionSnapshotTTL = 5 * time.Minute

func GetAccountPermissionKey(accountId int64) string {
	return "account:" + cast.ToString(accountId) + ":permissions"
}

// AppContextTTL 是令牌当前应用映射（auth:app-ctx:{jti}）的缓存 TTL。
// 与登录 JWT 有效期（30 分钟）对齐：映射不晚于令牌失效，避免"令牌已过期但映射
// 仍占用 Redis"的悬挂；令牌若续期需重新绑定。
const AppContextTTL = 30 * time.Minute

func GetAppContextKey(jti string) string {
	return "auth:app-ctx:" + jti
}
