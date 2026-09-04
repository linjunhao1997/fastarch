# Research: RBAC 基础管理（002）

Phase 0 产出。逐项消解 Technical Context 与 spec 中的决策点。日期：2026-09-05（v2：按宪法 v1.2.0 原则 X 复核，重写 D6、新增 D12/D13）

> 现状勘察结论（本文件决策的依据）：Account/Role/Permission 均无 Delete RPC；邮箱/角色编码/权限编码无唯一性检查也无 DB 唯一索引；`t_r_account_role` 关联表与 GORM 关联已存在但无任何 API 与 UI；`UpdateRole` 存在早退 bug（仅传权限变更列表时被静默跳过）；登录时权限快照写入 Redis（TTL 30 分钟）且无失效机制；auth 拦截器缓存未命中时直接返回原始 Redis 错误（快照过期或 Redis 重启即全体非 admin 请求鉴权失败，无回源、无结构化日志）；Account 已有 version 乐观锁（但 version 未随请求携带），Role/Permission 无并发防护。

---

## D1: 契约扩展方式与绑定 RPC 形态

**Decision**: 在 `idl/proto/iam/account.proto` 与 `authorization.proto` 两个**既有文件**内增量扩展，不新建契约文件：

```proto
// account.proto 新增
rpc DeleteAccount(DeleteAccountRequest) returns (google.protobuf.Empty)
  // DELETE /account/accounts/{id}
rpc UpdateAccountRoles(UpdateAccountRolesRequest) returns (google.protobuf.Empty)
  // PATCH /account/accounts/{account_id}/roles，body: "*"
// UpdateAccountRolesRequest { account_id, repeated added_role_code_list, repeated removed_role_code_list }
// Account 消息新增 repeated string role_code_list（Get/List 时经 Preload RoleList 回填）
// UpdateAccountRequest 新增 int64 version（必填，见 D4）
```

authorization.proto 对称新增 `DeleteRole`（DELETE `/authorization/roles/{id}`）、`DeletePermission`（DELETE `/authorization/permissions/{id}`）；`Role` 消息新增 `repeated string permission_code_list`（GetRole 回填，供编辑页回显）；两个 Update 请求新增 `version`。

**Rationale**: 绑定增量形态（added/removed 双列表）与既有 `UpdateRoleRequest.added_permission_code_list / removed_permission_code_list` 完全对称，前端可用同一套 delta 提交逻辑；单 RPC 同时完成绑定与解绑，覆盖 US4 场景与 FR-12 的批量要求。`Account`/`Role` 回带关联编码列表满足"绑定关系可查询"（FR-12），避免为读操作另开 RPC。

**Alternatives considered**:
- 独立的 BindRole / UnbindRole 两个 RPC：语义更窄，批量"一次编辑同时增删"（US2 场景 5）需两次调用、无原子性，否决。
- 新建 `rbac.proto` 承载绑定关系：绑定是账号子资源，归属 account 域更自然；另立文件破坏"`<域>/<资源>`"组织。
- 绑定返回绑定后全量角色列表：调用方可再查 Get，返回 `Empty` 与 Update 惯例一致。

---

## D2: 唯一性约束（FR-2 / FR-6 / FR-10）

**Decision**: 双层防护，模式同 001 D3——service 层前置查重（Create 与"改到唯一字段"的 Update 均查）返回可理解业务错误（`ErrEmailDuplicated` / `ErrRoleCodeDuplicated` / `ErrPermissionCodeDuplicated`，gRPC `AlreadyExists`）；DB 层补唯一索引兜底并发：`t_account.email`、`t_role.code`、`t_permission.code` 各加 `uniqueIndex`。唯一性范围为**全局**（spec 未按应用维度限定）。同时为 `t_r_account_role` 补 `(account_id, role_id)` 联合唯一索引（支撑 FR-14，见 D7）。

**Rationale**: 现状三处唯一性完全缺位（无查重、无索引，重复数据可直接产生），是 FR-2/6/10 的直接缺口；联合索引同时是绑定幂等的兜底。

**风险与前置条件**: AutoMigrate 创建唯一索引在存量重复数据时会失败——实施前须清理存量重复（quickstart 步骤 0 提供检查 SQL）。

