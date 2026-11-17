/**
 * 集中管理前端路由路径常量
 * 解决硬编码问题，避免路径分散在多个文件中
 */

// ============ 登录 ============
export const LOGIN_PATH = '/login';

// ============ 账号管理 ============
export const ACCOUNT_LIST_PATH = '/account_management/account_list';
export const CREATE_ACCOUNT_PATH = '/account_management/create_account';
export const EDIT_ACCOUNT_PATH = '/account_management/edit_account/:id';

// ============ 应用管理 ============
export const APPLICATION_LIST_PATH = '/application_management/application_list';
export const CREATE_APPLICATION_PATH = '/application_management/create_application';
export const EDIT_APPLICATION_PATH = '/application_management/edit_application/:id';

// ============ 认证管理 ============
export const AUTHENTICATION_POLICY_LIST_PATH = '/authentication_management/authentication_policy_list';
export const CREATE_AUTHENTICATION_POLICY_PATH = '/authentication_management/create_authentication_policy/:type';
export const EDIT_AUTHENTICATION_POLICY_PATH = '/authentication_management/edit_authentication_policy/:type/:id';
export const ACCESS_CREDENTIAL_LIST_PATH = '/authentication_management/access_credential_list';

// ============ 授权管理 ============
export const PERMISSION_LIST_PATH = '/authorization_management/permission_list';
export const CREATE_PERMISSION_PATH = '/authorization_management/permission_create';
export const EDIT_PERMISSION_PATH = '/authorization_management/permission_edit/:id';
export const ROLE_MANAGEMENT_PATH = '/authorization_management/role_management';
export const CREATE_ROLE_PATH = '/authorization_management/role_create';
export const EDIT_ROLE_PATH = '/authorization_management/role_edit/:id';
export const PERMISSION_SUBSCRIPTION_PATH = '/authorization_management/permission_subscription';
export const RESOURCE_MANAGEMENT_PATH = '/authorization_management/resource_management';

/**
 * 路由路径映射表（供 router.tsx 使用）
 */
export const ROUTE_PATHS = {
  login: LOGIN_PATH,
  account: {
    list: ACCOUNT_LIST_PATH,
    create: CREATE_ACCOUNT_PATH,
    edit: EDIT_ACCOUNT_PATH,
  },
  application: {
    list: APPLICATION_LIST_PATH,
    create: CREATE_APPLICATION_PATH,
    edit: EDIT_APPLICATION_PATH,
  },
  authentication: {
    policyList: AUTHENTICATION_POLICY_LIST_PATH,
    createPolicy: CREATE_AUTHENTICATION_POLICY_PATH,
    editPolicy: EDIT_AUTHENTICATION_POLICY_PATH,
    accessCredentialList: ACCESS_CREDENTIAL_LIST_PATH,
  },
  authorization: {
    permissionList: PERMISSION_LIST_PATH,
    createPermission: CREATE_PERMISSION_PATH,
    editPermission: EDIT_PERMISSION_PATH,
    roleManagement: ROLE_MANAGEMENT_PATH,
    createRole: CREATE_ROLE_PATH,
    editRole: EDIT_ROLE_PATH,
    permissionSubscription: PERMISSION_SUBSCRIPTION_PATH,
    resourceManagement: RESOURCE_MANAGEMENT_PATH,
  },
} as const;

/**
 * 解析路由路径，替换参数占位符
 * @example resolvePath('/user/:id', { id: 123 }) => '/user/123'
 */
export function resolvePath(path: string, params?: Record<string, string | number>): string {
  if (!params) return path;
  return Object.entries(params).reduce(
    (result, [key, value]) => result.replace(`:${key}`, String(value)),
    path
  );
}
