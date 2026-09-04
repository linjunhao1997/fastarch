import type { Page } from "./request";
import { request } from "./request";

export interface Account {
  id: string;
  name: string;
  email: string;
  kind: string;
  roleCodeList: string[];
  version: number;
  /** 只读回传：创建人（账号 id） */
  createdBy: string;
  /** 只读回传：修改人（账号 id） */
  updatedBy: string;
  /** 只读回传：创建时间（RFC3339） */
  createTime: string;
  /** 只读回传：修改时间（RFC3339） */
  updateTime: string;
}

export interface Application {
  id: string;
  name: string;
  appCode: string;
  description: string;
  enabledStatus: string;
}

export interface Permission {
  id: string;
  name: string;
  applicationId: number;
  permissionCode: string;
  enabledStatus: string;
  version: number;
  /** 只读回传：创建人（账号 id） */
  createdBy: string;
  /** 只读回传：修改人（账号 id） */
  updatedBy: string;
  /** 只读回传：创建时间（RFC3339） */
  createTime: string;
  /** 只读回传：修改时间（RFC3339） */
  updateTime: string;
}

export interface Role {
  id: string;
  name: string;
  applicationId: number;
  roleCode: string;
  enabledStatus: string;
  permissionCodeList: string[];
  version: number;
  /** 只读回传：创建人（账号 id） */
  createdBy: string;
  /** 只读回传：修改人（账号 id） */
  updatedBy: string;
  /** 只读回传：创建时间（RFC3339） */
  createTime: string;
  /** 只读回传：修改时间（RFC3339） */
  updateTime: string;
}

export interface Subscription {
  id: string;
  sourceApplicationCode: string;
  targetApplicationCode: string;
  roleCodeList: string[];
  permissionCodeList: string[];
  applicantId: number;
  approverId: number;
  applicantName: string;
  approverName: string;
  status: string;
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

interface ListSubscriptionResponse {
  page: Page;
  subscriptions: Subscription[];
}

interface PasswordAuthResponse {
  accessToken: string;
}

export const authApi = {
  login: (data: { email: string; password: string }) =>
    request<PasswordAuthResponse>('/authentication/password-auth', { method: 'POST', body: data, skipToken: true }),
};

export const permissionApi = {
  create: (data: { applicationId: number; name: string; code: string; value: string; description: string }) =>
    request<Permission>('/authorization/permissions', { method: 'POST', body: data }),

  get: (id: string) =>
    request<Permission>(`/authorization/permissions/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListPermissionsResponse>(`/authorization/permissions?current_page=${current_page}&page_size=${page_size}`),

  update: (id: string, data: { name?: string; enabledStatus?: string; version: number }) =>
    request<void>(`/authorization/permissions/${id}`, { method: 'PATCH', body: data }),

  delete: (id: string) =>
    request<void>(`/authorization/permissions/${id}`, { method: 'DELETE' }),
};

export const roleApi = {
  create: (data: { name: string; roleCode: string }) =>
    request<Role>('/authorization/roles', { method: 'POST', body: data }),

  get: (id: string) =>
    request<Role>(`/authorization/roles/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListRolesResponse>(`/authorization/roles?current_page=${current_page}&page_size=${page_size}`),

  update: (
    id: string,
    data: {
      name?: string;
      enabledStatus?: string;
      addedPermissionCodeList?: string[];
      removedPermissionCodeList?: string[];
      version: number;
    },
  ) =>
    request<void>(`/authorization/roles/${id}`, { method: 'PATCH', body: { id, ...data } }),

  delete: (id: string) =>
    request<void>(`/authorization/roles/${id}`, { method: 'DELETE' }),
};

export const subscriptionApi = {
  create: (data: { roleCodeList: string[]; permissionCodeList: string[] }) =>
    request<Subscription>('/authorization/subscriptions', { method: 'POST', body: data }),

  get: (id: string) =>
    request<Subscription>(`/authorization/subscriptions/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListSubscriptionResponse>(`/authorization/subscriptions?current_page=${current_page}&page_size=${page_size}`),

  update: (id: string, data: { roleCodeList: string[]; permissionCodeList: string[] }) =>
    request<void>(`/authorization/subscriptions/${id}`, { method: 'PATCH', body: { id, ...data } }),
};

export const accountApi = {
  create: (data: { name: string; email: string; kind: string }) =>
    request<Account>('/account/accounts', { method: 'POST', body: data }),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListAccountsResponse>(`/account/accounts?current_page=${current_page}&page_size=${page_size}`),

  get: (id: string) =>
    request<Account>(`/account/accounts/${id}`),

  update: (id: string, data: { name?: string; email?: string; version: number }) =>
    request<void>(`/account/accounts/${id}`, { method: 'PATCH', body: { id, ...data } }),

  updateRoles: (
    accountId: string,
    data: { addedRoleCodeList: string[]; removedRoleCodeList: string[] },
  ) => request<void>(`/account/accounts/${accountId}/roles`, { method: 'PATCH', body: { accountId, ...data } }),

  delete: (id: string) =>
    request<void>(`/account/accounts/${id}`, { method: 'DELETE' }),
};

export const applicationApi = {
  create: (data: { name: string; description?: string }) =>
    request<Application>('/application/applications', { method: 'POST', body: data }),

  get: (id: string) =>
    request<Application>(`/application/applications/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListApplicationsResponse>(`/application/applications?current_page=${current_page}&page_size=${page_size}`),

  update: (id: string, data: { name?: string; description?: string; enabledStatus?: string }) =>
    request<void>(`/application/applications/${id}`, { method: 'PATCH', body: { id, ...data } }),

  delete: (id: string) =>
    request<void>(`/application/applications/${id}`, { method: 'DELETE' }),
};
