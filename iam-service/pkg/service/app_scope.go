package service

import (
	"context"
	"errors"
	"strings"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	"gorm.io/gorm"
)

// 多应用数据隔离（裁决 #8）：宿主固定 appId 由拦截器从配置注入 ctx（菜单/字典用），
// 数据归属 appId 由请求头 appCode 校验后注入（业务数据用）；两者经本文件取用，
// 业务契约不出现 application_id 参数。未注入应用上下文时 fail-closed。

// requireApplicationId 返回宿主固定 appId（菜单/字典与管理域隔离用）；未注入时返回明确错误。
func requireApplicationId(ctx context.Context) (int64, error) {
	appId := auth.GetApplicationId(ctx)
	if appId == 0 {
		return 0, ErrApplicationContextRequired
	}
	return appId, nil
}

// ResolveDataAppByCode 导出构造 appCode 解析器（供 server 注入拦截器）。
func ResolveDataAppByCode(db *gorm.DB) auth.AppResolver {
	return resolveDataAppIdByCode(db)
}

// resolveDataAppIdByCode 把请求头 x-fa-app-code 解析为启用应用 id（auth.AppResolver 实现）。
// 供拦截器在 JWT 轨校验选择信号：只做 code→id + 启用校验，准入校验由 IsAppAdmitted 按
// 签名 claim 完成；停用/不存在返回 ErrApplicationNotFoundForScope。
func resolveDataAppIdByCode(db *gorm.DB) auth.AppResolver {
	return func(ctx context.Context, code string) (int64, error) {
		if strings.TrimSpace(code) == "" {
			return 0, ErrApplicationNotFoundForScope
		}
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
		return app.Id, nil
	}
}

// operatorDataScope 返回当前请求的数据归属应用（数据边界）：
//   - admin 与系统调用（accountId=0）旁路，scoped=false；
//   - 非 admin 优先取拦截器校验后注入的 dataAppId（来自请求头 appCode，
//     已校验 ∈ 接入集合）；未注入时回落到用户的 home（默认落地应用，
//     兼容未带 appCode 的内部/历史调用）。查不到用户或结果<=0 返回 (0,true) fail-closed。
//
// 该函数是所有账号/角色/权限/凭证隔离逻辑的统一边界来源。
func operatorDataScope(ctx context.Context, db *gorm.DB) (appId int64, scoped bool, err error) {
	if auth.IsAdminFromContext(ctx) || auth.GetAccountId(ctx) == 0 {
		return 0, false, nil
	}
	if dataAppId := auth.GetDataAppId(ctx); dataAppId > 0 {
		return dataAppId, true, nil
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

// operatorHomeScope 保留为 operatorDataScope 的别名（历史调用点兼容）。
func operatorHomeScope(ctx context.Context, db *gorm.DB) (int64, bool, error) {
	return operatorDataScope(ctx, db)
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

// dataScopeApplicationId 返回管理数据的边界应用 id（ADR 0006 + ADR 0008）：
//   - 非 admin 取 operatorDataScope（优先 dataAppId、回落 home），scoped=true；
//     home=0 时返回 (0,true)，由调用方 fail-closed；
//   - admin 与系统调用（accountId=0）取宿主固定 appId（拦截器从配置注入），scoped=false。
//
// 账号/角色/权限等管理数据用它取代“当前应用”作为过滤来源，保证登录到准入应用
// 也不会暴露 home 之外的数据。
func dataScopeApplicationId(ctx context.Context, db *gorm.DB) (appId int64, scoped bool, err error) {
	hostAppId, err := requireApplicationId(ctx)
	if err != nil {
		return 0, false, err
	}
	scopeAppId, scoped, err := operatorDataScope(ctx, db)
	if err != nil {
		return 0, scoped, err
	}
	if scoped {
		return scopeAppId, true, nil
	}
	return hostAppId, false, nil
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