**Alternatives considered**:
- 仅 service 查重：并发竞态下可产生重复数据，违反 FR 强制语义。
- 仅 DB 索引：约束冲突错误对用户不可理解（FR-19）。
- 唯一性按 application_id 分域：spec 表述为全局唯一且编码是跨系统引用纽带（Assumptions），分域反而引入歧义。

---

## D3: 删除语义与引用约束（FR-3 / FR-8 / FR-11 / US5）

**Decision**: 物理删除；引用约束采用"**拒绝删除**"策略（spec Assumptions 明确），模式同 003 D2/D4：

| 对象 | 删除前置检查（service 层，拒绝则返回对应业务错误） | 删除动作 |
| --- | --- | --- |
| Permission | `t_r_role_permission` 中存在该 permission_id → `ErrPermissionReferenced` | 删 `t_permission` 行 |
| Role | `t_r_account_role` 中存在该 role_id → `ErrRoleReferenced`（须先解绑账号） | **事务内**删 `t_role` 行 + 级联清除 `t_r_role_permission` 关联（FR-8） |
| Account | `t_r_account_role` 中存在该 account_id → `ErrAccountReferenced`（须先解绑角色，FR-3） | 删 `t_account` 行 |

三者均在 Delete 后检查 `RowsAffected`，为 0 返回"记录不存在"（复用 contrib GORM 错误映射），覆盖 US5 场景 6 的并发删除竞态。

**Rationale**: 引用判定以关联表计数为准（关联表即引用事实的单一来源，FR-15 精神）；拒绝式策略与"先解除引用再删除"的 spec 假设一致；Role 级联清除权限关联在事务内保证不留孤儿关联。

**Alternatives considered**:
- 级联删除（删角色连带解绑账号）：账号绑定关系属显式管理数据，静默消失违反"用户可理解"与最小意外原则，spec 也明确仅允许级联清权限关联。
- 软删除：同 003 D1 裁决，成本不成比例。

---

## D4: 并发修改检测（FR-18、Edge Case）

**Decision**: 统一乐观锁。**Account**：`UpdateAccountRequest` 新增 `version` 必填字段，service 改用客户端携带的 version 作为 `WHERE` 条件（现状是服务端 First 后用当前值，仅能防请求内竞态、防不了两个管理员的编辑窗口竞态，不满足 FR-18）；**Role / Permission**：模型新增 `version` 列（创建置 1，每次更新 +1），Update 请求携带 `version`。三者 RowsAffected==0 时区分语义：行不存在 → NotFound；行存在但 version 不匹配 → `ErrUpdateRoleBusy` / `ErrUpdatePermissionBusy`（Account 沿用既有 `ErrUpdateAccountBusy`）。绑定操作（UpdateAccountRoles）为集合增量语义，不做乐观锁（见 D7）。

**Rationale**: FR-18 要求"后保存方不得无痕迹覆盖先保存方"，必须以客户端编辑窗口起点为准做条件更新；三域同构降低认知成本（与 001 策略域 version 方案一致，全仓库乐观锁形态统一）。Account 既有 `version` 列与错误码直接复用。

**Alternatives considered**:
- 维持服务端 First→Update 现状：只覆盖毫秒级窗口，两个管理员先后保存仍会静默覆盖，违反 FR-18。
- 全表时间戳比对：同 001 D4 裁决，精度不足。

---

## D5: 既有缺陷修复（纳入本特性范围，FR-7 / FR-19）

**Decision**: 修复两处既有静默失败，作为 FR-7/FR-19 的前置条件：
1. **`UpdateRole` 早退 bug**：`len(updates) == 0` 时的早退 return 位于权限列表处理之前，导致"仅提交权限增减、不改名称/状态"时权限变更被静默丢弃——调整代码顺序，权限变更始终处理（补回归测试用例）。
2. **未知编码静默跳过**：UpdateRole 现将查不到的权限编码静默忽略；UpdateAccountRoles 对不存在角色码同病。改为：提交的编码集合中任一不存在 → 整体拒绝并指明缺失编码（`ErrPermissionNotFound` / `ErrRoleNotFound`，InvalidArgument）。

