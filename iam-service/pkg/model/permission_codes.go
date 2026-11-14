package model

import "regexp"

// 权限码词汇表与 API→码映射（011 权限码模型）：
//   - 权限码是唯一授权词汇，格式为 服务.模块.操作（如 iam.account.delete）
//   - API→码映射声明在代码内 methodMap（full method → 所需权限码，须全部持有）
//   - 新增 RPC：在 METHOD_PERMISSIONS 增加映射 + PERMISSION_CODES 增加码
//     （存量库经数据迁移补种权限行）

// PERMISSION_CODES 是全部 API 权限码词汇（与迁移 SQL 种子镜像）。
var PERMISSION_CODES = []string{
	// 账号模块
	"iam.account.create",
	"iam.account.get",
	"iam.account.list",
	"iam.account.update",
	"iam.account.delete",
	"iam.account.assign_roles",
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
	// 组件资源模块
	"iam.resource.create",
	"iam.resource.get",
	"iam.resource.list",
	"iam.resource.update",
	"iam.resource.delete",
}

// METHOD_PERMISSIONS 是 gRPC full method → 所需权限码的映射（011 裁决：结构归代码）。
// 判定语义：账号快照须包含映射清单中的全部权限码（fail-closed）；
// 未登记的方法一律拒绝。登录、自查类方法不在此表（拦截器 authOnly 豁免）。
var METHOD_PERMISSIONS = map[string][]string{
	// AccountService
	"/iam.v1.AccountService/CreateAccount":       {"iam.account.create"},
	"/iam.v1.AccountService/GetAccount":          {"iam.account.get"},
	"/iam.v1.AccountService/ListAccounts":        {"iam.account.list"},
	"/iam.v1.AccountService/UpdateAccount":       {"iam.account.update"},
	"/iam.v1.AccountService/DeleteAccount":       {"iam.account.delete"},
	"/iam.v1.AccountService/UpdateAccountRoles":  {"iam.account.assign_roles"},
	// ApplicationService
	"/iam.v1.ApplicationService/CreateApplication": {"iam.application.create"},
	"/iam.v1.ApplicationService/GetApplication":    {"iam.application.get"},
	"/iam.v1.ApplicationService/ListApplications":  {"iam.application.list"},
	"/iam.v1.ApplicationService/UpdateApplication": {"iam.application.update"},
	"/iam.v1.ApplicationService/DeleteApplication": {"iam.application.delete"},
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
	// AuthorizationService：权限订阅
	"/iam.v1.AuthorizationService/CreateSubscription": {"iam.subscription.create"},
	"/iam.v1.AuthorizationService/GetSubscription":    {"iam.subscription.get"},
	"/iam.v1.AuthorizationService/UpdateSubscription": {"iam.subscription.update"},
	"/iam.v1.AuthorizationService/ListSubscriptions":  {"iam.subscription.list"},
	// AuthorizationService：组件资源管理
	"/iam.v1.AuthorizationService/CreateResource": {"iam.resource.create"},
	"/iam.v1.AuthorizationService/GetResource":    {"iam.resource.get"},
	"/iam.v1.AuthorizationService/ListResources":  {"iam.resource.list"},
	"/iam.v1.AuthorizationService/ListResourceTree": {"iam.resource.list"},
	"/iam.v1.AuthorizationService/UpdateResource": {"iam.resource.update"},
	"/iam.v1.AuthorizationService/DeleteResource": {"iam.resource.delete"},
	// AccessCredentialService（specs/009-ak-sk-auth）
	"/iam.v1.AccessCredentialService/CreateAccessCredential":         {"iam.access_credential.create"},
	"/iam.v1.AccessCredentialService/GetAccessCredential":            {"iam.access_credential.get"},
	"/iam.v1.AccessCredentialService/ListAccessCredentials":          {"iam.access_credential.list"},
	"/iam.v1.AccessCredentialService/UpdateAccessCredential":         {"iam.access_credential.update"},
	"/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus":   {"iam.access_credential.status"},
	"/iam.v1.AccessCredentialService/ResetAccessCredentialSecret":    {"iam.access_credential.reset_secret"},
	"/iam.v1.AccessCredentialService/DeleteAccessCredential":         {"iam.access_credential.delete"},
}

var permissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)

// IsValidPermissionCode 校验权限码格式（服务.模块.操作，如 iam.account.delete）。
func IsValidPermissionCode(code string) bool {
	return permissionCodePattern.MatchString(code)
}

// MethodPermissionsFor 返回 full method 所需的权限码清单（须全部持有）；
// 未登记方法返回 nil（拦截器 fail-closed 拒绝）。
func MethodPermissionsFor(fullMethod string) []string {
	return METHOD_PERMISSIONS[fullMethod]
}

// ComponentSeed 是前端组件资源初始清单条目（结构归代码：与前端路由/页面镜像）。
type ComponentSeed struct {
	Id              int64
	Key             string
	Name            string
	Kind            string // menu | page | button
	ParentId        int64  // 0 表示根节点
	Order           int32
	PermissionCodes []string
}

// COMPONENT_SEEDS 是前端组件资源初始清单（与迁移 SQL 种子镜像）：
// 层级键为 menu，页面键为 page；管理员可在后台调整展示顺序与显隐，
// 也可创建 button 组件做页面内操作粒度的展示/置灰。
var COMPONENT_SEEDS = []ComponentSeed{
	{Id: 1001, Key: "iam", Name: "身份访问管理", Kind: ComponentKindMenu},
	{Id: 1002, Key: "account_management", Name: "账号管理", Kind: ComponentKindMenu, ParentId: 1001, Order: 1},
	{Id: 1003, Key: "application_management", Name: "应用管理", Kind: ComponentKindMenu, ParentId: 1001, Order: 2},
	{Id: 1004, Key: "authentication_management", Name: "认证管理", Kind: ComponentKindMenu, ParentId: 1001, Order: 3},
	{Id: 1005, Key: "authorization_management", Name: "权限管理", Kind: ComponentKindMenu, ParentId: 1001, Order: 4},
	{Id: 1006, Key: "account_list", Name: "用户列表", Kind: ComponentKindPage, ParentId: 1002, Order: 1, PermissionCodes: []string{"iam.account.list"}},
	{Id: 1007, Key: "application_list", Name: "应用列表", Kind: ComponentKindPage, ParentId: 1003, Order: 1, PermissionCodes: []string{"iam.application.list"}},
	{Id: 1008, Key: "authentication_policy_list", Name: "认证策略", Kind: ComponentKindPage, ParentId: 1004, Order: 1},
	{Id: 1009, Key: "access_credential_list", Name: "访问凭证", Kind: ComponentKindPage, ParentId: 1004, Order: 2, PermissionCodes: []string{"iam.access_credential.list"}},
	{Id: 1010, Key: "permission_list", Name: "权限列表", Kind: ComponentKindPage, ParentId: 1005, Order: 1, PermissionCodes: []string{"iam.permission.list"}},
	{Id: 1011, Key: "role_management", Name: "角色管理", Kind: ComponentKindPage, ParentId: 1005, Order: 2, PermissionCodes: []string{"iam.role.list"}},
	{Id: 1012, Key: "permission_subscription", Name: "权限订阅", Kind: ComponentKindPage, ParentId: 1005, Order: 3, PermissionCodes: []string{"iam.subscription.list"}},
	{Id: 1013, Key: "resource_management", Name: "资源管理", Kind: ComponentKindPage, ParentId: 1005, Order: 4, PermissionCodes: []string{"iam.resource.list"}},
}
