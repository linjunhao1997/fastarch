/**
 * 应用（t_application，多应用裁决 #7-#9）：
 * code 为服务段命名空间（权限码首段，创建后不可改）；
 * app_id 服务端生成；secret 明文仅创建响应一次性返回。
 */
export interface Application {
  id: string;
  appId: string;
  code: string;
  name: string;
  /** enabled 启用 / disabled 禁用 */
  status: string;
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

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
