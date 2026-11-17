/** 权限类型：菜单 / HTTP 接口 / gRPC 接口（创建后不可变） */
export type PermissionKind = 'menu' | 'http_api' | 'grpc';

export const PERMISSION_KIND_LABELS: Record<PermissionKind, string> = {
  menu: '菜单',
  http_api: 'HTTP 接口',
  grpc: 'gRPC 接口',
};

/** 被保护的具体资源（key 全局唯一，格式随 kind） */
export interface Resource {
  id: string;
  kind: PermissionKind;
  key: string;
  name: string;
  /** 仅 http_api 有值：该 HTTP 路由经网关映射的目标 RPC full method */
  rpcFullMethod?: string;
  enabledStatus: string;
  createTime: string;
  updateTime: string;
  applicationId: string;
  parentId?: string;
  version: number;
  displayOrder: number;
  application?: ApplicationSummary;
}

export interface ApplicationSummary {
  id: string;
  name: string;
  code: string;
  enabledStatus: string;
  description: string;
}

export interface ResourceTreeNode {
  resource: Resource;
  children: ResourceTreeNode[];
}

export interface ListResourceTreeResponse {
  roots: ResourceTreeNode[];
}

export interface CreateResourceRequest {
  applicationId: string;
  parentId?: string;
  kind: PermissionKind;
  key: string;
  name: string;
  rpcFullMethod?: string;
  disabled?: boolean;
  displayOrder?: number;
}

export interface UpdateResourceRequest {
  parentId?: string;
  parentCleared?: boolean;
  name?: string;
  rpcFullMethod?: string;
  disabled?: boolean;
  version: number;
  displayOrder?: number;
}

export interface Permission {
  id: string;
  name: string;
  applicationId: string;
  permissionCode: string;
  enabledStatus: string;
  version: number;
  kind: PermissionKind;
  resourceList: Resource[];
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface CreatePermissionRequest {
  applicationId: string;
  name: string;
  code: string;
  kind: PermissionKind;
  /** int64 以字符串传递，避免 JS 精度丢失 */
  resourceIdList?: string[];
  resourceKeyList?: string[];
  value?: string;
  description?: string;
}

export interface UpdatePermissionRequest {
  name?: string;
  enabledStatus?: string;
  resourceKeyList?: string[];
  /** int64 以字符串传递，避免 JS 精度丢失 */
  resourceIdList?: string[];
  version: number;
}

export interface Role {
  id: string;
  name: string;
  applicationId: string;
  roleCode: string;
  enabledStatus: string;
  permissionCodeList: string[];
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
  application?: ApplicationSummary;
  permissionList?: PermissionSummary[];
}

export interface PermissionSummary {
  id: string;
  name: string;
  permissionCode: string;
  kind: PermissionKind;
  enabledStatus: string;
}

export interface CreateRoleRequest {
  applicationId: string;
  name: string;
  roleCode: string;
}

export interface UpdateRoleRequest {
  name?: string;
  enabledStatus?: string;
  addedPermissionCodeList?: string[];
  removedPermissionCodeList?: string[];
  version: number;
}

export interface Subscription {
  id: string;
  sourceApplicationCode: string;
  targetApplicationCode: string;
  roleCodeList: string[];
  permissionCodeList: string[];
  applicantId: string;
  approverId: string;
  applicantName: string;
  approverName: string;
  status: string;
}
