package model

// HTTPRoute 是 HTTP 路由表常量（specs/005-typed-permission-model T029）：
// 从 idl/proto/iam/*.proto 的 google.api.http annotations 逐条转录，
// `<METHOD> <path>` 作为资源 key，RpcFullMethod 用于 http 执行域快照键。
// 与 proto 注解镜像：新增 HTTP 接口必须同步本清单。
// PasswordAuth 的登录路由不登记——免认证，无 Permission 保护（research.md D7 S2 例外）。
type HTTPRoute struct {
	Method        string
	Path          string
	RpcFullMethod string
}

// Key 返回资源 key："<METHOD> <path>"。
func (r HTTPRoute) Key() string {
	return r.Method + " " + r.Path
}

var HTTP_ROUTES = []HTTPRoute{
	// AccountService
	{"POST", "/account/accounts", "/iam.v1.AccountService/CreateAccount"},
	{"GET", "/account/accounts/{id}", "/iam.v1.AccountService/GetAccount"},
	{"GET", "/account/accounts", "/iam.v1.AccountService/ListAccounts"},
	{"PATCH", "/account/accounts/{id}", "/iam.v1.AccountService/UpdateAccount"},
	{"DELETE", "/account/accounts/{id}", "/iam.v1.AccountService/DeleteAccount"},
	{"PATCH", "/account/accounts/{account_id}/roles", "/iam.v1.AccountService/UpdateAccountRoles"},
	// ApplicationService
	{"POST", "/application/applications", "/iam.v1.ApplicationService/CreateApplication"},
	{"GET", "/application/applications/{id}", "/iam.v1.ApplicationService/GetApplication"},
	{"GET", "/application/applications", "/iam.v1.ApplicationService/ListApplications"},
	{"PATCH", "/application/applications/{id}", "/iam.v1.ApplicationService/UpdateApplication"},
	{"DELETE", "/application/applications/{id}", "/iam.v1.ApplicationService/DeleteApplication"},
	// AuthorizationService
	{"POST", "/authorization/permissions", "/iam.v1.AuthorizationService/CreatePermission"},
	{"GET", "/authorization/permissions/{id}", "/iam.v1.AuthorizationService/GetPermission"},
	{"GET", "/authorization/permissions", "/iam.v1.AuthorizationService/ListPermissions"},
	{"PATCH", "/authorization/permissions/{id}", "/iam.v1.AuthorizationService/UpdatePermission"},
	{"DELETE", "/authorization/permissions/{id}", "/iam.v1.AuthorizationService/DeletePermission"},
	{"POST", "/authorization/roles", "/iam.v1.AuthorizationService/CreateRole"},
	{"GET", "/authorization/roles/{id}", "/iam.v1.AuthorizationService/GetRole"},
	{"GET", "/authorization/roles", "/iam.v1.AuthorizationService/ListRoles"},
	{"PATCH", "/authorization/roles/{id}", "/iam.v1.AuthorizationService/UpdateRole"},
	{"DELETE", "/authorization/roles/{id}", "/iam.v1.AuthorizationService/DeleteRole"},
	{"POST", "/authorization/subscriptions", "/iam.v1.AuthorizationService/CreateSubscription"},
	{"GET", "/authorization/subscriptions/{id}", "/iam.v1.AuthorizationService/GetSubscription"},
	{"PATCH", "/authorization/subscriptions/{id}", "/iam.v1.AuthorizationService/UpdateSubscription"},
	{"GET", "/authorization/subscriptions", "/iam.v1.AuthorizationService/ListSubscriptions"},
	{"POST", "/authorization/check-permission", "/iam.v1.AuthorizationService/CheckPermission"},
	{"GET", "/authorization/my-permissions", "/iam.v1.AuthorizationService/GetMyPermissions"},
	{"GET", "/authorization/resources", "/iam.v1.AuthorizationService/ListResources"},
	{"GET", "/authorization/resource-tree", "/iam.v1.AuthorizationService/ListResourceTree"},
	{"POST", "/authorization/resources", "/iam.v1.AuthorizationService/CreateResource"},
	{"GET", "/authorization/resources/{id}", "/iam.v1.AuthorizationService/GetResource"},
	{"PUT", "/authorization/resources/{id}", "/iam.v1.AuthorizationService/UpdateResource"},
	{"DELETE", "/authorization/resources/{id}", "/iam.v1.AuthorizationService/DeleteResource"},
	// AccessCredentialService（specs/009-ak-sk-auth）
	{"POST", "/authentication/access-credentials", "/iam.v1.AccessCredentialService/CreateAccessCredential"},
	{"GET", "/authentication/access-credentials", "/iam.v1.AccessCredentialService/ListAccessCredentials"},
	{"GET", "/authentication/access-credentials/{id}", "/iam.v1.AccessCredentialService/GetAccessCredential"},
	{"PATCH", "/authentication/access-credentials/{id}", "/iam.v1.AccessCredentialService/UpdateAccessCredential"},
	{"POST", "/authentication/access-credentials/{id}/status", "/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus"},
	{"POST", "/authentication/access-credentials/{id}/reset-secret", "/iam.v1.AccessCredentialService/ResetAccessCredentialSecret"},
	{"DELETE", "/authentication/access-credentials/{id}", "/iam.v1.AccessCredentialService/DeleteAccessCredential"},
}
