/**
 * flow 业务 API（flow.v1，路径 /flow/...）。
 * 查询参数用 proto 字段名（snake_case），请求体用 protojson（camelCase）。
 */
import { request } from '@/shared/api/request';
import type {
  AnswerQuestionRequest,
  Artifact,
  CreateTaskRequest,
  ListEventsResponse,
  ListMembersParams,
  ListMembersResponse,
  ListMyQuestionsParams,
  ListMyTasksParams,
  ListQuestionsResponse,
  ListTasksResponse,
  Member,
  MemberBrief,
  Question,
  RaiseQuestionRequest,
  Task,
  WorkPackage,
} from '@/flow/types/flow';

const query = (base: string, params: Record<string, string | number | undefined>) => {
  const sp = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v === undefined || v === '' || v === null) return;
    sp.set(k, String(v));
  });
  const s = sp.toString();
  return s ? `${base}?${s}` : base;
};

export const memberApi = {
  me: () => request<Member>('/flow/me'),
  listRoles: () => request<{ roles: Member['roles'] }>('/flow/me/roles'),
  switchRole: (roleCode: string) =>
    request<Member>('/flow/me/role', { method: 'POST', body: { roleCode } }),
  /** 成员与角色（仅当前角色为 boss 可见/可改） */
  listMembers: ({ currentPage = 1, pageSize = 20, keyword = '' }: ListMembersParams = {}) =>
    request<ListMembersResponse>(
      query('/flow/members', {
        current_page: currentPage,
        page_size: pageSize,
        keyword,
      }),
    ),
  assignRoles: (memberId: string, roleCodes: string[]) =>
    request<MemberBrief>(`/flow/members/${memberId}/roles`, {
      method: 'POST',
      body: { id: memberId, roleCodes },
    }),
};

export const taskApi = {
  list: ({ currentPage = 1, pageSize = 10, keyword = '', state = '', scope = 'mine' }: ListMyTasksParams = {}) =>
    request<ListTasksResponse>(
      query('/flow/tasks', {
        current_page: currentPage,
        page_size: pageSize,
        keyword,
        state,
        scope,
      }),
    ),
  get: (id: string) => request<Task>(`/flow/tasks/${id}`),
  create: (data: CreateTaskRequest) => request<Task>('/flow/tasks', { method: 'POST', body: data }),
  close: (id: string) => request<void>(`/flow/tasks/${id}/close`, { method: 'POST' }),
  discard: (id: string) => request<void>(`/flow/tasks/${id}/discard`, { method: 'POST' }),
  start: (id: string) => request<WorkPackage>(`/flow/tasks/${id}/start`, { method: 'POST' }),
  submit: (id: string, data: { artifactType: string; title?: string; contentMd: string }) =>
    request<Artifact>(`/flow/tasks/${id}/submit`, { method: 'POST', body: data }),
  finish: (id: string, data: { artifactType?: string; title?: string; contentMd: string }) =>
    request<void>(`/flow/tasks/${id}/finish`, { method: 'POST', body: data }),
  accept: (id: string) => request<void>(`/flow/tasks/${id}/accept`, { method: 'POST' }),
  workPackage: (id: string) => request<WorkPackage>(`/flow/tasks/${id}/work-package`),
  artifacts: (id: string) => request<{ artifacts: Artifact[] }>(`/flow/tasks/${id}/artifacts`),
};

export const questionApi = {
  /** scope=mine 待我（当前角色）回答；raised 我提出的 */
  list: ({
    currentPage = 1,
    pageSize = 20,
    status = 'open',
    scope = 'mine',
    taskId,
  }: ListMyQuestionsParams = {}) =>
    request<ListQuestionsResponse>(
      query('/flow/questions', {
        current_page: currentPage,
        page_size: pageSize,
        status,
        scope,
        task_id: taskId,
      }),
    ),
  raise: (data: RaiseQuestionRequest) => request<Question>('/flow/questions', { method: 'POST', body: data }),
  escalate: (id: string, data: Omit<RaiseQuestionRequest, 'taskId'>) =>
    request<Question>(`/flow/questions/${id}/escalate`, { method: 'POST', body: data }),
  answer: (data: AnswerQuestionRequest) =>
    request<Question>(`/flow/questions/${data.questionId}/answer`, {
      method: 'POST',
      body: { optionId: data.optionId, text: data.text, evidence: data.evidence },
    }),
  /** 重开已闭环问题：仅原提问人或 boss 角色可调用 */
  reopen: (id: string, reason: string) =>
    request<Question>(`/flow/questions/${id}/reopen`, { method: 'POST', body: { reason } }),
};

export const eventApi = {
  list: (params: { limit?: number; entity?: string; entityId?: string } = {}) =>
    request<ListEventsResponse>(
      query('/flow/events', {
        limit: params.limit ?? 20,
        entity: params.entity,
        entity_id: params.entityId,
      }),
    ),
};
