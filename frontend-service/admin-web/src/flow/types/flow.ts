/**
 * flow 业务类型（对齐 flow.v1 proto，grpc-gateway JSON camelCase）。
 * 注意：任务的可开工状态是派生值 state（ready/blocked），库里只存 status（open/done/discarded）。
 */
import type { Page } from '@/shared/api/request';

export interface WorkRole {
  id: string;
  code: string;
  name: string;
  stage: string;
  sort: number;
}

export interface Member {
  id: string;
  accountId: string;
  name: string;
  title: string;
  currentRoleId: string;
  currentRole?: WorkRole;
  roles: WorkRole[];
  readyTasks: number;
  blockedTasks: number;
  openQuestions: number;
}

export interface SwitchRoleRequest {
  roleCode: string;
}

/** 成员卡片（成员与角色页）：不含任务统计 */
export interface MemberBrief {
  id: string;
  accountId: string;
  name: string;
  title: string;
  currentRoleId: string;
  currentRole?: WorkRole;
  roles: WorkRole[];
}

export interface ListMembersParams {
  currentPage?: number;
  pageSize?: number;
  keyword?: string;
}

export interface ListMembersResponse {
  page: Page;
  members: MemberBrief[];
  allRoles: WorkRole[];
  /** 调用者当前角色 code：前端据此决定是否可编辑（boss 才能改） */
  currentRoleCode: string;
}

/** 卡住任务的问题摘要 */
export interface TaskBlocker {
  questionId: string;
  body: string;
  toRoleName: string;
  answerKind: 'decision' | 'evidence' | string;
  createdAt: string;
}

export interface Task {
  id: string;
  roleId: string;
  roleName: string;
  title: string;
  brief: string;
  /** 落库终态：open / done / discarded */
  status: string;
  /** 派生：ready / blocked / done / discarded */
  state: string;
  openQuestions: number;
  blockers: TaskBlocker[];
  createdBy: string;
  /** v1.1.1：派发这条任务时的工作角色 */
  createdRoleId?: string;
  createdRoleName?: string;
  /** v1.2：归属人、父任务、直接下级进度 */
  ownerId?: string;
  ownerName?: string;
  parentTaskId?: string;
  parentTaskTitle?: string;
  childTotal?: number;
  childDone?: number;
  childOpen?: number;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface ListMyTasksParams {
  currentPage?: number;
  pageSize?: number;
  keyword?: string;
  state?: string;
  /** mine=我当前角色的任务；upstream=上游角色任务；created=我发起的；all=全链路（boss/pjm） */
  scope?: 'mine' | 'upstream' | 'created' | 'all';
}

export interface ListTasksResponse {
  page: Page;
  tasks: Task[];
  readyCount: number;
  blockedCount: number;
  roleId: string;
  roleName: string;
}

export interface CreateTaskRequest {
  title: string;
  brief?: string;
  roleCode?: string;
  /** 父任务 id；为空表示顶层任务 */
  parentTaskId?: string;
}

/** 当前角色闭环质量指标（只读，/flow/metrics） */
export interface FlowMetrics {
  roleId: string;
  roleName: string;
  openTasks: number;
  readyTasks: number;
  blockedTasks: number;
  openQuestions: number;
  answeredQuestions: number;
  reopenedQuestions: number;
  avgAnswerMinutes: number;
  p50AnswerMinutes: number;
  reopenRate: number;
  selfAnswerRate: number;
  stalledTasks: number;
}

export interface QuestionOption {
  id: string;
  label: string;
  rec: boolean;
}

export interface Question {
  id: string;
  taskId: string;
  taskTitle: string;
  fromRoleId: string;
  fromRoleName: string;
  toRoleId: string;
  toRoleName: string;
  askedBy: string;
  body: string;
  options: QuestionOption[];
  answer: string;
  picked: string;
  answerKind: 'decision' | 'evidence' | string;
  evidence: string;
  status: 'open' | 'answered' | 'discarded' | string;
  parentQuestionId: string;
  reopenCount: number;
  selfAnswer: boolean;
  answeredBy: string;
  /** v1.1.1：闭环该问题的工作角色（一人多角色时可还原） */
  answeredRoleId?: string;
  answeredRoleName?: string;
  answeredAt?: string;
  createTime?: string;
}

export interface ListMyQuestionsParams {
  currentPage?: number;
  pageSize?: number;
  /** open（默认）/ answered / all（未闭环+已闭环） */
  status?: 'open' | 'answered' | 'all';
  scope?: 'mine' | 'raised';
  /** 只看某个任务下的问题 */
  taskId?: string;
}

export interface ListQuestionsResponse {
  page: Page;
  questions: Question[];
  openCount: number;
}

export interface RaiseQuestionRequest {
  taskId: string;
  toRoleCode: string;
  body: string;
  answerKind?: 'decision' | 'evidence';
  options?: QuestionOption[];
}

export interface AnswerQuestionRequest {
  questionId: string;
  optionId?: string;
  text?: string;
  evidence?: string;
}

export interface FlowEvent {
  id: string;
  at: string;
  actor: string;
  actorName: string;
  action: string;
  entity: string;
  entityId: string;
  detail: string;
}

export interface ListEventsResponse {
  events: FlowEvent[];
}
