/**
 * 权限码常量（与后端 iam-service/pkg/model/permission_codes.go 词汇镜像）。
 * 数据源：GetMyPermissions.permissionCodeList（纯权限码快照）；
 * 判定入口：hasPermission(code)。菜单可见性另由服务端 my-menu-tree 按角色绑定过滤。
 * 新增页面操作入口时，引用后端已登记的码；码不存在 = 快照永不命中（fail-closed）。
 */
export const PERMISSIONS = {
  // 用户模块
  userCreate: 'iam.user.create',
  userGet: 'iam.user.get',
  userList: 'iam.user.list',
  userUpdate: 'iam.user.update',
  userDelete: 'iam.user.delete',
  userAssignRoles: 'iam.user.assign_roles',
  userResetPassword: 'iam.user.reset_password',
  // 应用模块
  applicationCreate: 'iam.application.create',
  applicationGet: 'iam.application.get',
  applicationList: 'iam.application.list',
  applicationUpdate: 'iam.application.update',
  applicationDelete: 'iam.application.delete',
  // 权限模块
  permissionCreate: 'iam.permission.create',
  permissionGet: 'iam.permission.get',
  permissionList: 'iam.permission.list',
  permissionUpdate: 'iam.permission.update',
  permissionDelete: 'iam.permission.delete',
  // 角色模块
  roleCreate: 'iam.role.create',
  roleGet: 'iam.role.get',
  roleList: 'iam.role.list',
  roleUpdate: 'iam.role.update',
  roleDelete: 'iam.role.delete',
  // 访问凭证模块（AK/SK）
  credentialCreate: 'iam.access_credential.create',
  credentialGet: 'iam.access_credential.get',
  credentialList: 'iam.access_credential.list',
  credentialUpdate: 'iam.access_credential.update',
  credentialChangeStatus: 'iam.access_credential.status',
  credentialResetSecret: 'iam.access_credential.reset_secret',
  credentialDelete: 'iam.access_credential.delete',
  // 菜单模块
  menuCreate: 'iam.menu.create',
  menuGet: 'iam.menu.get',
  menuList: 'iam.menu.list',
  menuUpdate: 'iam.menu.update',
  menuDelete: 'iam.menu.delete',
  // 字典模块
  dictCreate: 'iam.dict.create',
  dictGet: 'iam.dict.get',
  dictList: 'iam.dict.list',
  dictUpdate: 'iam.dict.update',
  dictDelete: 'iam.dict.delete',
} as const;
