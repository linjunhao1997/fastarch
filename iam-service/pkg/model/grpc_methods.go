package model

// GRPC_FULL_METHODS 是 gRPC 资源常量清单（specs/005-typed-permission-model T030，
// 数据模型 S1）：idl 四服务的全部 RPC full method，作为 grpc 类型资源来源。
// 服务间直连调用按 rpc:<full_method> 判定；新增 RPC 必须同步本清单。
var GRPC_FULL_METHODS = []string{
	// AuthenticationService
	"/iam.v1.AuthenticationService/PasswordAuth",
	// AccountService
	"/iam.v1.AccountService/CreateAccount",
	"/iam.v1.AccountService/GetAccount",
	"/iam.v1.AccountService/ListAccounts",
	"/iam.v1.AccountService/UpdateAccount",
	"/iam.v1.AccountService/DeleteAccount",
	"/iam.v1.AccountService/UpdateAccountRoles",
	// ApplicationService
	"/iam.v1.ApplicationService/CreateApplication",
	"/iam.v1.ApplicationService/GetApplication",
	"/iam.v1.ApplicationService/ListApplications",
	"/iam.v1.ApplicationService/UpdateApplication",
	"/iam.v1.ApplicationService/DeleteApplication",
	// AuthorizationService
	"/iam.v1.AuthorizationService/CreatePermission",
	"/iam.v1.AuthorizationService/GetPermission",
	"/iam.v1.AuthorizationService/ListPermissions",
	"/iam.v1.AuthorizationService/UpdatePermission",
	"/iam.v1.AuthorizationService/DeletePermission",
	"/iam.v1.AuthorizationService/CreateRole",
	"/iam.v1.AuthorizationService/GetRole",
	"/iam.v1.AuthorizationService/ListRoles",
	"/iam.v1.AuthorizationService/UpdateRole",
	"/iam.v1.AuthorizationService/DeleteRole",
	"/iam.v1.AuthorizationService/CreateSubscription",
	"/iam.v1.AuthorizationService/GetSubscription",
	"/iam.v1.AuthorizationService/UpdateSubscription",
	"/iam.v1.AuthorizationService/ListSubscriptions",
	"/iam.v1.AuthorizationService/CheckPermission",
	"/iam.v1.AuthorizationService/GetMyPermissions",
	"/iam.v1.AuthorizationService/ListResources",
	"/iam.v1.AuthorizationService/ListResourceTree",
	"/iam.v1.AuthorizationService/CreateResource",
	"/iam.v1.AuthorizationService/GetResource",
	"/iam.v1.AuthorizationService/UpdateResource",
	"/iam.v1.AuthorizationService/DeleteResource",
}
