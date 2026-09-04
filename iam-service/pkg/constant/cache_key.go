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
