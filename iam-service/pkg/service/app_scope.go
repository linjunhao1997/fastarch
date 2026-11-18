package service

import (
	"context"
	"errors"
	"log/slog"
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

// resolveLoginApplication 登录时解析目标应用（ADR 0007）：
//   - 按 code 查启用应用，不存在/停用 → ErrApplicationNotFoundForScope；
//   - 准入校验 fail-closed：admin 旁路；非 admin 必须所属应用（home）或
//     t_r_user_application 准入绑定，否则 ErrApplicationAccessDenied。
//
// 返回锁定进签名 JWT claim 的 application_id，此后不可变。
func resolveLoginApplication(ctx context.Context, db *gorm.DB, userId, homeAppId int64, isAdmin bool, code string) (int64, error) {
	var app model.Application
	err := db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnCode, code).
		Where(model.ApplicationColumnStatus, model.StatusEnabled).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrApplicationNotFoundForScope
		}
		return 0, err
	}
	if isAdmin {
		return app.Id, nil
	}
	// 所属应用隐式准入；其余应用须在 t_r_user_application 绑定（ADR 0006）。
	if homeAppId == app.Id {
		return app.Id, nil
	}
	var count int64
	err = db.WithContext(ctx).
		Model(model.RUserApplication{}).
		Where(model.RUserApplicationColumnUserId+" = ? AND "+model.RUserApplicationColumnApplicationId+" = ?", userId, app.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("login app access check failed", "user_id", userId, "application_id", app.Id, "error", err)
		return 0, err
	}
	if count == 0 {
		return 0, ErrApplicationAccessDenied
	}
	return app.Id, nil
}

// operatorHomeScope 返回操作者的所属应用（数据边界，ADR 0006）：
//   - admin 与系统调用（accountId=0）旁路，scoped=false；
//   - 非 admin 查库取 home（JWT 只带登录锁定的 application_id，不带 home）；
//     查不到用户或 home<=0 时返回 (0,true)，由调用方 fail-closed。
func operatorHomeScope(ctx context.Context, db *gorm.DB) (homeId int64, scoped bool, err error) {
	if auth.IsAdminFromContext(ctx) || auth.GetAccountId(ctx) == 0 {
		return 0, false, nil
	}
	var operator model.User
	err = db.WithContext(ctx).
		Select(model.UserColumnId, model.UserColumnHomeApplicationId).
		Where(model.UserColumnId, auth.GetAccountId(ctx)).
		First(&operator).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, true, nil
		}
		return 0, true, err
	}
	return operator.HomeApplicationId, true, nil
}

// requireRoleScope 校验目标角色的归属应用与操作者 home 一致（ADR 0006 扩展到角色）。
func requireRoleScope(ctx context.Context, db *gorm.DB, roleAppId int64) error {
	homeId, scoped, err := operatorHomeScope(ctx, db)
	if err != nil {
		return err
	}
	if !scoped {
		return nil
	}
	if homeId <= 0 || homeId != roleAppId {
		return ErrRoleOutOfScope
	}
	return nil
}

// dataScopeApplicationId 返回管理数据的边界应用 id（ADR 0006）：
//   - 非 admin 以所属应用（home）为准，scoped=true；home=0 时返回 (0,true)，由调用方 fail-closed；
//   - admin 与系统调用（accountId=0）沿用请求的登录应用上下文，scoped=false。
//
// 账号/角色/权限等管理数据用它取代“当前应用”作为过滤来源，保证登录到准入应用
// 也不会暴露 home 之外的数据。
func dataScopeApplicationId(ctx context.Context, db *gorm.DB) (appId int64, scoped bool, err error) {
	requestAppId, err := requireApplicationId(ctx)
	if err != nil {
		return 0, false, err
	}
	homeId, scoped, err := operatorHomeScope(ctx, db)
	if err != nil {
		return 0, scoped, err
	}
	if scoped {
		return homeId, true, nil
	}
	return requestAppId, false, nil
}

// requirePermissionScope 校验目标权限码的归属应用与操作者数据边界一致；
// 越界返回 ErrPermissionOutOfScope（1001065），home 未设置返回 ErrHomeApplicationRequired。
func requirePermissionScope(ctx context.Context, db *gorm.DB, permissionAppId int64) error {
	scopeAppId, scoped, err := dataScopeApplicationId(ctx, db)
	if err != nil {
		return err
	}
	if scoped && scopeAppId <= 0 {
		return ErrHomeApplicationRequired
	}
	if scopeAppId != permissionAppId {
		return ErrPermissionOutOfScope
	}
	return nil
}

// requireAdminOperator 应用域写操作硬门槛（ADR 0006 补充裁决）：仅 admin 或
// 系统调用（accountId=0，内部/迁移场景）可执行；非 admin 即使持有应用管理码也拒绝
// （码可经角色绑定自授，码门槛不构成租户边界，须服务层兜底）。
func requireAdminOperator(ctx context.Context) error {
	if auth.IsAdminFromContext(ctx) || auth.GetAccountId(ctx) == 0 {
		return nil
	}
	return ErrApplicationAdminRequired
}

// requireHomeMatch 校验目标 home 与操作者数据边界一致（账号域语义）：
// admin/系统调用旁路；非 admin home 未设置或不一致一律 ErrAccountOutOfScope。
func requireHomeMatch(ctx context.Context, db *gorm.DB, targetHomeId int64) error {
	homeId, scoped, err := operatorHomeScope(ctx, db)
	if err != nil {
		return err
	}
	if !scoped {
		return nil
	}
	if homeId <= 0 || homeId != targetHomeId {
		return ErrAccountOutOfScope
	}
	return nil
}

// requireRoleBindScope 校验待绑定角色清单的数据边界（ADR 0006 补充：绑定纳入隔离）：
// 非 admin 一律禁绑内置超管角色（is_super_admin 标志，登录按其计算 IsAdmin，
// 绑定即提权）；其余角色归属须等于操作者 home。admin/系统调用旁路。
func requireRoleBindScope(ctx context.Context, db *gorm.DB, roles []model.Role) error {
	homeId, scoped, err := operatorHomeScope(ctx, db)
	if err != nil {
		return err
	}
	if !scoped {
		return nil
	}
	for _, role := range roles {
		if role.IsSuperAdmin {
			return ErrRoleAdminBindForbidden
		}
		if homeId <= 0 || homeId != role.ApplicationId {
			return ErrRoleOutOfScope
		}
	}
	return nil
}

// requirePermissionBindScope 校验待挂权限码清单的数据边界（ADR 0006 补充）：
// 非 admin 仅可把 home 一致的码挂到角色上，防止经挂码自授跨应用能力。
func requirePermissionBindScope(ctx context.Context, db *gorm.DB, permissions []model.Permission) error {
	homeId, scoped, err := operatorHomeScope(ctx, db)
	if err != nil {
		return err
	}
	if !scoped {
		return nil
	}
	if homeId <= 0 {
		return ErrHomeApplicationRequired
	}
	for _, permission := range permissions {
		if permission.ApplicationId != homeId {
			return ErrPermissionOutOfScope
		}
	}
	return nil
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
