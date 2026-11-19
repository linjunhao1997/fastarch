export class UnauthorizedError extends Error {
  constructor(message?: string) {
    super(message || '请重新登录');
    this.name = 'UnauthorizedError';
  }
}

export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  headers?: Record<string, string>;
  skipToken?: boolean;
  /**
   * 不带数据范围头。自描述接口（profile / my-permissions / my-menu-tree）按登录态
   * 返回，不按数据归属过滤；若携带陈旧 appCode 会被接入校验拒绝，导致启动死锁。
   */
  skipDataScope?: boolean;
}

export function getCookie(name: string): string | null {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop()?.split(';').shift() || null;
  return null;
}

/**
 * 登录态 token 解析：localStorage 优先（登录页持久化），cookie 兜底。
 * cookie 路径不可依赖：sender 登录响应 Set-Cookie 为 HttpOnly 且同名
 * access_token，document.cookie 的同名写入被浏览器静默忽略（RFC 6265 §5.3）
 * 且不可读；网关亦不转发 Cookie 头。AuthWrapper 与请求层共用本解析。
 */
export function getStoredToken(): string | null {
  return localStorage.getItem('access_token') || getCookie('access_token');
}

function extractErrorMessage(data: unknown): string {
  if (data && typeof data === 'object') {
    const errorData = data as { message?: unknown; error?: unknown; msg?: unknown };
    if (typeof errorData.message === 'string') return errorData.message;
    if (typeof errorData.msg === 'string') return errorData.msg;
    if (typeof errorData.error === 'string') return errorData.error;
  }
  return '';
}

/** 当前数据范围应用 code（由 DataScopeProvider 设置，模块级避免循环依赖）。 */
let currentDataScopeCode = '';
const DATA_SCOPE_STORAGE_KEY = 'data_scope_app_code';

export function setRequestDataScope(code: string): void {
  currentDataScopeCode = code;
}

/** 登录/登出时清空范围：避免上一会话的 appCode 撞进新用户的接入集合。 */
export function clearRequestDataScope(): void {
  currentDataScopeCode = '';
  try {
    localStorage.removeItem(DATA_SCOPE_STORAGE_KEY);
  } catch {
    // ignore
  }
}

function resolveDataScopeCode(): string {
  if (currentDataScopeCode) return currentDataScopeCode;
  try {
    return localStorage.getItem(DATA_SCOPE_STORAGE_KEY) ?? '';
  } catch {
    return '';
  }
}

export async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {}, skipToken = false, skipDataScope = false } = options;

  const token = getStoredToken();

  const defaultHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (!skipToken && token && typeof token === 'string' && token.trim().length > 0) {
    defaultHeaders['Authorization'] = `Bearer ${token}`;
  }
  // 数据范围选择信号：服务端校验 ∈ 接入集合后才作为数据归属。
  // skipToken（登录）/ skipDataScope（自描述接口）不带，避免陈旧 appCode 撞接入集合。
  const scopeCode = skipToken || skipDataScope ? '' : resolveDataScopeCode();
  if (scopeCode) {
    defaultHeaders['X-Fa-App-Code'] = scopeCode;
  }

  const response = await fetch(endpoint, {
    method,
    headers: { ...defaultHeaders, ...headers },
    body: body ? JSON.stringify(body) : undefined,
    credentials: 'include',
  });

  let data: unknown = {};
  try {
    data = await response.json();
  } catch (e) {
    console.warn("[request] Failed to parse JSON response:", e);
  }

  // 错误统一走 HTTP 状态码：grpc-gateway 把 gRPC 错误转成非 2xx，
  // body 形如 {"code":<grpc状态码>,"message":"...","details":[ErrorInfo...]}
  if (!response.ok) {
    const serverMessage = extractErrorMessage(data);

    if (response.status === 401) {
      if (!skipToken) {
        // 统一登录：所有控制台的 401 都回 IAM 登录页，登录后再选控制台。
        // redirect 保留站内原路径，便于登录后回到深链接（由控制台选择页转发）。
        const redirectPath = encodeURIComponent(window.location.pathname + window.location.search);
        window.location.assign(`/login?redirect=${redirectPath}`);
      }
      // 使用服务器返回的错误消息，如果没有则使用默认消息
      throw new UnauthorizedError(serverMessage || undefined);
    }

    const errorMessage = serverMessage || `HTTP ${response.status} Error`;
    throw new ApiError(errorMessage, response.status);
  }

  return data as T;
}

export interface Page {
  total: number;
  currentPage: number;
  pageSize: number;
  totalPages: number;
}