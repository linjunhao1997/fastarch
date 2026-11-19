/**
 * sender 业务类型（对齐 sender.v1 proto，grpc-gateway JSON camelCase）。
 * 发送记录 = 落库实体；消息模板 = 应用内可复用内容定义。
 */
import type { ApplicationSummary } from '@/shared/types';

/** 发送记录（MessageRecord） */
export interface MessageRecord {
  id: string;
  recipient: string;
  templateCode: string;
  content: string;
  channel: string;
  /** pending / sent / failed（字典 send_status） */
  sendStatus: string;
  /** 归属应用摘要（服务端配置回填，sender 应用） */
  application?: ApplicationSummary;
  version: string;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface SendMessageRequest {
  recipient: string;
  templateCode?: string;
  content: string;
  channel?: string;
}

export interface ListMessagesParams {
  currentPage?: number;
  pageSize?: number;
  keyword?: string;
  sendStatus?: string;
}

export interface ListMessagesResponse {
  page: import('@/shared/api/request').Page;
  records: MessageRecord[];
}

/** 消息模板（MessageTemplate） */
export interface MessageTemplate {
  id: string;
  code: string;
  name: string;
  content: string;
  remark: string;
  /** 归属应用摘要（服务端配置回填，sender 应用） */
  application?: ApplicationSummary;
  version: string;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface CreateTemplateRequest {
  code: string;
  name: string;
  content: string;
  remark?: string;
}

export interface UpdateTemplateRequest {
  id: string;
  code?: string;
  name?: string;
  content?: string;
  remark?: string;
  version: string;
}

export interface ListTemplatesParams {
  currentPage?: number;
  pageSize?: number;
  keyword?: string;
}

export interface ListTemplatesResponse {
  page: import('@/shared/api/request').Page;
  templates: MessageTemplate[];
}
