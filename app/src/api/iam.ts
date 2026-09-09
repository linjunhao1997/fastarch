import type { Page } from './request';
import { request } from './request';
import type {
  Account,
  Application,
  CreateAccountRequest,
  CreateApplicationRequest,
  CreatePermissionRequest,
  CreateResourceRequest,
  CreateRoleRequest,
  ListResourceTreeResponse,
  MyProfile,
  Permission,
  PermissionKind,
  Resource,
  Role,
  Subscription,
  UpdateAccountRequest,
  UpdateAccountRolesRequest,
  UpdateApplicationRequest,
  UpdatePermissionRequest,
  UpdateResourceRequest,
  UpdateRoleRequest,
} from '../types';

export type {
  Account,
  Application,
  MyProfile,
  Permission,
  PermissionKind,
  Resource,
  Role,
  Subscription,
};

export interface GetMyPermissionsResponse {
  /** 兼容期保留，不再填充真实值 */
  permissionCodeList: string[];
  /** 可见菜单键（前端导航/路由渲染数据源） */
  menuKeyList: string[];
  /** 可调用的 HTTP 接口（映射后的 full method） */
  httpFullMethodList: string[];
  /** 服务间可调用的 gRPC 方法 */
  rpcFullMethodList: string[];
}

interface ListResourcesResponse {
  page: Page;
  resources: Resource[];
}

export const resourceApi = {
  listTree: (applicationId: string) =>
    request<ListResourceTreeResponse>(
      `/authorization/resource-tree?application_id=${applicationId}`,
    ),
  create: (data: CreateResourceRequest) =>
    request<Resource>('/authorization/resources', { method: 'POST', body: data }),
  get: (id: string) => request<Resource>(`/authorization/resources/${id}`),
  update: (id: string, data: UpdateResourceRequest) =>
    request<Resource>(`/authorization/resources/${id}`, { method: 'PUT', body: data }),
  delete: (id: string) => request<void>(`/authorization/resources/${id}`, { method: 'DELETE' }),
};

interface ListAccountsResponse {
  page: Page;
  accounts: Account[];
}

interface ListApplicationsResponse {
  page: Page;
  applications: Application[];
}

interface ListPermissionsResponse {
  page: Page;
  permissions: Permission[];
}

interface ListRolesResponse {
  page: Page;
  roles: Role[];
}

interface ListSubscriptionResponse {
  page: Page;
  subscriptions: Subscription[];
}

interface PasswordAuthResponse {
  accessToken: string;
}

const buildListUrl = (baseUrl: string, currentPage: number, pageSize: number, keyword = '') => {
  const params = new URLSearchParams({
    current_page: String(currentPage),
    page_size: String(pageSize),
  });
  const trimmedKeyword = keyword.trim();
  if (trimmedKeyword) params.set('keyword', trimmedKeyword);
  return `${baseUrl}?${params.toString()}`;
};

export const authApi = {
  login: (data: { email: string; password: string }) =>
    request<PasswordAuthResponse>('/authentication/password-auth', {
      method: 'POST',
      body: data,
      skipToken: true,
    }),
};

export const permissionApi = {
  create: (data: CreatePermissionRequest) =>
    request<Permission>('/authorization/permissions', { method: 'POST', body: data }),
  get: (id: string) => request<Permission>(`/authorization/permissions/${id}`),
  list: (currentPage = 1, pageSize = 10, keyword = '', kind = '') => {
    const baseUrl = buildListUrl('/authorization/permissions', currentPage, pageSize, keyword);
    return request<ListPermissionsResponse>(kind ? `${baseUrl}&kind=${kind}` : baseUrl);
  },
  update: (id: string, data: UpdatePermissionRequest) =>
    request<void>(`/authorization/permissions/${id}`, { method: 'PATCH', body: data }),
  delete: (id: string) =>
    request<void>(`/authorization/permissions/${id}`, { method: 'DELETE' }),
  listResources: (currentPage = 1, pageSize = 100, kind: PermissionKind | '' = '', keyword = '') => {
    const baseUrl = buildListUrl('/authorization/resources', currentPage, pageSize, keyword);
    return request<ListResourcesResponse>(kind ? `${baseUrl}&kind=${kind}` : baseUrl);
  },
};

export const roleApi = {
  create: (data: CreateRoleRequest) =>
    request<Role>('/authorization/roles', { method: 'POST', body: data }),
  get: (id: string) => request<Role>(`/authorization/roles/${id}`),
  list: (currentPage = 1, pageSize = 10, keyword = '') =>
    request<ListRolesResponse>(
      buildListUrl('/authorization/roles', currentPage, pageSize, keyword),
    ),
  update: (id: string, data: UpdateRoleRequest) =>
    request<void>(`/authorization/roles/${id}`, { method: 'PATCH', body: { id, ...data } }),
  delete: (id: string) => request<void>(`/authorization/roles/${id}`, { method: 'DELETE' }),
};

export const subscriptionApi = {
  get: (id: string) => request<Subscription>(`/authorization/subscriptions/${id}`),
  list: (currentPage = 1, pageSize = 10, keyword = '') =>
    request<ListSubscriptionResponse>(
      buildListUrl('/authorization/subscriptions', currentPage, pageSize, keyword),
    ),
};

export const accountApi = {
  create: (data: CreateAccountRequest) =>
    request<Account>('/account/accounts', { method: 'POST', body: data }),
  get: (id: string) => request<Account>(`/account/accounts/${id}`),
  list: (currentPage = 1, pageSize = 10, keyword = '') =>
    request<ListAccountsResponse>(
      buildListUrl('/account/accounts', currentPage, pageSize, keyword),
    ),
  update: (id: string, data: UpdateAccountRequest) =>
    request<void>(`/account/accounts/${id}`, { method: 'PATCH', body: data }),
  updateRoles: (id: string, data: UpdateAccountRolesRequest) =>
    request<void>(`/account/accounts/${id}/roles`, { method: 'PATCH', body: data }),
  delete: (id: string) => request<void>(`/account/accounts/${id}`, { method: 'DELETE' }),
};

export const applicationApi = {
  create: (data: CreateApplicationRequest) =>
    request<Application>('/application/applications', { method: 'POST', body: data }),
  get: (id: string) => request<Application>(`/application/applications/${id}`),
  list: (currentPage = 1, pageSize = 10, keyword = '') =>
    request<ListApplicationsResponse>(
      buildListUrl('/application/applications', currentPage, pageSize, keyword),
    ),
  update: (id: string, data: UpdateApplicationRequest) =>
    request<void>(`/application/applications/${id}`, { method: 'PATCH', body: data }),
  delete: (id: string) =>
    request<void>(`/application/applications/${id}`, { method: 'DELETE' }),
};

export const getMyPermissions = () => request<GetMyPermissionsResponse>('/authorization/my-permissions');

/** 个人信息：当前登录用户基本信息 + 所属应用列表（US2，页面接入见 US3） */
export const profileApi = {
  get: () => request<MyProfile>('/authentication/my-profile'),
};

/** 左侧菜单树：按当前应用 + 登录态权限过滤（服务端裁决，US3） */
export const menuApi = {
  listMyTree: (applicationId: string) =>
    request<ListResourceTreeResponse>(`/authorization/my-menu-tree?application_id=${applicationId}`),
};
