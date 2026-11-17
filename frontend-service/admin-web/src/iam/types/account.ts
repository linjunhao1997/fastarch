/** 用户（t_user 语义；username 为唯一登录名，email 可空唯一） */
import type { MyProfileApplication } from '@/shared/types';

/** 准入应用摘要（后端回填展示字段，前端直接渲染） */
export interface AccountApplication {
  id: string;
  /** 服务段命名空间（iam/order…） */
  code: string;
  name: string;
  /** enabled / disabled */
  status: string;
}

export interface Account {
  id: string;
  username: string;
  email: string;
  nickname: string;
  /** 用户状态：enabled 启用 / disabled 禁用 */
  status: string;
  roleCodeList: string[];
  /** 可进入的额外应用清单（不含所属应用；所属应用隐式可进入） */
  applications: AccountApplication[];
  /** 所属应用 id（"0" = 未设置），默认进入且隐式可进入 */
  homeApplicationId: string;
  /** 所属应用展示摘要（后端回填，便于列表/详情直接渲染） */
  homeApplication?: AccountApplication;
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
  /** 所属应用 id（string 编码）：必填且已启用（数据边界 ADR 0006） */
  homeApplicationId: string;
  /** 额外准入应用 id 集合，可为空；所属应用隐式准入，服务端自动剔除 */
  applicationIds?: string[];
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

export type { MyProfileApplication } from '@/shared/types';

export interface MyProfile {
  id: string;
  username: string;
  nickname: string;
  email: string;
  status: string;
  createdBy: string;
  createTime: string;
  applications: MyProfileApplication[];
  /** 所属应用 id（"0" = 未设置） */
  homeApplicationId: string;
  /** 是否 admin 角色（服务端派生）：数据边界 UI 锁定依据 */
  isAdmin: boolean;
}

export interface UpdateAccountRolesRequest {
  addedRoleCodeList: string[];
  removedRoleCodeList: string[];
}
