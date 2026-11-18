# 所属应用 = 数据边界，准入应用 = 功能边界

> 模型 A（#8 多应用裁决）的细化：home 单值必填落地到创建契约，账号域按 home 隔离。

用户有两条独立边界。**准入应用**是功能边界：可进入并使用该应用的功能，受权限码控制；菜单/字典展示配置跟当前所选应用走（#8 现状保留）。**所属应用**是数据边界：无论身处哪个应用控制台，只能查看和操作所属应用的数据。创建用户必填 1 个所属应用；所属应用隐式准入、无需放进准入集合、无法取消。admin 旁路两条边界。

2026-11-19 补充裁决：**权限码管理也纳入数据边界**（与角色同理）。权限码本身仍是全局唯一授权词汇，但其管理列表/详情/增删改按 `t_permission.application_id` 与操作者 home 对齐；非 admin 不可跨 home 查看或维护码。

2026-11-19 补充裁决（绑定与剩余域，审计后用户确认）：**绑定操作纳入数据边界**——`UpdateAccountRoles`/`CreateAccount` 初始角色对每个待绑定角色校验归属 home，且非 admin 一律禁绑内置 admin 角色（登录按角色码计算 IsAdmin，绑定 admin 角色即提权；即使操作者 home 与 admin 角色归属一致也拒绝，`1001068`）；`UpdateRole` 新增码逐个校验归属 home（`1001065`），堵住经挂码自授跨应用能力的路径。**凭证域按所属用户 home 隔离**——AccessCredential 全部 7 接口校验凭证所属用户的 home（`1001062`），List 联表 `t_user` 过滤，home=0 fail-closed。**应用域写操作 admin-only 硬门槛**——Create/Update/Delete 服务层仅放行 admin 或系统调用（`1001066`）：码可经角色绑定自授，码门槛不构成租户边界，须服务层兜底；读接口维持码门槛（非 admin 创建用户/设置准入仍需应用清单）。`DeleteApplication` 补引用校验：角色/权限/菜单/字典归属、用户 home 与准入绑定任一引用即禁删（`1001067`）。菜单/字典维持登录应用上下文（本 ADR 原裁决不变）。

2026-11-19 补充裁决（角色码唯一性口径，用户确认）：**角色码应用内唯一**——`t_role` 全局唯一索引改为复合唯一 `(application_id, code)`（迁移 000014），不同应用可各自定义同名角色（如 ops/admin），消除跨租户码互斥；`CreateRole` 查重带归属应用。**超管判定与角色码解耦**：新增 `is_super_admin` 标志，登录 IsAdmin、禁删、非 admin 禁绑三处均按标志判定；仅内置 admin 角色置 1（服务端控制，创建/更新契约不接受该字段），各应用可自建 'admin' 应用级管理员角色而不获得平台超管。**绑定码按目标用户 home 解析**：`UpdateAccountRoles` 的角色码在目标用户 home 应用内查找（非 admin 目标 home=操作者 home，语义不变；admin/系统调用同样收敛），跨应用授予走 `CreateAccount` role_ids 或先调整目标 home；admin 不再能按码跨应用绑角色（如需，后续契约扩展 role_ids）。

2026-11-18 晚补充裁决：**角色也纳入数据边界**（推翻本 ADR 原文「角色全局、不纳入 home 隔离」一条）。`t_role` 增加 `application_id`，非 admin 仅可见/可操作归属应用与自身 home 一致的角色；角色归属创建时锁定，不提供运行时切换。

## 为什么

#8 落地后准入应用承载"可进入 + 数据可见"双重语义，非 admin 切换到准入应用即可见该应用全部管理数据。2026-11-18 用户裁决把功能与数据拆开：准入代表"可以使用准入应用的功能，只不过受权限控制"，所属应用代表"只可以查看和操作所属应用的数据"。曾提案"非 admin 控制台锁定所属应用"被否——那会使准入应用失去功能入口意义。

## 决策（2026-11-18 用户确认）

