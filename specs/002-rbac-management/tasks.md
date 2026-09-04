# Tasks: RBAC 基础管理（用户 / 角色 / 权限）

**Input**: Design documents from `/specs/002-rbac-management/`

**Prerequisites**: plan.md ✅（v2） | spec.md ✅ | research.md ✅（v2：D6 修订、D12/D13 新增） | data-model.md ✅（v2） | contracts/ ✅（account-service.proto、authorization-service.proto） | quickstart.md ✅（v2）

**Tests**: 宪法 VII 与 plan.md 将测试列为**强制约束**——每个 US 阶段先写测试（跑出 FAIL）再实现。

**Organization**: 按用户故事分组；契约合入与模型/错误码为共享基础（Phase 2），此后各故事可独立实现与验证。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 可并行（不同文件、不依赖未完成任务）
- **[Story]**: 所属用户故事（US1–US5，对应 spec.md 优先级 P1–P5）
- 描述中包含精确文件路径

## Path Conventions

- 契约：`idl/proto/iam/`（生成物 `idl/pb/iam/`、`idl/openapiv2/iam/` 由 protoc.sh 产出）
- 后端：`iam-service/pkg/model/`、`iam-service/pkg/service/`
- 前端：`app/src/api/iam.ts`、`app/src/pages/{AccountManagement,AuthorizationManagement}/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 实施前置条件确认

- [X] T001 按 quickstart.md 步骤 0 执行存量重复数据检查（`t_account.email`、`t_role.code`、`t_permission.code`、`t_r_account_role(account_id, role_id)` 四组 GROUP BY 查询）；存在重复则先人工合并/清理，否则 Phase 2 的唯一索引迁移将失败（research.md D2 风险）
- [X] T002 [P] 确认测试基线：`cd iam-service && go test ./pkg/service/...` 当前全绿（application_service_test.go 通过），`cd app && npm run lint` 无既有错误

**Checkpoint**: 前置条件就绪，可进入契约与模型变更

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 契约、模型与错误码变更——阻塞所有用户故事

**⚠️ CRITICAL**: 以下任务完成后才可开始任何 US 阶段（服务层实现依赖生成的 pb 类型与 model 列）

- [X] T003 将 [contracts/account-service.proto](../002-rbac-management/contracts/account-service.proto) 的【新增】增量合入 `idl/proto/iam/account.proto`：DeleteAccount / UpdateAccountRoles 两个 RPC（含 HTTP 注解）、`UpdateAccountRequest.version`、`Account.role_code_list`、`UpdateAccountRolesRequest` / `DeleteAccountRequest` 消息
- [X] T004 [P] 将 [contracts/authorization-service.proto](../002-rbac-management/contracts/authorization-service.proto) 的【新增】增量合入 `idl/proto/iam/authorization.proto`：DeleteRole / DeletePermission 两个 RPC（含 HTTP 注解）、`UpdateRoleRequest.version`、`UpdatePermissionRequest.version`、`Role.permission_code_list`、`DeleteRoleRequest` / `DeletePermissionRequest` 消息
- [X] T005 运行 `cd idl/proto && ./protoc.sh` 重新生成 `idl/pb/iam/` 与 `idl/openapiv2/iam/`；`cd iam-service && go build ./...`（此时 service 缺新方法会编译失败，属预期——接口断言 `var _ iam_v1.XxxServiceServer` 将强制补齐）依赖 T003、T004
- [X] T006 [P] `iam-service/pkg/model/account.go`：`Email` 字段加 `uniqueIndex` gorm tag；确认 `RAccountRole` 结构体注册进迁移清单（`migration.go` 的 Models）并加 `(account_id, role_id)` 联合唯一索引 tag（research.md D2、D7）
- [X] T007 [P] `iam-service/pkg/model/role.go`：`Code` 字段加 `uniqueIndex`；新增 `Version int64` 字段（column:version，创建置 1）（research.md D4）
- [X] T008 [P] `iam-service/pkg/model/permission.go`：`Code` 字段加 `uniqueIndex`；新增 `Version int64` 字段（research.md D4）
- [X] T009 `iam-service/pkg/service/service_error.go`：按既有 `Error` 结构新增 RBAC 域业务错误（沿用 1001004 起编号）：`ErrEmailDuplicated`、`ErrRoleCodeDuplicated`、`ErrPermissionCodeDuplicated`（AlreadyExists）、`ErrAccountReferenced`、`ErrRoleReferenced`、`ErrPermissionReferenced`（FailedPrecondition，消息指明"请先解除绑定/引用"）、`ErrRoleNotFound`、`ErrPermissionNotFound`（InvalidArgument，消息含缺失编码）、`ErrUpdateRoleBusy`、`ErrUpdatePermissionBusy`（FailedPrecondition，"已被他人修改，请刷新后重试"）（research.md D2/D3/D4/D5）
- [X] T010 启动 `go run . grpc` 验证 AutoMigrate 成功落库：三处唯一索引、两处 version 列、`t_r_account_role` 联合唯一索引（依赖 T006、T007、T008）

**Checkpoint**: 契约生成、模型与错误码就绪——五个用户故事此后可并行实施

---

## Phase 3: User Story 1 - 账号管理 (Priority: P1) 🎯 MVP

**Goal**: 账号列表分页稳定、邮箱唯一性保障、修改具备并发检测——账号域在既有 CRU 之上达到 FR-1/FR-2/FR-18 目标态

**Independent Test**: 创建重复邮箱账号被拒并提示；两个会话先后修改同一账号，后保存方收到冲突提示；列表翻页结果稳定不重复

### Tests for User Story 1 ⚠️（先写，确认 FAIL）

- [X] T011 [P] [US1] 新建 `iam-service/pkg/service/account_service_test.go`（SQLite 内存库模式，沿用 application_service_test.go 的 setup 惯例）：表驱动覆盖——创建重复邮箱被拒、修改为已占用邮箱被拒、携带过期 version 更新被拒（ErrUpdateAccountBusy 语义）、更新不存在 id 返回 NotFound、成功更新后 version+1（research.md D8）

### Implementation for User Story 1

- [X] T012 [US1] `iam-service/pkg/service/account_service.go` CreateAccount：插入前按 email 查重，命中返回 `ErrEmailDuplicated`（research.md D2）
- [X] T013 [US1] `iam-service/pkg/service/account_service.go` UpdateAccount：改用请求携带的 `version` 作条件更新（`WHERE id = ? AND version = ?`），成功后 version+1；RowsAffected==0 时先查存在性区分 NotFound 与 `ErrUpdateAccountBusy`；当 email 有变更时先查重（research.md D4）
- [X] T014 [US1] `iam-service/pkg/service/account_service.go` ListAccounts：查询追加 `ORDER BY id ASC`（research.md D11）
- [X] T015 [US1] `app/src/api/iam.ts` accountApi.update 增加 `version` 参数并随请求体提交；`app/src/pages/AccountManagement/EditAccountPage.tsx` 以 get 回填数据中的 version 提交（需同步确认 pb 的 Account 消息当前是否回传 version——若否，get 回填值暂取列表数据或按契约回填调整，以实现时生成代码为准）
- [X] T016 [US1] 验证 `app/src/pages/AccountManagement/CreateAccountPage.tsx` 与 EditAccountPage 对邮箱重复/并发冲突错误经既有 `useApiErrorNotification` 链路展示用户可理解提示（FR-19）；如有缺失补齐错误分支

**Checkpoint**: US1 独立可验证（`go test ./pkg/service/ -run TestAccount` 全绿 + quickstart 步骤 5.1 场景）

---

## Phase 4: User Story 2 - 角色管理与角色-权限配置 (Priority: P2)

**Goal**: 角色编码唯一、编辑可配置权限（含早退 bug 修复）、更新并发检测——FR-5/6/7 目标态

**Independent Test**: 创建重复编码被拒；仅提交权限增减列表时变更生效（回归）；同时增删权限一次生效；过期 version 提交被拒

### Tests for User Story 2 ⚠️（先写，确认 FAIL）

- [X] T017 [P] [US2] 新建 `iam-service/pkg/service/authorization_service_test.go`（角色部分）：角色编码重复被拒、**早退 bug 回归用例**（仅传 added/removed 列表、不传 name 时权限集合仍变更）、未知权限码整体拒绝且集合不变、增删同请求同时生效、过期 version 被拒、GetRole 回填 permission_code_list（research.md D5/D8）

### Implementation for User Story 2

- [X] T018 [US2] `iam-service/pkg/service/authorization_service.go` CreateRole：插入前按 code 查重，命中返回 `ErrRoleCodeDuplicated`（research.md D2）
- [X] T019 [US2] `iam-service/pkg/service/authorization_service.go` UpdateRole：修复早退 bug——将权限增减处理移至 `len(updates) == 0` 早退 return 之前，保证仅传权限列表时变更生效（research.md D5-①，T017 回归用例锁定）
- [X] T020 [US2] `iam-service/pkg/service/authorization_service.go` UpdateRole：added/removed 集合中任一编码不存在即整体拒绝（`ErrPermissionNotFound`，消息含缺失编码）；改用请求 version 条件更新并 +1，RowsAffected 区分 NotFound / `ErrUpdateRoleBusy`（research.md D4/D5-②）
- [X] T021 [US2] `iam-service/pkg/service/authorization_service.go` GetRole 经 Preload PermissionList 回填 `permission_code_list`；ListRoles 追加 `ORDER BY id ASC`（research.md D1/D11）
- [X] T022 [US2] `app/src/pages/AuthorizationManagement/EditRolePage.tsx`：新增"权限配置"区块——`permissionApi.list` 提供选项、get 回填的 permissionCodeList 回显当前权限、编辑后前端 diff 出 added/removed 增量随表单提交；`app/src/api/iam.ts` roleApi.update 增加 `version` 参数（research.md D10）
- [X] T023 [US2] 验证 `app/src/pages/AuthorizationManagement/CreateRolePage.tsx` 对编码重复错误展示用户可理解提示，缺失则补齐

**Checkpoint**: US1 + US2 独立可用（`go test ./pkg/service/ -run 'TestAccount|TestRole'` 全绿）

---

## Phase 5: User Story 3 - 权限定义管理 (Priority: P3)

**Goal**: 权限编码唯一、更新并发检测——FR-9/10 目标态（编码不可变契约层已满足）

**Independent Test**: 创建重复编码被拒；两个会话先后修改同一权限，后保存方收到冲突提示

### Tests for User Story 3 ⚠️（先写，确认 FAIL）

- [X] T024 [P] [US3] `iam-service/pkg/service/authorization_service_test.go` 追加权限部分用例：权限编码重复被拒、过期 version 被拒、成功更新后 version+1（research.md D8）

### Implementation for User Story 3

- [X] T025 [US3] `iam-service/pkg/service/authorization_service.go` CreatePermission：插入前按 code 查重，命中返回 `ErrPermissionCodeDuplicated`（research.md D2）
- [X] T026 [US3] `iam-service/pkg/service/authorization_service.go` UpdatePermission：改用请求 version 条件更新并 +1，RowsAffected 区分 NotFound / `ErrUpdatePermissionBusy`；ListPermissions 追加 `ORDER BY id ASC`（research.md D4/D11）
- [X] T027 [US3] `app/src/api/iam.ts` permissionApi.update 增加 `version` 参数；`app/src/pages/AuthorizationManagement/EditPermissionPage.tsx` 回填 version 提交；确认 Create/Edit 权限页对编码重复/冲突错误展示可理解提示（research.md D10）

**Checkpoint**: US1–US3 独立可用

---

## Phase 6: User Story 4 - 账号-角色绑定 (Priority: P4)

**Goal**: 账号可绑定/解绑角色（幂等、校验角色存在、单事务），绑定关系经 Get/List 可查询——FR-12/13/14 目标态

**Independent Test**: 为账号绑定角色后 Get 返回 role_code_list 含该角色；重复绑定幂等成功；绑定不存在角色被拒；解绑未绑定角色幂等成功

### Tests for User Story 4 ⚠️（先写，确认 FAIL）

- [X] T028 [P] [US4] `iam-service/pkg/service/account_service_test.go` 追加绑定用例：绑定成功且 Get/List 回填 role_code_list、重复绑定幂等（无重复行）、added 含不存在编码整体拒绝（ErrRoleNotFound）、removed 未绑定编码幂等成功、同一请求增删同时生效、关联表行数断言（research.md D7/D8）

### Implementation for User Story 4

- [X] T029 [US4] `iam-service/pkg/service/account_service.go` 新增 UpdateAccountRoles：单事务内——①校验 added ∪ removed 编码全部存在（缺失即 `ErrRoleNotFound`）；②过滤已绑定编码（幂等）；③`Associations().Append` 绑定 / 按条件删除解绑（removed 幂等）；④DB 联合唯一索引兜底并发（research.md D7）
- [X] T030 [US4] `iam-service/pkg/service/account_service.go` GetAccount / ListAccounts：Preload RoleList 回填 `role_code_list`（契约见 contracts/account-service.proto）（research.md D1）
- [X] T031 [US4] `app/src/api/iam.ts` accountApi 新增 `updateRoles(accountId, { addedRoleCodeList, removedRoleCodeList })`；`app/src/pages/AccountManagement/AccountListPage.tsx` 行操作新增"绑定角色"：Modal 内 `Select mode="multiple"`（选项来自 roleApi.list，初值 = 当前行 roleCodeList），确认时 diff 出增/减增量提交（research.md D6/D10；生效提示文案按 D6 v2 修订，见 T050）

**Checkpoint**: US1–US4 独立可用；SC-001 完整授权流程可跑通

---

## Phase 7: User Story 5 - 删除能力（账号 / 角色 / 权限） (Priority: P5)

**Goal**: 三域物理删除 + 引用约束（拒绝删除）+ 角色级联清权限关联——FR-3/8/11/17 目标态

**Independent Test**: 未引用对象删除成功；被引用对象删除被拒并提示；角色删除后其权限关联被清除；同一对象二次删除收到"不存在"提示

### Tests for User Story 5 ⚠️（先写，确认 FAIL）

- [X] T032 [P] [US5] 两个测试文件追加删除用例：DeletePermission 被角色引用拒绝 / 未引用成功；DeleteRole 被账号绑定拒绝 / 未绑定成功且 `t_r_role_permission` 关联被级联清除；DeleteAccount 被绑定拒绝 / 未绑定成功；三者对不存在 id 均返回 NotFound（二次删除并发场景）（research.md D3/D8）

### Implementation for User Story 5

- [X] T033 [US5] `iam-service/pkg/service/account_service.go` 新增 DeleteAccount：查 `t_r_account_role` 计数，命中返回 `ErrAccountReferenced`；否则物理删除并检查 RowsAffected（research.md D3）
- [X] T034 [US5] `iam-service/pkg/service/authorization_service.go` 新增 DeletePermission：查 `t_r_role_permission` 计数，命中返回 `ErrPermissionReferenced`；否则物理删除并检查 RowsAffected（research.md D3）
- [X] T035 [US5] `iam-service/pkg/service/authorization_service.go` 新增 DeleteRole：查 `t_r_account_role` 计数，命中返回 `ErrRoleReferenced`；否则**事务内**删除角色行 + 级联清除该角色在 `t_r_role_permission` 的关联，检查 RowsAffected（FR-8）（research.md D3）
- [X] T036 [US5] `app/src/api/iam.ts` 三个 Api 增 `delete(id)`；`app/src/pages/AccountManagement/AccountListPage.tsx`、`app/src/pages/AuthorizationManagement/RoleManagementPage.tsx`、`PermissionListPage.tsx` 行操作新增"删除"：`Modal.confirm` 二次确认（FR-17）→ 成功刷新列表 → 失败（被引用/不存在）经 `useApiErrorNotification` 展示后端消息（research.md D10）

**Checkpoint**: 五个用户故事全部独立可用

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: 跨故事收尾与整体验证

- [X] T037 [P] `cd iam-service && go vet ./... && go test ./pkg/service/...` 全绿；`go build ./...` 无警告
- [X] T038 [P] `cd app && npm run lint`（及类型检查）通过；确认无 `any` 引入（宪法 I）
- [ ] T039 按 `specs/002-rbac-management/quickstart.md` 完成端到端验证（步骤 1–5 全部场景 + 步骤 4.3 生效时机/读穿回源/Redis 故障场景 + 步骤 5.6/5.7），记录结果；重点核对：存量唯一索引迁移成功、早退 bug 回归、绑定变更立即生效（不重新登录）、读穿回源快照重建、Redis 故障 fail-closed 明确报错、日志含 trace_id/业务码、三类引用约束拒绝提示（依赖 Phase 9 完成）

---

## Phase 9: v2 修订 — 立即生效与分布式韧性（宪法 v1.2.0 原则 X）

**Purpose**: 落地 research.md D6 v2（绑定/角色权限变更立即生效）、D12（高可用基线：读穿降级、fail-closed）、D13（结构化错误日志）

**⚠️ CRITICAL**: T039 端到端验证必须在本 Phase 完成后执行；后端任务（T043–T049）完成前不得交付"立即生效"语义

- [X] T040 [P] `iam-service/go.mod`：引入 `github.com/alicebob/miniredis/v2`（dev 依赖），`go mod tidy` 确认可解析（research.md D8 v2）
- [X] T041 [P] `idl/lib/grpclib/logging_interceptor.go`（新建）：`LoggingUnaryServerInterceptor`——slog 记录每个已完成 RPC 的结构化字段 `method`、`grpc_code`、`business_code`（实现 `BusinessError` 接口的错误透出业务码）、`duration_ms`、`trace_id`；成功 Info 级、错误 Warn 级（research.md D13-①）
- [X] T042 `idl/lib/grpclib/response_interceptor.go`：`TraceIDInterceptor` 在 metadata 无 trace-id 时生成（krand）并写入 response header；`contrib/pkg/server/grpcserver/server.go` 默认拦截器链调整为 Recovery → TraceID → Logging → Validation → ErrorDetails（依赖 T041，research.md D13-①）
- [X] T043 [P] `iam-service/pkg/constant/cache_key.go`：权限快照 TTL 常量由 30min 调整为 5min（登录写入与读穿回源写回共用该常量）（research.md D6-③）
- [X] T044 ⚠️（先写，确认 FAIL）[P] `iam-service/pkg/service/account_service_test.go` 与 `authorization_service_test.go`：以 miniredis 构造 rdb 追加用例——① UpdateAccountRoles 成功后目标账号 `account:<id>:permissions` 键被删除；② UpdateRole 权限集合变更后所有绑定账号的键被删除；③ 读穿回源（空快照下校验后快照按 DB 权限并集重建、TTL≈5min）；④ 重复绑定无重复行（依赖 T040/T043，research.md D8 v2）
- [X] T045 `iam-service/pkg/auth/auth_interceptor.go`：缓存未命中（`redis.Nil`）改为读穿回源——经可注入的回源函数（含 auth 域测试）从 DB 重建快照（写回 TTL 按 T043 常量）后继续校验；Redis 其他故障 fail-closed 返回可理解错误并 slog 记录 account_id/method/底层错误（依赖 T043/T044，research.md D6-②/D12）
- [X] T046 `iam-service/pkg/service/authorization_service.go` CheckPermission：与 T045 共用回源逻辑，缓存未命中读穿重建（依赖 T045，research.md D6-②）
- [X] T047 `iam-service/pkg/service/account_service.go` UpdateAccountRoles：事务提交成功后删除目标账号的权限快照键；删除失败仅记日志不回滚绑定（快照为派生数据，TTL 兜底）（依赖 T043，research.md D6-①）
- [X] T048 `iam-service/pkg/service/authorization_service.go` UpdateRole（权限集合增减或禁用变更）与 DeleteRole（级联清关联）：提交后查询 `t_r_account_role` 中绑定该角色的全部账号 id，逐一删除其权限快照键（依赖 T043，research.md D6-①）
- [X] T049 `iam-service/pkg/service/account_service.go` 与 `authorization_service.go`：错误路径返回前 slog 记录操作语义 + 关键上下文（account/role/permission id、提交编码集合）+ 底层错误（SQL/Redis 错误仅入日志，不透出客户端）（依赖 T041，research.md D13-②）
- [X] T050 [P] `app/src/pages/AccountManagement/AccountListPage.tsx` 绑定弹窗提示文案改为"权限变更已生效"（T031 修订，D6 v2）；`EditRolePage.tsx` 权限配置保存成功提示同样调整为"权限变更已生效"（research.md D10 v2）
- [X] T051 `cd iam-service && go vet ./... && go test ./pkg/service/... && go build ./...` 全绿；`cd contrib && go build ./...`、`cd idl && go build ./...` 通过（依赖 T041–T050）

**Checkpoint**: 绑定/权限变更立即生效、Redis 故障可降级、日志可诊断——T039 可执行

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 无依赖，立即开始——T001（存量数据检查）是 T010 迁移验证的硬前置
- **Foundational (Phase 2)**: 依赖 Setup；T005 依赖 T003+T004，T010 依赖 T006/T007/T008；**阻塞全部 US 阶段**
- **User Stories (Phase 3–7)**: 均只依赖 Phase 2，可并行（不同文件域：account_service.go / authorization_service.go / app 页面互不冲突）
  - US4 的 T031（绑定弹窗选项来自 roleApi.list）在运行时依赖角色数据存在，但**代码层面不依赖 US2**
  - US5 的引用约束测试（T032）用到绑定关系，**实现顺序建议在 US4 之后**（或同分支后合并验证）
- **v2 修订 (Phase 9)**: 依赖 Phase 3–7（T044–T048 在既有 UpdateAccountRoles/UpdateRole/DeleteRole 实现之上叠加缓存失效与读穿逻辑）；T041–T043、T050 与既有实现无依赖可先行
- **Polish (Phase 8)**: T037/T038 可与 Phase 9 并行；**T039 依赖 Phase 9 完成**

### Within Each User Story

- 测试任务先行（跑出 FAIL）→ service 实现 → 前端接入 → Checkpoint 验证
- 同一文件内的任务（如 T012→T013→T014 均在 account_service.go）按序执行，不并行

### Parallel Opportunities

- Phase 2：T003/T004（两份契约）、T006/T007/T008（三个模型文件）可并行
- 各 US 的测试任务（T011/T017/T024/T028/T032）在 Phase 2 完成后即可并行编写
- 前端任务（T015/T016、T022/T023、T027、T031、T036）与后端同 US 任务在文件层面独立，可并行推进

---

## Parallel Example: Phase 2 契约与模型 / Phase 9 修订

```text
# 可同时派发的任务（互不依赖、不同文件）：
Task: "合入 account.proto 契约增量（T003）"
Task: "合入 authorization.proto 契约增量（T004）"
Task: "account.go 唯一索引与 RAccountRole 迁移（T006）"
Task: "role.go code 唯一索引与 version（T007）"
Task: "permission.go code 唯一索引与 version（T008）"

