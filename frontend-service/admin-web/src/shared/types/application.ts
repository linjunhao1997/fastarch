/**
 * 应用（t_application）的跨应用视图：
 * 所有宿主（iam / sender）都需要读取应用清单与当前应用，因此放 shared。
 * code 为服务段命名空间，创建后不可改；完整管理字段见 iam/types/application。
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

/** 当前登录用户可进入的应用摘要（GetMyProfile.applications） */
export interface MyProfileApplication {
  id: string;
  code: string;
  name: string;
  status: string;
}
