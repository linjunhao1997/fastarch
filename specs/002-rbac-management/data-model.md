# Data Model: RBAC 基础管理（002）

Phase 1 产出。日期：2026-09-05

三实体 + 两关联表均为**既有结构上的增量**：新增唯一索引与 version 列，实体字段本体不变。

## 实体

### Account（账号）— 既有实体，表 `t_account`（`iam-service/pkg/model/account.go`）

| 字段 | 类型 | 本特性变更 | 说明 |
| --- | --- | --- | --- |
| id | int64 | 不变 | PK，`krand.NewID()` 生成，对外以字符串呈现 |
| email | string | **新增 uniqueIndex** | 唯一（FR-2），登录凭证（FR-4，密码不在本特性范围） |
| name / kind / status / salt / password / ak / sk | — | 不变 | kind：`0`=用户 / `1`=系统 |
| version | int64 | **改用** | 乐观锁已存在，改为客户端携带（D4） |
| create_time / update_time | time.Time | 已有 | 审计时间戳（D16：service 层注入，响应回传） |
| created_by / updated_by | int64 | **新增列** | 操作者账号 id，取 `auth.GetAccountId(ctx)`（D16） |
| RoleList | []*Role | 不变 | many2many → `t_r_account_role`（joinForeignKey: account_id） |

> 契约层 `Account { id, name, email, kind, role_code_list(新增), version, created_by/updated_by/create_time/update_time(新增·v4 只读回传) }`；status/salt/password 等敏感与内部字段不出契约（FR-4）。

### Role（角色）— 既有实体，表 `t_role`（`pkg/model/role.go`）

| 字段 | 类型 | 本特性变更 | 说明 |
| --- | --- | --- | --- |
| id | int64 | 不变 | PK |
| code | string | **新增 uniqueIndex** | 角色编码，全局唯一（FR-6），**创建后不可变**（契约 Update 无此字段，已满足） |
| name / description / application_id / disabled | — | 不变 | 展示字段可修改 |
| version | int64 | **新增列** | 乐观锁（创建置 1，每次更新 +1，D4） |
| create_time / update_time | time.Time | **新增列** | 审计时间戳（D16） |
| created_by / updated_by | int64 | **新增列** | 操作者账号 id（D16） |
| PermissionList | []*Permission | 不变 | many2many → `t_r_role_permission`（joinForeignKey: role_id） |

> 契约层 `Role { id, name, role_code, enabled_status, permission_code_list(新增), version, created_by/updated_by/create_time/update_time(新增·v4 只读回传) }`。

### Permission（权限）— 既有实体，表 `t_permission`（`pkg/model/permission.go`）

| 字段 | 类型 | 本特性变更 | 说明 |
| --- | --- | --- | --- |
| id | int64 | 不变 | PK |
| code | string | **新增 uniqueIndex** | 权限编码，全局唯一（FR-10），**创建后不可变**（US3 场景 4：被引用也不得改码；契约层已满足） |
| name / value / description / application_id / disabled / kind | — | 不变 | name 可修改（US3 场景 3） |
| version | int64 | **新增列** | 乐观锁（D4） |
| create_time / update_time | time.Time | **新增列** | 审计时间戳（D16） |
| created_by / updated_by | int64 | **新增列** | 操作者账号 id（D16） |

> 契约层 `Permission { id, name, application_id, permission_code, enabled_status, version, created_by/updated_by/create_time/update_time(新增·v4 只读回传) }`。

### 审计字段规则（D16，宪法 XII）

- 赋值责任归后端：Create 注入 create_time/update_time/created_by/updated_by；Update 仅注入 update_time/updated_by；created_by/create_time 任何更新路径不可改写
- 操作者取 `auth.GetAccountId(ctx)`（无认证上下文时 0）
- 契约请求消息不含审计字段；响应消息只读回传；前端列表只读展示，创建/编辑表单无审计输入项

## 关联表（引用事实的单一来源，FR-15）

| 表 | 结构 | 本特性变更 | 用途 |
| --- | --- | --- | --- |
| `t_r_account_role` | `RAccountRole { id, account_id, role_id }`（结构体已存在于 account.go） | **纳入迁移清单 + 新增 (account_id, role_id) 联合唯一索引** | 账号-角色绑定（FR-12/14）；Account/Role 删除的引用判定（D3）；绑定幂等兜底（D7） |
| `t_r_role_permission` | GORM many2many 隐式管理（结构体 `RRolePermission` 在 role.go 中被注释） | 不改结构 | 角色-权限关联（FR-7 既有）；Permission 删除的引用判定；Role 删除的级联清理对象 |

