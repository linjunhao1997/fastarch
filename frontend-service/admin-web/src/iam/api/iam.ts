import type { Page } from '@/shared/api/request';
import { request } from '@/shared/api/request';
import type {
  AccessCredential,
  Account,
  Application,
  ChangeAccessCredentialStatusRequest,
  CreateAccessCredentialRequest,
  CreateAccessCredentialResponse,
  CreateAccountRequest,
  CreateApplicationRequest,
  CreateApplicationResponse,
  CreatePermissionRequest,
  CreateRoleRequest,
  MyProfileApplication,
  ResetAccountPasswordRequest,
  MyProfile,
  Permission,
  PermissionKind,
  ResetAccessCredentialSecretResponse,
  Role,
  UpdateAccessCredentialRequest,
  UpdateAccountRequest,
  UpdateAccountRolesRequest,
  UpdateApplicationRequest,
  UpdatePermissionRequest,
  UpdateRoleRequest,
} from '@/iam/types';

export type {
  Account,
  Application,
  MyProfile,
  Permission,
  PermissionKind,
  Role,
};

export interface GetMyPermissionsResponse {
  /** 当前用户持有的全部权限码（唯一授权词汇；驱动 hasPermission 判定） */
  permissionCodeList: string[];
}


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

export interface ListMyApplicationsResponse {
  applications: MyProfileApplication[];
  /** 所属应用 id（"0" = 未设置），前端默认选中；隐式准入，可能不在 applications 中 */
  homeApplicationId: string;
}

export const authApi = {
  login: (data: { identifier: string; password: string }) =>
    request<PasswordAuthResponse>('/authentication/password-auth', {
      method: 'POST',
      body: data,
      skipToken: true,
    }),
  /** 当前用户可进入的应用（admin 全量启用应用；其他按准入绑定） */
  listMyApplications: () =>
    request<ListMyApplicationsResponse>('/authentication/my-applications'),
  /** 选择当前应用上下文：服务端写 jti→应用映射（切换不重签令牌） */
  selectApplication: (applicationCode: string) =>
    request<void>('/authentication/select-application', {
      method: 'POST',
      body: { applicationCode },
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
  /** 管理员重置用户密码（无需旧密码；新密码 ≥8 位） */
  /** 设置准入应用与所属应用：applicationIds 全量覆盖（不含所属应用） */
  updateApplications: (id: string, applicationIds: string[], homeApplicationId: string) =>
    request<void>(`/account/accounts/${id}/applications`, {
      method: 'PATCH',
      body: { accountId: id, applicationIds, homeApplicationId },
    }),
  resetPassword: (id: string, data: ResetAccountPasswordRequest) =>
    request<void>(`/account/accounts/${id}/reset-password`, { method: 'POST', body: data }),
  delete: (id: string) => request<void>(`/account/accounts/${id}`, { method: 'DELETE' }),
};

export const applicationApi = {
  /** 创建响应一次性返回 secret 明文（服务端仅存 sha256 摘要） */
  create: (data: CreateApplicationRequest) =>
    request<CreateApplicationResponse>('/application/applications', { method: 'POST', body: data }),
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

interface ListAccessCredentialsResponse {
  page: Page;
  accessCredentials: AccessCredential[];
}

/** 组装凭证列表 query：分页 + 可空筛选（snake_case 参数名与后端 proto 字段一致） */
const buildAccessCredentialListUrl = (
  currentPage: number,
  pageSize: number,
  filters: Record<string, string>,
) => {
  const params = new URLSearchParams({
    current_page: String(currentPage),
    page_size: String(pageSize),
  });
  Object.entries(filters).forEach(([key, value]) => {
    const trimmedValue = value.trim();
    if (trimmedValue) params.set(key, trimmedValue);
  });
  return `/authentication/access-credentials?${params.toString()}`;
};

/**
 * 访问凭证（AK/SK）服务（specs/009-ak-sk-auth）。
 * 请求体仅含业务字段，审计字段由服务端注入（宪法 XII）；id/accountId 一律 string。
 */
export const accessCredentialApi = {
  create: (data: CreateAccessCredentialRequest) =>
    request<CreateAccessCredentialResponse>('/authentication/access-credentials', {
      method: 'POST',
      body: data,
    }),
  list: (currentPage = 1, pageSize = 10, filters: Record<string, string> = {}) =>
    request<ListAccessCredentialsResponse>(
      buildAccessCredentialListUrl(currentPage, pageSize, filters),
    ),
  get: (id: string) => request<AccessCredential>(`/authentication/access-credentials/${id}`),
  /** 仅 remark、expireTime 可编辑（契约可改字段全集），version 乐观锁 */
  update: (id: string, data: UpdateAccessCredentialRequest) =>
    request<void>(`/authentication/access-credentials/${id}`, { method: 'PATCH', body: data }),
  changeStatus: (id: string, data: ChangeAccessCredentialStatusRequest) =>
    request<void>(`/authentication/access-credentials/${id}/status`, { method: 'POST', body: data }),
  /** 重置 SK：响应一次性返回新明文 secretKey（FR-008） */
  resetSecret: (id: string) =>
    request<ResetAccessCredentialSecretResponse>(
      `/authentication/access-credentials/${id}/reset-secret`,
      { method: 'POST' },
    ),
  delete: (id: string) =>
    request<void>(`/authentication/access-credentials/${id}`, { method: 'DELETE' }),
};

/** 个人信息：当前登录用户基本信息 + 所属应用列表（US2，页面接入见 US3） */
export const profileApi = {
  get: () => request<MyProfile>('/authentication/my-profile'),
};

/**
 * 菜单 / 字典 API 已迁至 shared/api/menu-dict-api（动态前缀，随当前应用切换）。
 * 保留 re-export 兼容旧引用路径。
 */
export { menuApi, dictApi } from '@/shared/api/menu-dict-api';
