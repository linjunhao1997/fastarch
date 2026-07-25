package constant

import "github.com/spf13/cast"

func GetAccountPermissionKey(accountId int64) string {
	return "account:" + cast.ToString(accountId) + ":permissions"
}
