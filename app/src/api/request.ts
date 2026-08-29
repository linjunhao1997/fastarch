export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: any;
  headers?: Record<string, string>;
  skipToken?: boolean;
}

export function getCookie(name: string): string | null {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop()?.split(';').shift() || null;
  return null;
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

  let data: any = {};
  try {
    data = await response.json();
  } catch (e) {
    console.warn("[request] Failed to parse JSON response:", e);
  }

  // 错误统一走 HTTP 状态码：grpc-gateway 把 gRPC 错误转成非 2xx，
  // body 形如 {"code":<grpc状态码>,"message":"...","details":[ErrorInfo...]}
  if (!response.ok) {
    let errorMessage = `HTTP ${response.status} Error`;
    if (data) {
      if (data.message) {
        errorMessage = data.message;
      } else if (data.error) {
        errorMessage = data.error;
      }
    }
    throw new Error(errorMessage);
  }

  return data as T;
}

export interface Page {
  total: number;
  currentPage: number;
  pageSize: number;
  totalPages: number;
}
