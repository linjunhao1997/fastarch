/**
 * 菜单级权限键（specs/005-typed-permission-model T036）。
 * 值 = router.tsx handle.key，与后端 iam-service/pkg/model/menu_keys.go 的
 * MENU_KEYS 镜像清单一致；数据源为 GetMyPermissions.menuKeyList（menu: 前缀键）。
 * 新增前端菜单需同步后端 MENU_KEYS 常量，否则无法授权。
 */
export const MENU_KEYS = {
  iam: 'iam',
  accountManagement: 'account_management',
  accountList: 'account_list',
  applicationManagement: 'application_management',
  applicationList: 'application_list',
  authenticationManagement: 'authentication_management',
  authenticationPolicyList: 'authentication_policy_list',
  authorizationManagement: 'authorization_management',
  permissionList: 'permission_list',
  roleManagement: 'role_management',
  permissionSubscription: 'permission_subscription',
  resourceManagement: 'resource_management',
} as const;

/**
 * 按钮级权限常量：值为 gRPC full method（specs/005-typed-permission-model T036）。
 * 数据源为 GetMyPermissions.httpFullMethodList（http: 前缀键），由
 * hasHttpPermission 判定，驱动页面内操作入口显隐（宪法 XI 入口即行动）。
 */
export const PERMISSIONS = {
  accountList: '/iam.v1.AccountService/ListAccounts',
  accountCreate: '/iam.v1.AccountService/CreateAccount',
  accountUpdate: '/iam.v1.AccountService/UpdateAccount',
  accountUpdateRoles: '/iam.v1.AccountService/UpdateAccountRoles',
  accountDelete: '/iam.v1.AccountService/DeleteAccount',
  applicationList: '/iam.v1.ApplicationService/ListApplications',
  applicationCreate: '/iam.v1.ApplicationService/CreateApplication',
  applicationUpdate: '/iam.v1.ApplicationService/UpdateApplication',
  applicationDelete: '/iam.v1.ApplicationService/DeleteApplication',
  permissionList: '/iam.v1.AuthorizationService/ListPermissions',
  permissionCreate: '/iam.v1.AuthorizationService/CreatePermission',
  permissionUpdate: '/iam.v1.AuthorizationService/UpdatePermission',
  permissionDelete: '/iam.v1.AuthorizationService/DeletePermission',
  roleList: '/iam.v1.AuthorizationService/ListRoles',
  roleCreate: '/iam.v1.AuthorizationService/CreateRole',
  roleUpdate: '/iam.v1.AuthorizationService/UpdateRole',
  roleDelete: '/iam.v1.AuthorizationService/DeleteRole',
  subscriptionList: '/iam.v1.AuthorizationService/ListSubscriptions',
} as const;