**Rationale**: FR-7 的核心验收（US2 场景 3/5）直接踩中 bug ①；静默跳过违反宪法 IV"不得静默失败"与 FR-19，且 US2 场景 3 要求"保存后权限集合包含新增项"——静默丢弃即验收失败。修复收敛在本特性既有文件内，无范围膨胀。

**Alternatives considered**:
- 忽略未知编码但返回警告字段：调用方大概率不看警告，且契约复杂化；显式拒绝语义最清晰。
- 不修 bug、前端保证总是携带 name：以调用方约定掩盖服务端缺陷，违反宪法 VI（契约与实现各自完整）。

---

## D6: 绑定/角色权限变更的权限生效时机（Edge Case、FR-15）— v2 修订

**Decision**: **立即生效**。
> **修订说明**：v1 曾裁决"下次登录生效"，与 spec Clarifications 的明确答复——"立即生效：权限校验实时读取绑定关系（可加短 TTL 缓存），变更后无需重新登录"——直接冲突，属裁决错误，本版予以纠正并同步修订 plan/data-model/quickstart。

实现为"**主动失效 + 读穿回源 + 短 TTL 兜底**"三件套：

1. **主动失效**：变更事务提交成功后，service 层立即删除受影响账号的权限快照键 `account:<id>:permissions`：
   - `UpdateAccountRoles`：目标账号 id；
   - `UpdateRole`（权限集合增减或禁用状态变更）：查询 `t_r_account_role` 中绑定该角色的全部账号 id，逐一删除；
   - `DeleteRole`（级联清除权限关联）同 `UpdateRole` 处理。
2. **读穿回源**：auth 拦截器与 `CheckPermission` 的缓存未命中（`redis.Nil`）不再直接报错，改为从 DB 重建快照（查询账号绑定角色的权限并集，写回 Redis，TTL 5 分钟）后继续校验。此修复同时消除现状"快照过期/Redis 重启即全体非 admin 请求鉴权失败"的可用性缺陷（宪法 X：单点故障不得级联崩溃）。
3. **短 TTL 兜底**：快照 TTL 由 30 分钟缩短为 5 分钟（constant 统一定义），作为"失效-重建竞态"的时间上界：早于事务提交开始的并发登录重建可能把旧快照写回，残余中间态被 TTL 有界（spec 明确允许"短 TTL 缓存"）；管理操作为低频场景，秒级窗口可接受。

**Rationale**: spec Clarifications 与 Dependencies 均为强制语义（"MUST 立即生效"），不存在二选一；主动失效覆盖绝大多数变更（生效延迟秒级内）；读穿回源让 Redis miss/重启/清空都不再阻断鉴权（读 DB 结果，fail-closed 语义不变——回源查出的权限并集照常判定）；短 TTL 兜住并发重建竞态的残余风险。

**Alternatives considered**:
- 保持下次登录生效（v1 裁决）：违反 spec 强制语义，否决。
- 缓存版本号/延迟双删消除竞态：复杂度与出错面显著上升，本特性数据量级下短 TTL 已满足"立即生效（可加短 TTL 缓存）"的验收语义，否决。
- 拦截器每次实时查 DB、彻底弃用缓存：语义最简，但常态鉴权延迟与 DB 压力上升，且放弃既有 Redis 设施；读穿模式在两者间取得平衡，否决。

---

## D7: 绑定幂等与原子性（FR-13 / FR-14）

**Decision**: `UpdateAccountRoles` 单事务内执行：① 校验 `added_role_code_list` 与 `removed_role_code_list` 中全部编码真实存在（FR-13，缺失即拒绝，见 D5-②）；② 绑定前先查既有绑定，过滤已绑定的编码（重复绑定幂等——结果视为成功，FR-14；DB 联合唯一索引兜底并发）；③ 解绑对不存在的绑定关系保持幂等（`Delete` 影响行数为 0 不报错）；④ 增删在同一事务中提交（US2 场景 5 的"两类变更同时生效"语义对绑定同样适用）。若增删后绑定为空集，结果合法（账号可以没有任何角色）。

**Rationale**: 幂等语义（FR-14 明确允许"视为成功或给出已绑定提示"）取"视为成功"，前端无需区分；事务保证批量提交的原子性，避免半成功中间态（宪法 IV）。

