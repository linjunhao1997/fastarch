import type { ConsoleKinds } from '@/shared/api/app-context';

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
  userAssignApplications: 'iam.user.assign_applications',
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
  // sender 模块（与 sender-service/pkg/service/method_permissions.go 对齐）
  senderRecordSend: 'sender.record.send',
  senderRecordGet: 'sender.record.get',
  senderRecordList: 'sender.record.list',
  senderRecordDelete: 'sender.record.delete',
  senderTemplateCreate: 'sender.template.create',
  senderTemplateGet: 'sender.template.get',
  senderTemplateList: 'sender.template.list',
  senderTemplateUpdate: 'sender.template.update',
  senderTemplateDelete: 'sender.template.delete',
  // sender 菜单/字典模块（sender 控制台自持，与 IAM 模块码两套词汇）
  senderMenuList: 'sender.menu.list',
  senderMenuCreate: 'sender.menu.create',
  senderMenuUpdate: 'sender.menu.update',
  senderMenuDelete: 'sender.menu.delete',
  senderDictList: 'sender.dict.list',
  senderDictCreate: 'sender.dict.create',
  senderDictUpdate: 'sender.dict.update',
  senderDictDelete: 'sender.dict.delete',
  // flow 模块（与 flow-service/pkg/service/method_permissions.go 对齐）
  flowMemberGet: 'flow.member.get',
  flowMemberSwitch: 'flow.member.switch',
  flowTaskList: 'flow.task.list',
  flowTaskGet: 'flow.task.get',
  flowTaskCreate: 'flow.task.create',
  flowTaskClose: 'flow.task.close',
  flowTaskDiscard: 'flow.task.discard',
  flowQuestionList: 'flow.question.list',
  flowQuestionRaise: 'flow.question.raise',
  flowQuestionEscalate: 'flow.question.escalate',
  flowQuestionAnswer: 'flow.question.answer',
  flowEventList: 'flow.event.list',
  flowMenuList: 'flow.menu.list',
  flowMenuCreate: 'flow.menu.create',
  flowMenuUpdate: 'flow.menu.update',
  flowMenuDelete: 'flow.menu.delete',
  flowDictList: 'flow.dict.list',
  flowDictCreate: 'flow.dict.create',
  flowDictUpdate: 'flow.dict.update',
  flowDictDelete: 'flow.dict.delete',
  // home 模块（与 home-service/pkg/service/method_permissions.go 对齐）
  homePageGet: 'home.page.get',
  homePageUpdate: 'home.page.update',
  homePagePublish: 'home.page.publish',
  homeMenuList: 'home.menu.list',
  homeMenuCreate: 'home.menu.create',
  homeMenuUpdate: 'home.menu.update',
  homeMenuDelete: 'home.menu.delete',
  homeDictList: 'home.dict.list',
  homeDictCreate: 'home.dict.create',
  homeDictUpdate: 'home.dict.update',
  homeDictDelete: 'home.dict.delete',
} as const;

/**
 * 菜单/字典操作码按控制台选择（ADR 0007：单工程双控制台，iam/sender 各自自持模块码）。
 * 共享页面（MenuManagementPage / DictManagementPage）据此在渲染时取当前控制台对应码；
 * getConsole() 未显式缓存时回落 iam 控制台，与 app-context 缺省一致。
 */
export const MENU_MODULE_CODES: Record<ConsoleKinds, { create: string; update: string; delete: string }> = {
  iam: { create: 'iam.menu.create', update: 'iam.menu.update', delete: 'iam.menu.delete' },
  sender: { create: 'sender.menu.create', update: 'sender.menu.update', delete: 'sender.menu.delete' },
  flow: { create: 'flow.menu.create', update: 'flow.menu.update', delete: 'flow.menu.delete' },
  home: { create: 'home.menu.create', update: 'home.menu.update', delete: 'home.menu.delete' },
};

export const DICT_MODULE_CODES: Record<ConsoleKinds, { create: string; update: string; delete: string }> = {
  iam: { create: 'iam.dict.create', update: 'iam.dict.update', delete: 'iam.dict.delete' },
  sender: { create: 'sender.dict.create', update: 'sender.dict.update', delete: 'sender.dict.delete' },
  flow: { create: 'flow.dict.create', update: 'flow.dict.update', delete: 'flow.dict.delete' },
  home: { create: 'home.dict.create', update: 'home.dict.update', delete: 'home.dict.delete' },
};