# Phase 9 可并行组：
Task: "go.mod 引入 miniredis（T040）"
Task: "grpclib 新增 Logging 拦截器（T041）"
Task: "cache_key.go TTL 调整（T043）"
Task: "前端提示文案调整（T050）"
```

## Implementation Strategy

### MVP First（仅 User Story 1）

1. Phase 1 Setup + Phase 2 Foundational（T001–T010）
2. Phase 3 US1（T011–T016）→ **STOP and VALIDATE**：邮箱唯一 + 并发检测独立可用
3. 此时账号域已达 FR-1/2/18 目标态，可交付

### Incremental Delivery

1. Foundation → US1（账号域加固）→ US2（角色+权限配置，含 bug 修复）→ US3（权限域加固）→ US4（绑定闭环，SC-001 可完整跑通）→ US5（删除闭环）
2. 每个故事独立测试、独立交付；US5 最后交付符合 spec 优先级意图（引用约束正确性要求高，独立验证）

## Notes

- [P] 任务 = 不同文件、无未完成依赖
- 测试均为强制（宪法 VII）：先写测试并确认 FAIL，实现后转绿
- 所有新增错误必须用户可理解（FR-19），错误码集中在 service_error.go
- 每个任务或逻辑组完成后提交一次
- 在任意 Checkpoint 停下均可独立验证当前故事
- **v2 修订背景**：宪法 v1.2.0 新增原则 X（分布式韧性），绑定/角色权限变更生效时机由 v1 的"下次登录生效"纠正为"立即生效"（research.md D6 v2），并新增高可用基线（D12）与结构化错误日志（D13）；Phase 1–8 的 [X] 为 v1 实施进度标记，Phase 9 为全部待办
- 同一文件内的 Phase 9 任务（T047/T049 均在 account_service.go、T046/T048/T049 均在 authorization_service.go）按序执行，不并行
