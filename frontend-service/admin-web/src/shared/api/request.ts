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

  const token = localStorage.getItem("access_token") || getCookie("access_token");

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