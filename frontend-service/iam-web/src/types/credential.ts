/**
 * 访问凭证（AK/SK）类型（specs/009-ak-sk-auth）。
 *
 * 字段名与后端 grpc-gateway（protojson，默认 camelCase JSON 名）实际下发格式一致，
 * 与既有 types/account.ts 等保持同一惯例；id/accountId 等雪花 ID 一律 string 编码。
 */

/** 凭证状态：与账号状态取值一致 */
export type AccessCredentialStatus = 'enabled' | 'disabled';

/** 列表状态筛选取值：'' 表示不筛 */
export type AccessCredentialStatusFilter = AccessCredentialStatus | '';

/**
 * 访问凭证（不含 secretKey——明文仅出现在创建/重置响应中，FR-002）。
 * 审计字段为服务端注入的只读回传（宪法 XII），任何请求体不得携带。
 */
export interface AccessCredential {
  /** 雪花 ID（string 编码） */
  id: string;
  /** AK，FAK 前缀，全局唯一，创建后不可变 */
  accessKey: string;
  /** 绑定账号 id（string 编码） */
  accountId: string;
  /** 绑定账号名（服务端组装，只读） */
  accountName: string;
  remark: string;
  status: AccessCredentialStatus;
  /** 过期时间（ISO 字符串），null = 永不过期 */
  expireTime: string | null;
  /** 最近使用时间（ISO 字符串），null = 从未使用 */
  lastUsedTime: string | null;
  /** 乐观锁版本号 */
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface CreateAccessCredentialRequest {
  /** 绑定账号 id；账号必须存在且启用（FR-004） */
  accountId: string;
  /** 备注用途，最长 255 */
  remark: string;
  /** 过期时间（ISO 字符串）；不传/null = 永不过期 */
  expireTime?: string | null;
}

export interface CreateAccessCredentialResponse {
  credential: AccessCredential;
  /** 一次性明文 SK，仅此响应返回（FR-002） */
  secretKey: string;
}

export interface UpdateAccessCredentialRequest {
  remark: string;
  /** 过期时间（ISO 字符串）；null = 永不过期 */
  expireTime?: string | null;
  /** 乐观锁版本号（必传） */
  version: number;
}

export interface ChangeAccessCredentialStatusRequest {
  id: string;
  /** 目标状态：enabled 启用 / disabled 禁用 */
  status: AccessCredentialStatus;
}

export interface ResetAccessCredentialSecretResponse {
  credential: AccessCredential;
  /** 一次性明文新 SK，旧 SK 立即失效（FR-008） */
  secretKey: string;
}

/** 列表请求（分页 + 可空筛选，均不传 = 不筛）；由 api 层展开为 query 参数 */
export interface ListAccessCredentialsRequest {
  currentPage: number;
  pageSize: number;
  /** 按绑定账号筛选（string 编码雪花 ID） */
  accountId?: string;
  /** 按 AK 精确/前缀筛选 */
  accessKey?: string;
  /** 按状态筛选，'' = 不筛 */
  status?: AccessCredentialStatusFilter;
}
