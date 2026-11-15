/** 用户（t_user 语义；username 为唯一登录名，email 可空唯一） */
export interface Account {
  id: string;
  username: string;
  email: string;
  nickname: string;
  /** 用户状态：enabled 启用 / disabled 禁用 */
  status: string;
  roleCodeList: string[];
  /** 可进入的应用 id 集合（仅 GetAccount 详情回填，列表接口不返回） */
  applicationIdList: string[];
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface CreateAccountRequest {
  username: string;
  nickname?: string;
  /** 邮箱可选；传了后端校验格式且唯一 */
  email?: string;
  /** 初始密码必填（≥8 位，bcrypt 摘要落库） */
  password: string;
  /** 用户状态，缺省 enabled */
  status?: string;
  /** 初始角色 id 集合（string 编码），可为空 */
  roleIds?: string[];
}

export interface UpdateAccountRequest {
  nickname?: string;
  email?: string;
  /** 用户状态：enabled / disabled；缺省不修改 */
  status?: string;
  version: number;
}

export interface ResetAccountPasswordRequest {
  id: string;
  newPassword: string;
}

export interface MyProfileApplication {
  id: string;
  code: string;
  name: string;
  status: string;
}

export interface MyProfile {
  id: string;
  username: string;
  nickname: string;
  email: string;
  status: string;
  createdBy: string;
  createTime: string;
  applications: MyProfileApplication[];
}

export interface UpdateAccountRolesRequest {
  addedRoleCodeList: string[];
  removedRoleCodeList: string[];
}
