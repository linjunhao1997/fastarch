import type { Page } from "./request";
import { request } from "./request";

export interface Account {
  id: string;
  name: string;
  email: string;
  kindId: string;
}

export interface Application {
  id: string;
  name: string;
  appCode: string;
  description: string;
  enabledStatus: string;
}

interface Permission {
  id: string;
  name: string;
  applicationId: number;
  permissionCode: string;
  enabledStatus: string;
}

interface Role {
  id: string;
  name: string;
  applicationId: number;
  roleCode: string;
  enabledStatus: string;
}

interface Subscription {
  id: number;
  sourceApplicationCode: number;
  targetApplicationCode: number;
  roleCodeList: number;
  permissionCodeList: number;
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
  create: (data: { application_id: number; name: string; code: string; value: string; description: string }) =>
    request<Permission>('/authorization/permissions', { method: 'POST', body: data }),

  get: (id: number) =>
    request<Permission>(`/authorization/permissions/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListPermissionsResponse>(`/authorization/permissions?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { name?: string; enabled_status?: string }) =>
    request<void>('/authorization/permissions', { method: 'PATCH', body: { id, ...data } }),
};

export const roleApi = {
  create: (data: { name: string; role_code: string }) =>
    request<Role>('/authorization/roles', { method: 'POST', body: data }),

  get: (id: number) =>
    request<Role>(`/authorization/roles/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListRolesResponse>(`/authorization/roles?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { name?: string; enabled_status?: string; added_permission_code_list?: number[]; removed_permission_code_list?: number[] }) =>
    request<void>(`/authorization/roles/${id}`, { method: 'PATCH', body: { id, ...data } }),
};

export const subscriptionApi = {
  create: (data: { role_code_list: number; permission_code_list: number }) =>
    request<Subscription>('/authorization/subscriptions', { method: 'POST', body: data }),

  get: (id: number) =>
    request<Subscription>(`/authorization/subscription/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListSubscriptionResponse>(`/authorization/subscriptions?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { role_code_list: number; permission_code_list: number }) =>
    request<void>(`/authorization/subscriptions/${id}`, { method: 'PATCH', body: { id, ...data } }),
};

export const accountApi = {
  create: (data: { name: string; email: string; kind: string }) =>
    request<Account>('/account/accounts', { method: 'POST', body: data }),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListAccountsResponse>(`/account/accounts?current_page=${current_page}&page_size=${page_size}`),

  get: (id: number) =>
    request<Account>(`/account/accounts/${id}`),

  update: (id: number, data: { name?: string; email?: string }) =>
    request<void>(`/account/accounts/${id}`, { method: 'PATCH', body: { id, ...data } }),
};

export const applicationApi = {
  create: (data: { name: string; description?: string }) =>
    request<Application>('/application/applications', { method: 'POST', body: data }),

  get: (id: number) =>
    request<Application>(`/application/applications/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListApplicationsResponse>(`/application/applications?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { name?: string; description?: string; enabled_status?: string }) =>
    request<void>(`/application/applications/${id}`, { method: 'PATCH', body: { id, ...data } }),
};
