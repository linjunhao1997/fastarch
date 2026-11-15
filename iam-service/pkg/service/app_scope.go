package service

import (
	"context"
	"errors"
	"strings"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	"gorm.io/gorm"
)

// 多应用数据隔离（裁决 #8）：应用上下文由 token 承载（JWT jti → Redis 映射，
// 机器令牌取 claim），拦截器注入 ctx；服务层统一经本文件取用，业务契约不出现
// application_id 参数。未选应用上下文时 fail-closed。

// requireApplicationId 返回当前请求的应用上下文 id；未选应用时返回明确错误。
func requireApplicationId(ctx context.Context) (int64, error) {
	appId := auth.GetApplicationId(ctx)
	if appId == 0 {
		return 0, ErrApplicationContextRequired
	}
	return appId, nil
}

// permissionCodeSegment 返回点式权限码的服务段（iam.account.list → iam）。
func permissionCodeSegment(code string) string {
	if idx := strings.Index(code, "."); idx > 0 {
		return code[:idx]
	}
	return code
}

// requireCodeSegmentMatchesApp 校验权限码服务段与当前应用 code 一致
// （服务段隔离裁决：码的命名空间必须等于所属应用注册的服务段）。
func requireCodeSegmentMatchesApp(ctx context.Context, db *gorm.DB, appId int64, code string) error {
	var app model.Application
	err := db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnId, appId).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFoundForScope
		}
		return err
	}
	if permissionCodeSegment(code) != app.Code {
		return ErrPermissionCodeSegmentMismatch
	}
	return nil
}
