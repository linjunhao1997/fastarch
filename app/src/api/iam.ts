interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: any;
  headers?: Record<string, string>;
  skipToken?: boolean;
}

function getCookie(name: string): string | null {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop()?.split(';').shift() || null;
  return null;
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {}, skipToken = false } = options;

  const token = localStorage.getItem("access_token") || getCookie("access_token");
  console.log("[request] token:", token, typeof token, token?.length, "skipToken:", skipToken);

  const defaultHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (!skipToken && token && typeof token === 'string' && token.trim().length > 0) {
    defaultHeaders['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(endpoint, {
    method,
    headers: { ...defaultHeaders, ...headers },
    body: body ? JSON.stringify(body) : undefined,
    credentials: 'include',
  });

  let data: any = {};
  try {
    data = await response.json();
  } catch (e) {
    console.warn("[request] Failed to parse JSON response:", e);
  }

  console.log("[request] Response status:", response.status, "Response data:", data);

  if (!response.ok) {
    console.log("[request] HTTP error detected, status:", response.status);
    let errorMessage = `HTTP ${response.status} Error`;
    if (data) {
      console.log("[request] Error data exists, checking message field:", data.message);
      if (data.message) {
        errorMessage = data.message;
        console.log("[request] Using data.message as error:", errorMessage);
      } else if (data.error) {
        errorMessage = data.error;
        console.log("[request] Using data.error as error:", errorMessage);
      } else if (data.status && data.status.message) {
        errorMessage = data.status.message;
        console.log("[request] Using data.status.message as error:", errorMessage);
      }
    }
    console.log("[request] Throwing error:", errorMessage);
    throw new Error(errorMessage);
  }

  // 检查业务状态码（格式1: {"status": {"code": "0000", "message": "success"}}）
  if (data.status && data.status.code !== "0000") {
    console.log("[request] Business error (format1):", data.status);
    throw new Error(data.status.message || "Request failed");
  }
  
  // 检查业务状态码（格式2: {"code": 7, "message": "permission denied"}）
  if (data.code && data.code !== 0 && data.code !== "0000") {
    console.log("[request] Business error (format2): code=", data.code, "message=", data.message);
    throw new Error(data.message || "Request failed");
  }

  return data;
}

interface Status {
  code: string;
  data_id: string;
  message: string;
}

interface Page {
  total: number;
  current_page: number;
  page_size: number;
  total_pages: number;
}

interface Permission {
  id: string;
  name: string;
  application_id: number;
  permission_code: string;
  enabled_status: string;
}

interface Role {
  id: string;
  name: string;
  application_id: number;
  role_code: string;
  enabled_status: string;
}

interface Subscription {
  source_application_code: number;
  target_application_code: number;
  role_code_list: number;
  permission_code_list: number;
  applicant_id: number;
  approver_id: number;
  applicant_name: string;
  approver_name: string;
  status: string;
}

export interface Account {
  id: string;
  name: string;
  email: string;
  kind_id: string;
}

export interface Application {
  id: string;
  name: string;
  app_code: string;
  description: string;
  enabled_status: string;
}

interface GetApplicationResponse {
  status: Status;
  application: Application;
}

interface StatusResponse {
  status: Status;
}

interface GetPermissionResponse {
  status: Status;
  permission: Permission;
}

interface ListPermissionsResponse {
  status: Status;
  page: Page;
  permissions: Permission[];
}

interface GetRoleResponse {
  status: Status;
  role: Role;
}

interface ListRolesResponse {
  status: Status;
  page: Page;
  roles: Role[];
}

interface GetSubscriptionResponse {
  status: Status;
  subscription: Subscription;
}

interface ListSubscriptionResponse {
  status: Status;
  page: Page;
  subscriptions: Subscription[];
}

interface ListAccountsResponse {
  status: Status;
  page: Page;
  accounts: Account[];
}

interface GetAccountResponse {
  status: Status;
  account: Account;
}

interface ListApplicationsResponse {
  status: Status;
  page: Page;
  applications: Application[];
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
    request<StatusResponse>('/authorization/permissions', { method: 'POST', body: data }),

  get: (id: number) =>
    request<GetPermissionResponse>(`/authorization/permissions/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListPermissionsResponse>(`/authorization/permissions?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { name?: string; enabled_status?: string }) =>
    request<StatusResponse>('/authorization/permissions', { method: 'PATCH', body: { id, ...data } }),
};

export const roleApi = {
  create: (data: { name: string; role_code: string }) =>
    request<StatusResponse>('/authorization/roles', { method: 'POST', body: data }),

  get: (id: number) =>
    request<GetRoleResponse>(`/authorization/roles/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListRolesResponse>(`/authorization/roles?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { name?: string; enabled_status?: string; added_permission_code_list?: number[]; removed_permission_code_list?: number[] }) =>
    request<StatusResponse>(`/authorization/roles/${id}`, { method: 'PATCH', body: { id, ...data } }),
};

export const subscriptionApi = {
  create: (data: { role_code_list: number; permission_code_list: number }) =>
    request<StatusResponse>('/authorization/subscriptions', { method: 'POST', body: data }),

  get: (id: number) =>
    request<GetSubscriptionResponse>(`/authorization/subscription/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListSubscriptionResponse>(`/authorization/subscriptions?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { role_code_list: number; permission_code_list: number }) =>
    request<StatusResponse>(`/authorization/subscriptions/${id}`, { method: 'PATCH', body: { id, ...data } }),
};

export const accountApi = {
  create: (data: { name: string; email: string; kind: string }) =>
    request<StatusResponse>('/account/accounts', { method: 'POST', body: data }),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListAccountsResponse>(`/account/accounts?current_page=${current_page}&page_size=${page_size}`),

  get: (id: number) =>
    request<GetAccountResponse>(`/account/accounts/${id}`),

  update: (id: number, data: { name?: string; email?: string }) =>
    request<StatusResponse>(`/account/accounts/${id}`, { method: 'PATCH', body: { id, ...data } }),
};

export const applicationApi = {
  create: (data: { name: string; email: string }) =>
    request<StatusResponse>('/application/applications', { method: 'POST', body: data }),

  get: (id: number) =>
    request<GetApplicationResponse>(`/application/applications/${id}`),

  list: (current_page: number = 1, page_size: number = 10) =>
    request<ListApplicationsResponse>(`/application/applications?current_page=${current_page}&page_size=${page_size}`),

  update: (id: number, data: { name?: string; enabled_status?: string }) =>
    request<StatusResponse>(`/application/applications/${id}`, { method: 'PATCH', body: { id, ...data } }),
};