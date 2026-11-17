export const ACTIONS = {
  create: '创建',
  edit: '编辑',
  delete: '删除',
  save: '保存',
  confirm: '确定',
  cancel: '取消',
  reset: '重置',
  search: '搜索',
  bindRoles: '绑定角色',
  retry: '重试',
  notOpen: '未开放',
} as const;

export const buildSuccessMessage = (action: string) => `${action}成功`;
export const buildFailureMessage = (action: string) => `${action}失败`;
export const buildConfirmDeleteMessage = (entityType: string, entityName: string) =>
  `确定删除${entityType} ${entityName} 吗？删除后不可恢复。`;
export const FORBIDDEN_MESSAGE = '无权限';
