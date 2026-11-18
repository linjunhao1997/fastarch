/**
 * sender 业务 API（sender.v1，路径 /sender/...）。
 * 从共享 request 层发出，自动带 JWT；菜单/字典走 shared/api/menu-dict-api 动态前缀。
 */
import { request } from '@/shared/api/request';
import type {
  CreateTemplateRequest,
  ListMessagesParams,
  ListMessagesResponse,
  ListTemplatesParams,
  ListTemplatesResponse,
  MessageRecord,
  MessageTemplate,
  SendMessageRequest,
  UpdateTemplateRequest,
} from '@/sender/types/sender';

/** sender 独立登录端点：凭据校验委托 IAM，登录即锁定 sender 应用（ADR 0007） */
export const authApi = {
  login: (data: { identifier: string; password: string }) =>
    request<{ accessToken: string }>('/sender/authentication/password-auth', {
      method: 'POST',
      body: data,
      skipToken: true,
    }),
};

const buildListUrl = (
  base: string,
  currentPage = 1,
  pageSize = 10,
  keyword = '',
  extra?: Record<string, string>,
) => {
  const params = new URLSearchParams({
    current_page: String(currentPage),
    page_size: String(pageSize),
  });
  const trimmed = keyword.trim();
  if (trimmed) params.set('keyword', trimmed);
  for (const [key, value] of Object.entries(extra ?? {})) {
    if (value) params.set(key, value);
  }
  return `${base}?${params.toString()}`;
};

export const messageApi = {
  /** 发送消息（模拟投递 pending→sent） */
  send: (data: SendMessageRequest) =>
    request<MessageRecord>('/sender/messages', { method: 'POST', body: data }),
  get: (id: string) => request<MessageRecord>(`/sender/records/${id}`),
  list: ({ currentPage = 1, pageSize = 10, keyword = '', sendStatus }: ListMessagesParams = {}) =>
    request<ListMessagesResponse>(
      buildListUrl('/sender/records', currentPage, pageSize, keyword, {
        send_status: sendStatus ?? '',
      }),
    ),
  delete: (id: string) => request<void>(`/sender/records/${id}`, { method: 'DELETE' }),
};

export const templateApi = {
  create: (data: CreateTemplateRequest) =>
    request<MessageTemplate>('/sender/templates', { method: 'POST', body: data }),
  get: (id: string) => request<MessageTemplate>(`/sender/templates/${id}`),
  list: ({ currentPage = 1, pageSize = 10, keyword = '' }: ListTemplatesParams = {}) =>
    request<ListTemplatesResponse>(buildListUrl('/sender/templates', currentPage, pageSize, keyword)),
  update: (id: string, data: UpdateTemplateRequest) =>
    request<void>(`/sender/templates/${id}`, { method: 'PATCH', body: data }),
  delete: (id: string) => request<void>(`/sender/templates/${id}`, { method: 'DELETE' }),
};
