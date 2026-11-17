/**
 * IAM 应用管理专属类型（创建/更新请求、创建响应）。
 * Application 通用视图在 shared/types/application.ts，此处 re-export 保持既有 import 路径。
 */
import type { Application } from '@/shared/types';

export type { Application } from '@/shared/types';

export interface CreateApplicationRequest {
  name: string;
  /** 服务段编码：小写字母开头，仅小写字母/数字/下划线，全局唯一 */
  code: string;
  status?: string;
}

export interface CreateApplicationResponse {
  application: Application;
  /** 一次性明文 secret（服务端仅存 sha256 摘要），关闭后不可再查 */
  secret: string;
}

export interface UpdateApplicationRequest {
  name?: string;
  status?: string;
  version: number;
}