**Alternatives considered**:
- 重复绑定报错（AlreadyExists）：FR-14 允许但增加前端分支，"视为成功"更简单且语义无损。
- 以 ` Associations().Append/Delete` 直接操作：Append 遇重复会插入重复行（现状无索引），必须前置过滤 + 唯一索引双保险。

---

## D8: 服务层测试（宪法 VII）

**Decision**: 沿用 003/001 建立的 SQLite 内存库模式，新增 `account_service_test.go` 与 `authorization_service_test.go`，覆盖——
- **Account**：邮箱重复拒绝（创建/修改）、UpdateAccount version 冲突拒绝、DeleteAccount 被绑定拒绝 / 未绑定成功 / 并发删除 NotFound、绑定后 role_code_list 回填正确；
- **Authorization**：角色编码/权限编码重复拒绝、UpdateRole 早退 bug 回归（仅传权限列表时变更生效）、未知权限码拒绝、增删同时生效、DeletePermission 被引用拒绝 / DeleteRole 被绑定拒绝 / 删除角色级联清关联、乐观锁冲突、GetRole 回填 permission_code_list。

**Rationale**: 本特性全部新增路径都是"拒绝类"分支（唯一性/引用/并发），恰是回归风险最高、必须自动化锁定的部分；bug ① 修复必须有用例防止复发。

> **v2 补充（宪法 X）**：测试须额外覆盖缓存失效路径——引入 `miniredis`（dev 依赖）构造 rdb：① `UpdateAccountRoles` 成功后 `account:<id>:permissions` 键被删除；② `UpdateRole` 权限集合变更后所有绑定账号的键被删除；③ 读穿回源（构造空快照，校验后快照按 DB 权限并集重建）；④ 绑定幂等（重复提交无重复行）。拦截器读穿回源逻辑抽取为可注入测试的独立函数后由 service/auth 域测试覆盖。

**Alternatives considered**: 同 003 D6（go-sqlmock / MySQL 容器 / 不写测试，均否决）。

---

## D9: 认证与权限控制（FR-16）

**Decision**: 无需新增代码。Auth 拦截器按 `<appCode>:<fullMethodName>` 校验，新增 RPC 方法名自动纳入（不在白名单）；admin 角色旁路逻辑不变。

**Rationale**: 同 001 D9 / 003 D7。注意：非 admin 账号须由授权数据显式授予对应权限码方可操作（运营数据配置，非代码）。

**Alternatives considered**: 无。

---

## D10: 前端接入

**Decision**: `app/src/api/iam.ts` 扩展：`accountApi` 增 `delete(id)` 与 `updateRoles(accountId, {addedRoleCodeList, removedRoleCodeList})`；`roleApi` / `permissionApi` 增 `delete(id)`；各 `update` 增携带 `version`（来源为 get 回填数据）。页面改动：
- **AccountListPage**：行操作增"删除"（`Modal.confirm` 二次确认，FR-3/FR-17）与"绑定角色"（Modal 内 `Select mode="multiple"`，选项来自 `roleApi.list`，初值 = 当前 `role_code_list`，确认时前端 diff 出增/减增量提交）；绑定弹窗注明"下次登录生效"（D6）。
- **EditRolePage**：新增"权限配置"区块——`roleApi.get` 回填的 `permission_code_list` 作为当前权限回显，`permissionApi.list` 提供可选项，编辑后 diff 出 added/removed 列表随表单提交（依赖 D5-① 修复）。
- **RoleManagementPage / PermissionListPage**：行操作增"删除"（二次确认 + `useApiErrorNotification` 展示引用约束错误）。
- 所有表单的长度/格式校验（邮箱格式、编码长度）沿用 antd Form rules 在提交前完成（Edge Case 3）；删除/修改失败（含对象已被他人删除）经既有错误通知链路展示后端消息。

**Rationale**: 与既有页面模式（Form + Modal.confirm + useApiErrorNotification）完全一致；diff 增量提交使前端无需感知后端并发语义，version 由 get 数据直传。

**Alternatives considered**:
- 绑定独立成页：多一次路由跳转，绑定是账号的低频子操作，Modal 足够；后续如需批量管理可独立立项。
- 绑定 UI 全量提交（提交绑定后完整列表）：需要后端做全量 diff，增量提交与既有 UpdateRole 契约对称，更简单。

