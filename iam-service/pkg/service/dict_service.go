package service

import (
	"log/slog"

	"iam-service/pkg/auth"

	"github.com/linjunhao1997/contrib/pkg/dict"
	"gorm.io/gorm"
)

// NewDictService 构建通用字典能力（contrib/pkg/dict，ADR 0001），注入 iam 宿主钩子：
// 应用上下文取宿主固定 appId（拦截器从配置注入 ctx），账号身份取自 auth ctx（ADR 0008）。
// 钩子在编译期静态齐全，NewDictService 的校验错误不可达。
func NewDictService(db *gorm.DB) *dict.DictService {
	svc, err := dict.NewDictService(dict.Config{
		DB:                       db,
		ApplicationIdFromContext: requireApplicationId,
		AccountIdFromContext:     auth.GetAccountId,
	})
	if err != nil {
		// 钩子静态齐全，理论不可达；防御性兜底
		slog.Error("build contrib dict service failed", "error", err)
		panic(err)
	}
	return svc
}
