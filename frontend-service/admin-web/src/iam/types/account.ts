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
  /** 已绑角色编码清单（按操作者数据范围过滤，只读回传） */
  roleCodeList: string[];
  /** 已绑角色 id（与 roleCodeList 同源同序，按数据范围过滤；绑定接口按 id 承载） */
  roleIds: string[];
  /** 可接入的应用清单（后端回填展示字段，前端直接渲染） */
  applications: AccountApplication[];
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
  /** 接入应用 id 集合（string 编码）：必填，至少一个已启用应用 */
  applicationIds: string[];
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
  /** 是否 admin 角色（服务端派生） */
  isAdmin: boolean;
}

export interface UpdateAccountRolesRequest {
  addedRoleIds: string[];
  removedRoleIds: string[];
}