> **v2 修订**：绑定/编辑成功提示不再注明"下次登录生效"（D6 v2 已改为立即生效），改为提示"权限变更已生效"。

---

## D12: 分布式环境高可用基线（宪法 X）

**Decision**: 本特性范围内的服务保持并显式落实以下高可用基线：
1. **无状态实例**：iam-service 不引入任何本地内存态（会话在 JWT + Redis、权限快照在 Redis、业务数据在 MySQL），多实例水平扩缩无亲和性要求，实例重启不丢状态；
2. **外部依赖故障隔离**：MySQL / Redis 连接参数沿用 contrib `database` / `cache` 初始化（连接池、超时）；鉴权链路对 Redis 的依赖降级为"未命中读穿 DB"（D6-②），Redis 整体不可用时管理操作按 fail-closed 明确报错（`FailedPrecondition`，消息可理解）并记录结构化日志（D13），**不得静默失败、不得崩溃级联**；
3. **优雅退出**：gRPC/HTTP 服务沿用 contrib `grpcserver.Run` / httpserver 的信号驱动 GracefulStop（10s 超时强停），滚动发布不中断在途请求。

**Rationale**: 宪法 X 要求服务实例无状态化、单点故障可降级；现状已在架构上满足 1/3，缺口在 2（鉴权链路 Redis 硬依赖）——由 D6-② 与本条共同闭合。

**Alternatives considered**:
- 引入本地缓存加速鉴权：制造状态副本，破坏无状态基线与缓存一致性，否决。
- Redis 故障时 fail-open（放行）：可用性换安全性的错误交易，鉴权语义必须 fail-closed，否决。

---

## D13: 结构化错误日志（宪法 X、FR-19）

**Decision**: 对外错误语义与对内日志诊断分离，三层落点：
1. **contrib `grpclib` 新增 `LoggingUnaryServerInterceptor` 并接线 `TraceIDInterceptor`**：默认拦截器链调整为 Recovery → TraceID → Logging → Validation → ErrorDetails。Logging 以 `slog` 记录每个已完成 RPC 的结构化字段：`method`、`grpc_code`、`business_code`（实现 `BusinessError` 接口的错误透出业务码）、`duration_ms`、`trace_id`；TraceID 拦截器负责透传/生成 trace-id 并写入 response header，与日志字段对齐。
2. **service 层错误路径**：所有返回业务错误或底层错误的分支，在返回前以 `slog.Warn/Error` 记录：操作语义、关键上下文（account/role/permission id、提交的编码集合）、底层错误详情（如 DB 约束冲突、Redis 故障）。底层技术错误（SQL、Redis 错误）**仅入日志不透出客户端**。
3. **契约层错误**保持现状：业务码（service_error.go）+ 用户可理解消息（FR-19），前端经既有错误通知链路展示。

**Rationale**: 宪法 X 要求"错误码、发生位置、关键上下文、trace ID 齐备"；现状 Recovery 拦截器仅兜 panic、service 层零日志、trace-id 拦截器存在但未接线——三处缺口一次闭合；拦截器层统一落日志避免在每个 service 方法重复样板。

**Alternatives considered**:
- 仅在 service 层逐点手写日志：覆盖不全且样板重复；拦截器兜底全量请求日志 + service 层补业务上下文是分层正确解。
- 引入 zap/logrus 等新日志库：违反宪法 VIII（不引入平行技术体系），contrib 已统一 `log/slog`，否决。

---

## D11: 列表分页稳定性（Edge Case 1）

**Decision**: 三域 List 查询统一追加 `ORDER BY id ASC` 作为稳定排序基准（当前查询无显式排序，翻页可能重复/遗漏），分页语义沿用 `helper.GetLimitAndOffset` + `common.v1.Page`。

**Rationale**: spec Edge Case 要求翻页"稳定且不重复、不遗漏"；无序分页在 MySQL 下不保证稳定，id 单调递增（krand 趋势递增）是最廉价且无歧义的稳定键。

**Alternatives considered**: 按 create_time 排序：同秒创建顺序不稳定，仍需 id 作次级键，直接以 id 为主键更简。