> 无外键级联声明；删除的引用约束全部在 service 层判定（D3，同 003 D2 裁决）。

## 关系总览

```text
Account ──< t_r_account_role >── Role ──< t_r_role_permission >── Permission
            （绑定，本特性        （关联，既有能力，       （被角色引用，
              新增管理入口）        本特性修复 bug）          删除需无引用）
```

权限获取路径唯一：Account → Role → Permission（FR-15）；Redis 权限快照（`account:<id>:permissions`，TTL 5min）为派生数据（D6 v2）：绑定/角色权限变更提交后由 service 层主动删除对应账号的键（立即生效）；缓存未命中时由拦截器/CheckPermission 从上表关系读穿重建。

## 状态转换

```text
创建 ──► 存活（version=n）
           │  Update（携带 version=n；成功 version=n+1；不匹配 → 并发冲突拒绝）
           │  Role 额外支持：UpdateRole 增/减权限关联（增量、单事务）
           ▼
     Delete（引用检查）──被引用──► 拒绝："请先解除引用/绑定"
           │ 未引用                     （Permission←角色引用；Role←账号绑定；Account←角色绑定）
           ▼
       物理删除，不可查询
           │  Role 删除例外：事务内级联清除 t_r_role_permission（FR-8）
           │  （Account/Role/Permission 删除均不级联删除其他实体本身）
           ▼
       RowsAffected=0（已被并发删除）──► "对象不存在"明确提示
```

绑定关系（t_r_account_role 行）的状态：不存在 → 绑定（幂等，已绑定视为成功）→ 存在 → 解绑（幂等，不存在视为成功）。绑定变更提交后立即失效受影响账号的权限快照并读穿重建，已登录会话立即生效（D6 v2）；"失效-重建竞态"残余中间态由 TTL 有界。

## 校验规则汇总

| 规则 | 来源 | 实施位置 |
| --- | --- | --- |
| email 全局唯一 | FR-2 | service 查重 `ErrEmailDuplicated` + DB uniqueIndex（D2） |
| 角色编码全局唯一、不可变 | FR-6 | service 查重 `ErrRoleCodeDuplicated` + DB uniqueIndex；Update 契约无 code 字段（D2/D5） |
| 权限编码全局唯一、不可变 | FR-10 / US3-4 | 同上（D2/D5） |
| 更新必须携带匹配 version | FR-18 | 三域 Update 条件更新，冲突返回 `ErrUpdateXxxBusy`（D4） |
| 删除目标必须无引用 | FR-3/8/11 / US5 | service 层关联表计数检查（D3） |
| 角色删除级联清权限关联 | FR-8 | service 层事务内清除（D3） |
| 绑定的角色必须存在 | FR-13 | service 层编码集合校验，缺失即拒绝（D5-②/D7） |
| 绑定唯一（幂等） | FR-14 | service 层前置过滤 + DB (account_id, role_id) 唯一索引（D7） |
| 删除前二次确认 | FR-17 | 前端 `Modal.confirm`（三处列表页） |
| 邮箱格式、长度上限 | Edge Case 3 | proto `buf.validate.email`（既有）+ 前端 Form rules |
| UpdateRole 权限变更不可静默丢弃 | FR-7 / FR-19 | 修复早退 bug + 回归测试（D5-①） |
| 未知编码整体拒绝 | FR-19 | service 层校验（D5-②） |
| 列表翻页稳定 | Edge Case 1 | List 查询 `ORDER BY id ASC`（D11） |
| 绑定/权限变更立即生效 | Edge Case 4 / FR-15 | 提交后主动失效权限快照 + 拦截器读穿重建 + TTL 5min 兜底（D6 v2） |
| Redis 故障不级联、错误日志可诊断 | 宪法 X | 读穿降级、fail-closed 明确报错、Logging/TraceID 拦截器 + service 层结构化日志（D12/D13） |
| 审计字段服务端注入、请求体不含、响应只读回传 | 宪法 XII / D16 | service 层显式赋值（Create 四项 / Update 两项）+ 契约响应消息回传 + 前端只读列 |
