package model

import "regexp"

// 权限码词汇表与 API→码映射（011 权限码模型）：
//   - 权限码是唯一授权词汇，格式为 服务.模块.操作（如 iam.user.delete）
//   - API→码映射声明在代码内 METHOD_PERMISSIONS（full method → 所需权限码，须全部持有）
//   - 新增 RPC：在 METHOD_PERMISSIONS 增加映射 + PERMISSION_CODES 增加码
//     （存量库经数据迁移补种权限行）

// PERMISSION_CODES 是全部 API 权限码词汇（与迁移 SQL 000003+000005 种子镜像，
// 共 43 个码：000005 补种 iam.user.reset_password）。
var PERMISSION_CODES = []string{
	// 用户模块（iam.account.* 已随 011 模型改名 iam.user.*）
	"iam.user.create",
	"iam.user.get",
	"iam.user.list",
	"iam.user.update",
	"iam.user.delete",
	"iam.user.assign_roles",
	"iam.user.reset_password",
	// 应用模块
	"iam.application.create",
	"iam.application.get",
	"iam.application.list",
	"iam.application.update",
	"iam.application.delete",
	// 权限模块
	"iam.permission.create",
	"iam.permission.get",
	"iam.permission.list",
	"iam.permission.update",
	"iam.permission.delete",
	// 角色模块
	"iam.role.create",
	"iam.role.get",
	"iam.role.list",
	"iam.role.update",
	"iam.role.delete",
	// 权限订阅模块
	"iam.subscription.create",
	"iam.subscription.get",
	"iam.subscription.update",
	"iam.subscription.list",
	// 访问凭证模块（AK/SK）
	"iam.access_credential.create",
	"iam.access_credential.get",
	"iam.access_credential.list",
	"iam.access_credential.update",
	"iam.access_credential.status",
	"iam.access_credential.reset_secret",
	"iam.access_credential.delete",
	// 菜单模块（组件资源模型已废弃，菜单改由 t_menu 承担）
	"iam.menu.create",
	"iam.menu.get",
	"iam.menu.list",
	"iam.menu.update",
	"iam.menu.delete",
	// 字典模块
	"iam.dict.create",
	"iam.dict.get",
	"iam.dict.list",
	"iam.dict.update",
	"iam.dict.delete",
}

// METHOD_PERMISSIONS 是 gRPC full method → 所需权限码的映射（011 裁决：结构归代码）。
// 判定语义：账号快照须包含映射清单中的全部权限码（fail-closed）；
// 未登记的方法一律拒绝。登录、自查类方法不在此表（拦截器 authOnly 豁免）。
var METHOD_PERMISSIONS = map[string][]string{
	// AccountService（用户服务语义，proto 方法名未动）
	"/iam.v1.AccountService/CreateAccount":      {"iam.user.create"},
	"/iam.v1.AccountService/GetAccount":         {"iam.user.get"},
	"/iam.v1.AccountService/ListAccounts":       {"iam.user.list"},
	"/iam.v1.AccountService/UpdateAccount":      {"iam.user.update"},
	"/iam.v1.AccountService/DeleteAccount":      {"iam.user.delete"},
	"/iam.v1.AccountService/UpdateAccountRoles": {"iam.user.assign_roles"},
	"/iam.v1.AccountService/ResetAccountPassword": {"iam.user.reset_password"},
	// ApplicationService
	"/iam.v1.ApplicationService/CreateApplication": {"iam.application.create"},
	"/iam.v1.ApplicationService/GetApplication":    {"iam.application.get"},
	"/iam.v1.ApplicationService/ListApplications":  {"iam.application.list"},
	"/iam.v1.ApplicationService/UpdateApplication": {"iam.application.update"},
	"/iam.v1.ApplicationService/DeleteApplication": {"iam.application.delete"},
	// MenuService（菜单 = t_menu + t_r_role_menu，组件资源模型已废弃）
	"/iam.v1.MenuService/CreateMenu":     {"iam.menu.create"},
	"/iam.v1.MenuService/GetMenu":        {"iam.menu.get"},
	"/iam.v1.MenuService/ListMenuTree":   {"iam.menu.list"},
	"/iam.v1.MenuService/UpdateMenu":     {"iam.menu.update"},
	"/iam.v1.MenuService/DeleteMenu":     {"iam.menu.delete"},
	"/iam.v1.MenuService/UpdateRoleMenus": {"iam.menu.update"},
	"/iam.v1.MenuService/ListRoleMenus":   {"iam.menu.get"},
	// DictService
	"/iam.v1.DictService/CreateDictType": {"iam.dict.create"},
	"/iam.v1.DictService/UpdateDictType": {"iam.dict.update"},
	"/iam.v1.DictService/DeleteDictType": {"iam.dict.delete"},
	"/iam.v1.DictService/ListDictTypes":  {"iam.dict.list"},
	"/iam.v1.DictService/CreateDictItem": {"iam.dict.create"},
	"/iam.v1.DictService/UpdateDictItem": {"iam.dict.update"},
	"/iam.v1.DictService/DeleteDictItem": {"iam.dict.delete"},
	"/iam.v1.DictService/ListDictItems":  {"iam.dict.list"},
	// AuthorizationService：权限
	"/iam.v1.AuthorizationService/CreatePermission": {"iam.permission.create"},
	"/iam.v1.AuthorizationService/GetPermission":    {"iam.permission.get"},
	"/iam.v1.AuthorizationService/ListPermissions":  {"iam.permission.list"},
	"/iam.v1.AuthorizationService/UpdatePermission": {"iam.permission.update"},
	"/iam.v1.AuthorizationService/DeletePermission": {"iam.permission.delete"},
	// AuthorizationService：角色
	"/iam.v1.AuthorizationService/CreateRole": {"iam.role.create"},
	"/iam.v1.AuthorizationService/GetRole":    {"iam.role.get"},
	"/iam.v1.AuthorizationService/ListRoles":  {"iam.role.list"},
	"/iam.v1.AuthorizationService/UpdateRole": {"iam.role.update"},
	"/iam.v1.AuthorizationService/DeleteRole": {"iam.role.delete"},
	// AccessCredentialService（specs/009-ak-sk-auth）
	"/iam.v1.AccessCredentialService/CreateAccessCredential":       {"iam.access_credential.create"},
	"/iam.v1.AccessCredentialService/GetAccessCredential":          {"iam.access_credential.get"},
	"/iam.v1.AccessCredentialService/ListAccessCredentials":        {"iam.access_credential.list"},
	"/iam.v1.AccessCredentialService/UpdateAccessCredential":       {"iam.access_credential.update"},
	"/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus": {"iam.access_credential.status"},
	"/iam.v1.AccessCredentialService/ResetAccessCredentialSecret":  {"iam.access_credential.reset_secret"},
	"/iam.v1.AccessCredentialService/DeleteAccessCredential":       {"iam.access_credential.delete"},
}

var permissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)

// IsValidPermissionCode 校验权限码格式（服务.模块.操作，如 iam.user.delete）。
func IsValidPermissionCode(code string) bool {
	return permissionCodePattern.MatchString(code)
}

// MethodPermissionsFor 返回 full method 所需的权限码清单（须全部持有）；
// 未登记方法返回 nil（拦截器 fail-closed 拒绝）。
func MethodPermissionsFor(fullMethod string) []string {
	return METHOD_PERMISSIONS[fullMethod]
}
