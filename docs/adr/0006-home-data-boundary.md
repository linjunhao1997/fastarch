# 所属应用 = 数据边界，准入应用 = 功能边界

> 模型 A（#8 多应用裁决）的细化：home 单值必填落地到创建契约，账号域按 home 隔离。

用户有两条独立边界。**准入应用**是功能边界：可进入并使用该应用的功能，受权限码控制；菜单/字典/权限管理数据跟当前所选应用走（#8 现状保留）。**所属应用**是数据边界：无论身处哪个应用控制台，只能查看和操作所属应用的数据。创建用户必填 1 个所属应用；所属应用隐式准入、无需放进准入集合、无法取消。admin 旁路两条边界。

## 为什么

#8 落地后准入应用承载"可进入 + 数据可见"双重语义，非 admin 切换到准入应用即可见该应用全部管理数据。2026-11-18 用户裁决把功能与数据拆开：准入代表"可以使用准入应用的功能，只不过受权限控制"，所属应用代表"只可以查看和操作所属应用的数据"。曾提案"非 admin 控制台锁定所属应用"被否——那会使准入应用失去功能入口意义。

## 决策（2026-11-18 用户确认）

- **创建必填 home**：`CreateAccountRequest.home_application_id` 必填且已启用；`application_ids` 为额外准入（服务端自动剔除 home、去重、整批校验存在），同事务写 `t_r_user_application`。
- **账号域数据隔离**：非 admin 的 `ListAccounts` 仅含 home 一致的用户；`GetAccount/UpdateAccount/DeleteAccount/ResetAccountPassword/UpdateAccountRoles/UpdateAccountApplications` 目标 home 不一致一律 `1001062` PermissionDenied；未设置 home（0）fail-closed。
- **非 admin 创建/改 home 锁定**：创建时 home 必须等于操作者 home；`UpdateAccountApplications` 非 admin 不得显式变更目标 home（空串保留旧值放行）。
- **系统调用旁路**：无认证上下文（accountId=0，内部/迁移场景）与 admin 同等不隔离，沿用审计字段 D16 语义。
- **功能边界零改动**：菜单/字典/权限服务、`ListMyApplications/SelectApplication`、登录 home 优先绑定均不变。
- **UI**：创建页所属应用单选必填 + 准入多选可空；非 admin home 锁定（`GetMyProfile.is_admin` 新增只读字段驱动）；列表加所属应用列。

## Considered Options

- **A. 非 admin 控制台锁定所属应用**：数据边界最纯，但准入应用在 admin-web 无功能入口意义，被用户否决。
- **C. 只改创建表单，可见性维持现状**：不满足"只能查看所属应用数据"，否决。
- **B（选定）. 功能跟准入 + 码，数据跟 home**：两条边界正交，#8 成果保留，账号域补隔离。

## Consequences

- 角色当前全局（无应用列），不纳入 home 隔离，仍由权限码门槛；后续若角色按应用自持再议。
- 存量 home=0 用户不回填：非 admin 视角 fail-closed（空列表/拒绝操作），admin 经"设置应用"补设。
- 非 admin 应用管理员只能管理本应用用户；跨应用用户管理归 admin。
- 业务应用自有系统接入时，准入绑定可作为其入口准入凭证（功能边界），数据边界由各宿主按 home 语义自行落地。
