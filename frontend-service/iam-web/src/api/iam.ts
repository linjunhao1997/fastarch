import type { Page } from './request';
import { request } from './request';
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
  CreateDictItemRequest,
  CreateDictTypeRequest,
  CreateMenuRequest,
  CreatePermissionRequest,
  DictItem,
  DictType,
  CreateRoleRequest,
  ListDictItemsResponse,
  ListDictTypesResponse,
  ListMenuTreeResponse,
  Menu,
  MyProfileApplication,
  ResetAccountPasswordRequest,
  UpdateDictItemRequest,
  UpdateDictTypeRequest,
  UpdateMenuRequest,
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
} from '../types';

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
  /** 分配可进入应用：全量覆盖语义（空数组 = 清空准入） */
  updateApplications: (id: string, applicationIds: string[]) =>
    request<void>(`/account/accounts/${id}/applications`, {
      method: 'PATCH',
      body: { accountId: id, applicationIds },
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
 * 菜单（t_menu 语义）：结构与展示由服务端下发，可见性 = 角色菜单绑定（t_r_role_menu）。
 * listMyTree 为当前登录用户的可见菜单树（含停用应用的祖先回补由服务端完成，
 * 管理员返回全量启用菜单）。id/parentId 一律 string。
 */
export const menuApi = {
  listMyTree: () => request<ListMenuTreeResponse>('/menu/my-menu-tree'),
  /** 全量菜单树（管理员视角，含停用） */
  listTree: () => request<ListMenuTreeResponse>('/menu/menu-tree'),
  create: (data: CreateMenuRequest) =>
    request<Menu>('/menu/menus', { method: 'POST', body: data }),
  get: (id: string) => request<Menu>(`/menu/menus/${id}`),
  update: (id: string, data: UpdateMenuRequest) =>
    request<void>(`/menu/menus/${id}`, { method: 'PATCH', body: data }),
  delete: (id: string) => request<void>(`/menu/menus/${id}`, { method: 'DELETE' }),
  /** 角色菜单绑定：全量覆盖语义（空数组 = 清空绑定） */
  updateRoleMenus: (roleId: string, menuIds: string[]) =>
    request<void>(`/menu/roles/${roleId}/menus`, {
      method: 'PATCH',
      body: { roleId, menuIds },
    }),
  /** 角色已绑定菜单 id 集合（编辑角色页回显） */
  listRoleMenus: (roleId: string) =>
    request<{ menuIds: string[] }>(`/menu/roles/${roleId}/menus`),
};

/**
 * 字典（t_dict_type / t_dict_item）：类型 code 唯一；条目按 type_id 归属。
 * id/typeId 一律 string 编码（雪花 ID 惯例）。
 */
export const dictApi = {
  createType: (data: CreateDictTypeRequest) =>
    request<DictType>('/dict/types', { method: 'POST', body: data }),
  getType: (id: string) => request<DictType>(`/dict/types/${id}`),
  updateType: (id: string, data: UpdateDictTypeRequest) =>
    request<void>(`/dict/types/${id}`, { method: 'PATCH', body: data }),
  deleteType: (id: string) => request<void>(`/dict/types/${id}`, { method: 'DELETE' }),
  listTypes: (currentPage = 1, pageSize = 10, keyword = '') =>
    request<ListDictTypesResponse>(
      buildListUrl('/dict/types', currentPage, pageSize, keyword),
    ),
  createItem: (data: CreateDictItemRequest) =>
    request<DictItem>('/dict/items', { method: 'POST', body: data }),
  getItem: (id: string) => request<DictItem>(`/dict/items/${id}`),
  updateItem: (id: string, data: UpdateDictItemRequest) =>
    request<void>(`/dict/items/${id}`, { method: 'PATCH', body: data }),
  deleteItem: (id: string) => request<void>(`/dict/items/${id}`, { method: 'DELETE' }),
  listItems: (typeId: string, currentPage = 1, pageSize = 50, keyword = '') =>
    request<ListDictItemsResponse>(
      `${buildListUrl('/dict/items', currentPage, pageSize, keyword)}&type_id=${typeId}`,
    ),
};