- **创建必填 home**：`CreateAccountRequest.home_application_id` 必填且已启用；`application_ids` 为额外准入（服务端自动剔除 home、去重、整批校验存在），同事务写 `t_r_user_application`。
- **账号域数据隔离**：非 admin 的 `ListAccounts` 仅含 home 一致的用户；`GetAccount/UpdateAccount/DeleteAccount/ResetAccountPassword/UpdateAccountRoles/UpdateAccountApplications` 目标 home 不一致一律 `1001062` PermissionDenied；未设置 home（0）fail-closed。
- **非 admin 创建/改 home 锁定**：创建时 home 必须等于操作者 home；`UpdateAccountApplications` 非 admin 不得显式变更目标 home（空串保留旧值放行）。
- **系统调用旁路**：无认证上下文（accountId=0，内部/迁移场景）与 admin 同等不隔离，沿用审计字段 D16 语义。
- **菜单/字典配置零改动**：菜单/字典仍按当前应用上下文读写；登录即锁定应用（ADR 0007），不再有运行时切换应用。
- **角色归属应用（2026-11-18 晚补充）**：`t_role.application_id`（迁移 000013）成为角色数据边界；`ListRoles` 按操作者 home 过滤，`GetRole/UpdateRole/DeleteRole` 越界返回 `1001064`，`CreateRole` 非 admin 强制归属 home（home=0 返回 `1001061`），admin 与系统调用旁路。存量角色按所挂权限的归属应用回填，admin 角色归 IAM。proto 以新字段号恢复 `Role.application_id`/`CreateRoleRequest.application_id`（历史 reserved 号不复用）。
- **权限归属应用（2026-11-19 补充）**：`ListPermissions` 按操作者 home 过滤（home=0 非 admin 返回空列表）；`GetPermission/UpdatePermission/DeletePermission` 越界返回 `1001065`，`CreatePermission` 强制归属 home（home=0 返回 `1001061`）且码服务段须等于 home 应用 code；admin 与系统调用继续按登录应用上下文管理。权限快照与判权（`CheckPermission`）不受影响，仍按码全局判定。
- **绑定隔离（2026-11-19 补充）**：`UpdateAccountRoles`/`CreateAccount` 初始角色逐个校验待绑定角色归属 home（越界 `1001064`），非 admin 禁绑内置 admin 角色（`1001068`）；`UpdateRole` 新增码逐个校验归属 home（越界 `1001065`）。admin 与系统调用旁路。
- **凭证域隔离（2026-11-19 补充）**：AccessCredential 全部接口按凭证所属用户 home 隔离（越界 `1001062`），List 联表 `t_user` 过滤，home=0 fail-closed；admin 与系统调用旁路。
- **应用域写门槛（2026-11-19 补充）**：Create/Update/Delete 服务层 admin-only（`1001066`），读维持码门槛；Delete 校验角色/权限/菜单/字典归属与用户 home/准入引用（`1001067`）。
- **角色码唯一性口径（2026-11-19 补充）**：复合唯一 `(application_id, code)`（000014）；超管按 `is_super_admin` 标志判定（登录/禁删/禁绑），'admin' 码退化为命名约定；`UpdateAccountRoles` 码按目标用户 home 解析，跨应用码不可达（`1001010`）。
- **UI**：创建页所属应用单选必填 + 准入多选可空；非 admin home 锁定（`GetMyProfile.is_admin` 新增只读字段驱动）；列表加所属应用列。

## Considered Options

- **A. 非 admin 控制台锁定所属应用**：数据边界最纯，但准入应用在 admin-web 无功能入口意义，被用户否决。
- **C. 只改创建表单，可见性维持现状**：不满足"只能查看所属应用数据"，否决。
- **B（选定）. 功能跟准入 + 码，数据跟 home**：两条边界正交，#8 成果保留，账号域补隔离。

## Consequences

- 角色按应用自持（000013 起）：角色归属纳入 home 隔离，跨应用管理角色须由 admin 执行。
- 权限码管理按 home 隔离（2026-11-19 起）：非 admin 即使登录到准入应用控制台，也只看到 home 应用的码；跨 home 维护由 admin 执行。权限码作为授权词汇仍全局唯一、判权不按 home 过滤。
- 存量 home=0 用户不回填：非 admin 视角 fail-closed（空列表/拒绝操作），admin 经"设置应用"补设。
- 非 admin 应用管理员只能管理本应用用户；跨应用用户管理归 admin。
- 业务应用自有系统接入时，准入绑定可作为其入口准入凭证（功能边界），数据边界由各宿主按 home 语义自行落地。
- 码门槛不再是唯一租户约束（2026-11-19 起）：绑定、凭证、应用写均有服务层数据边界/admin 硬门槛，非 admin 即使持有任意权限码也无法跨 home 操作或写应用。
- 角色码不再是全局键（2026-11-19 起）：跨应用同名码并存，绑定解析依赖目标 home；应用级 'admin' 角色与平台超管彻底分离，误绑应用级角色不再提权。
