package service

import (
	"context"

	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/spf13/cast"
)

// operatorFromContext 取当前操作者（审计字段服务端注入，宪法 XII）。
// 身份由入站 authz 拦截器注入 ctx（仅审计用）；无身份时为空串。
func operatorFromContext(ctx context.Context) string {
	return cast.ToString(authz.GetAccountId(ctx))
}

// translateNotFound 把 gorm 记录不存在转成本域业务错误，其余原样透传。
func toString(id int64) string {
	return cast.ToString(id)
}
