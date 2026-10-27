export interface Account {
  id: string;
  name: string;
  email: string;
  kind: string;
  /** 账号状态：enabled 启用 / disabled 禁用 */
  status: string;
  roleCodeList: string[];
  /** 所属应用 id 列表（string 编码，与 id 惯例一致） */
  applicationIdList: string[];
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface CreateAccountRequest {
  name: string;
  email: string;
  password: string;
  kind: string;
  /** 账号状态，缺省 enabled */
  status?: string;
  /** 归属应用 id 列表，可为空（无归属账号） */
  applicationIdList?: string[];
}

export interface UpdateAccountRequest {
  name?: string;
  email?: string;
  password?: string;
  kind?: string;
  /** 账号状态：enabled / disabled */
  status?: string;
  /**
   * 归属应用 id 列表，全量替换语义：
   * - 不传（undefined）= 不修改归属；
   * - 传（含空数组）= 以该集合全量替换既有归属。
   */
  applicationIdList?: string[];
  version: number;
}

export interface MyProfileApplication {
  id: string;
  code: string;
  name: string;
  status: string;
}

export interface MyProfile {
  id: string;
  name: string;
  email: string;
  kind: string;
  status: string;
  createdBy: string;
  createTime: string;
  applications: MyProfileApplication[];
}

export interface UpdateAccountRolesRequest {
  addedRoleCodeList: string[];
  removedRoleCodeList: string[];
}
