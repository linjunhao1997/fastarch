import { getConsole } from './app-context';

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

export async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {}, skipToken = false } = options;

  const token = getStoredToken();

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
        const redirectPath = encodeURIComponent(window.location.pathname + window.location.search);
        // 按当前控制台回各自登录页（ADR 0007：IAM 与 sender 各有独立登录页）
        const loginPath = getConsole() === 'sender' ? '/sender/login' : '/login';
        window.location.assign(`${loginPath}?redirect=${redirectPath}`);
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