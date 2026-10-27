# Tasks: AK/SK 签名鉴权凭证管理

**Input**: Design documents from `/specs/009-ak-sk-auth/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: 本功能包含测试任务。依据宪法原则 VII（可测试性优先）与项目既有约定（`*_test.go` 与被测代码同包，内存 SQLite + miniredis），每个用户故事均附带测试任务；签名规范向量测试先行。

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- 后端：`iam-service/`（pkg/model、pkg/auth、pkg/service、pkg/cipher、server、conf）
- 契约：`idl/`（proto/iam、lib/gwlib、pb 预生成入库）
- 前端：`frontend-service/iam-web/src/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 契约先行落盘与配置外置，为全部用户故事提供编译与运行基础

- [X] T001 按契约 `specs/009-ak-sk-auth/contracts/credential.proto` 落盘 `idl/proto/iam/credential.proto`（package iam.v1、google.api.http 注解、buf.validate 校验），执行项目既有代码生成流程产出 pb/gw/openapiv2，`go build ./...` 编译通过
- [X] T002 [P] 扩展 `iam-service/conf/config.go` 新增 `Credential` 配置段（`timestamp_window_seconds` 默认 300、`max_credentials_per_account` 默认 20、`encrypt_key` 从环境变量 `IAM_CREDENTIAL_ENCRYPT_KEY` 注入），同步更新 `conf/config.yaml` 注释样例；加密密钥缺失时服务启动报错（禁止硬编码/入库/入仓）

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 模型、加解密、错误定义、资源常量、网关透传——全部用户故事的前置

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 [P] 创建 `iam-service/pkg/cipher/aes_gcm.go`：AES-256-GCM Encrypt/Decrypt（随机 nonce 前置拼接密文，密钥 32 字节），配套 `pkg/cipher/aes_gcm_test.go`（加解密往返、篡改密文失败、错误密钥失败）
- [X] T004 [P] 创建 `iam-service/pkg/model/access_credential.go`：AccessCredential 实体（表 `t_access_credential`，字段与唯一索引按 data-model.md；审计字段 + Version 乐观锁；无 is_deleted），`TableName()`、`init()` 调用 `migrate(AccessCredential{})` 注册 AutoMigrate、`AccessFields` 字段列表、`//go:generate contrib gen_gorm_mysql_repository`；**生成后手工补回 uniqueIndex 列常量**（项目已知问题）；配套 `access_credential_test.go`（AutoMigrate 建表、access_key 唯一约束、时间自动填充）
- [X] T005 [P] 扩展 `iam-service/pkg/service/service_error.go`：追加凭证域业务错误（ErrAccessCredentialNotFound / ErrAccessCredentialAccountInvalid / ErrAccessCredentialLimitExceeded / ErrAccessCredentialSkDecrypt，1001XXX 系列，gRPC 码按 data-model.md）；创建 `iam-service/pkg/auth/signature_error.go`：验签错误（ErrSignatureMissing / ErrSignatureKeyUnknown / ErrSignatureKeyDisabled / ErrSignatureKeyExpired / ErrSignatureTimestampExpired / ErrSignatureNonceReplay / ErrSignatureMismatch，均 Unauthenticated）
- [X] T006 [P] 追加资源常量与种子：`iam-service/pkg/model/http_routes.go`（凭证 7 条 HTTP 路由）、`grpc_methods.go`（7 个 gRPC full method）、`menu_keys.go`（访问凭证菜单 key）、`seed.go`（凭证权限资源 + 菜单资源幂等种子，沿用既有 Seed 事务模式）；配套 `seed_test.go` 增补幂等断言
- [X] T007 扩展 `idl/lib/gwlib`：header matcher 与 `with_metadata.go` 透传 `x-fa-access-key` / `x-fa-timestamp` / `x-fa-nonce` / `x-fa-signature` / `x-fa-signature-method` 请求头为 gRPC 元数据，补单测透传断言

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - 管理员为用户创建 AK/SK 凭证 (Priority: P1) 🎯 MVP

**Goal**: 管理员可为指定用户创建 AK/SK 凭证，SK 一次性展示；凭证出现在列表中（含基础分页）

**Independent Test**: 创建凭证后响应一次性返回 AK + 明文 SK；列表出现该凭证且 SK 仅掩码；为无效用户创建被拒。全程不依赖签名验签与启停/删除功能。

### Implementation for User Story 1

- [X] T008 [US1] 创建 `iam-service/pkg/service/access_credential_service.go`：服务骨架（db + rdb 字段、`var _ iam_v1.AccessCredentialServiceServer` 编译断言、model→pb 转换函数、account_name 批量 IN 组装）、实现 `CreateAccessCredential`（AK=`FAK`+krand 随机、SK=32 字节 CSPRNG Base64、AES-GCM 加密落库、用户存在且启用校验、单用户上限 20、审计字段服务端赋值、事务内完成）与 `GetAccessCredential`、`ListAccessCredentials`（基础分页、字段常量查询、禁 N+1）
- [X] T009 [US1] 在 `iam-service/server/grpc.go` 注册 `AccessCredentialServiceServer`、`server/http.go` 注册 `RegisterAccessCredentialServiceHandlerFromEndpoint`
- [X] T010 [US1] 创建 `iam-service/pkg/service/access_credential_service_test.go`（内存 SQLite + miniredis，沿用 setupAccountTestDB 模式）：创建成功返回一次性 SK、用户不存在/停用拒绝、超上限拒绝、AK 唯一冲突重试、审计字段服务端注入且请求值不可覆盖、列表分页与字段全量断言
- [X] T011 [P] [US1] 创建 `frontend-service/iam-web/src/types/credential.ts`（AccessCredential / CreateAccessCredentialRequest / ListAccessCredentialsRequest / Response 等严格类型，id/account_id 为 string）并在 `src/types/index.ts` barrel 导出
- [X] T012 [US1] 在 `frontend-service/iam-web/src/api/iam.ts` 追加 `accessCredentialApi`（createAccessCredential / listAccessCredentials / getAccessCredential，封装 request<T>()）
- [X] T013 [US1] 创建 `frontend-service/iam-web/src/pages/AuthenticationManagement/AccessCredentialListPage.tsx`：列表页（复用 ListPageShell / useTableQuery / 四态：加载/空/异常/无权限；全量业务字段展示，审计字段只读；rowKey=id）+ 创建弹窗（选择用户、备注、可选过期时间、useSubmitLock 防重复提交；成功步骤醒目展示一次性明文 SK + "仅展示一次"警示 + 复制按钮）
- [X] T014 [US1] 在 `frontend-service/iam-web/src/routers/router.tsx` 注册访问凭证路由（懒加载、handle 含 key/icon/label/breadcrumb/permission），`src/types/permissions.ts` 追加菜单 key 与 PERMISSIONS 映射（与后端 menu_keys.go 对齐）

**Checkpoint**: MVP 可独立验证——创建凭证全流程 + 列表可见（quickstart.md 场景 A、C 基础部分）

---

## Phase 4: User Story 2 - 调用方使用 AK/SK 签名调用接口 (Priority: P1)

**Goal**: 请求可凭 AK/SK 签名通过验签，以绑定用户身份进入既有 RBAC 权限判定；防重放、时效窗口生效

**Independent Test**: 用有效凭证签名调用授权接口成功；篡改/过期/重放/禁用/未授权场景按签名规范错误语义拒绝。不依赖前端页面。

### Implementation for User Story 2

- [X] T015 [US2] 创建 `iam-service/pkg/auth/signature.go`：纯函数实现签名规范（`specs/009-ak-sk-auth/contracts/ak-sk-signature-spec.md`）——四要素头提取、规范化串构造（method/URI/query 升序/ak/timestamp/nonce/body SHA-256）、HMAC-SHA256 计算与常数时间比较；配套 `signature_test.go` 签名向量（规范示例复现、篡改必败、空 body 摘要向量）
- [X] T016 [US2] 创建 `iam-service/pkg/auth/signature_store.go`：AK→凭证点查（唯一索引，每请求查库保证禁用/过期即时生效）、SK 解密（pkg/cipher）、状态/过期判定、nonce 防重放（Redis `SETNX aksk:nonce:<ak>:<nonce>` EX=窗口）、last_used_time 节流更新（`SETNX aksk:lastused:<ak>` EX=60，best-effort 失败仅记日志）；配套 `signature_store_test.go`（miniredis：重放拒绝、新 nonce 放行、节流窗口）
- [X] T017 [US2] 扩展 `iam-service/pkg/auth/auth_interceptor.go`：元数据含 `x-fa-access-key` 时走签名验签分支（错误按 signature_error.go 语义化返回 Unauthenticated + ErrorInfo reason），通过后 `WithAccountId(绑定用户)` 注入并复用既有权限快照判定（http:/rpc: 资源键）；无该头时维持 JWT 分支完全不变
- [X] T018 [US2] 扩展 `iam-service/pkg/auth/auth_interceptor_test.go`：7 类验签失败语义断言、验签通过后按绑定用户权限放行/拒绝（FR-015）、未授权接口 PERMISSION_DENIED、JWT 轨道回归用例（FR-010a 双轨）

**Checkpoint**: quickstart.md 场景 B、F 全部通过——签名链路独立可用

---

## Phase 5: User Story 3 - 管理员查看与检索凭证列表 (Priority: P2)

**Goal**: 按 AK / 所属用户 / 状态筛选，翻页条件保留，空态友好

**Independent Test**: 多用户凭证下按筛选条件准确过滤、分页总数正确、翻页后条件保留、无数据空态。依赖 US1 的列表页骨架。

### Implementation for User Story 3

- [X] T019 [US3] 扩展 `iam-service/pkg/service/access_credential_service.go` 的 `ListAccessCredentials`：account_id / access_key（前缀模糊）/ status 筛选条件（where 条件字段均走索引，禁全表扫描），配套 `access_credential_service_test.go` 增补筛选/分页/组合条件用例
- [X] T020 [US3] 扩展 `frontend-service/iam-web/src/pages/AuthenticationManagement/AccessCredentialListPage.tsx`：筛选控件（用户选择、AK 输入防抖 useDebouncedValue、状态选择）、翻页保留查询条件、空数据友好提示完善；`accessCredentialApi` 补齐筛选参数传递

**Checkpoint**: quickstart.md 场景 C 完整通过

---

## Phase 6: User Story 4 - 启用 / 禁用凭证 (Priority: P2)

**Goal**: 管理员启停凭证，禁用后签名调用 5 秒内（实际即时）被拒，操作可审计

**Independent Test**: 禁用后签名调用返回 KEY_DISABLED，启用后恢复；重复设置同状态幂等成功。依赖 US2 的验签链路验证实时性。

### Implementation for User Story 4

- [X] T021 [US4] 在 `iam-service/pkg/service/access_credential_service.go` 实现 `ChangeAccessCredentialStatus`（POST 动作语义、幂等：同状态重复设置返回成功、乐观锁 Version 并发保护、updated_by 审计赋值、结构化审计日志含操作人/AK/动作/traceId），配套测试：启停往返、幂等、并发版本冲突、审计断言
- [X] T022 [US4] `frontend-service/iam-web/src/api/iam.ts` 补 `changeAccessCredentialStatus`；列表页增加启用/禁用操作（状态标签 + 操作按钮按权限显隐），二次确认弹窗说明"该凭证调用将全部失败"，操作后列表状态刷新

**Checkpoint**: quickstart.md 场景 D 通过

---

## Phase 7: User Story 5 - 删除凭证与 SK 重置 (Priority: P3)

**Goal**: 凭证可物理删除（引用校验 + 审计）；SK 可重置轮换（旧 SK 立即失效）

**Independent Test**: 重置后旧 SK 签名被拒、新 SK 通过、AK 不变；删除后凭证消失且调用 KEY_UNKNOWN；删除用户时名下凭证计入引用校验。依赖 US2 验签链路。

### Implementation for User Story 5

- [X] T023 [US5] 在 `iam-service/pkg/service/access_credential_service.go` 实现 `ResetAccessCredentialSecret`（AK 不变、新 SK 加密落库、明文一次性返回、审计日志），配套测试：重置往返、旧签名被拒语义（结合 signature_store 点查）
- [X] T024 [US5] 在 `iam-service/pkg/service/access_credential_service.go` 实现 `DeleteAccessCredential`（物理删除，基础架构型模型；删除前应用层校验、删除记录审计日志：操作人/AK/时间；幂等：二次删除返回 NotFound），配套测试：删除后点查失效、审计断言、二次删除语义
- [X] T025 [US5] 扩展 `iam-service/pkg/service/account_service.go` 的账户删除校验：删除用户前检查名下凭证数为 0（引用完整性检查点，引用存在时拒绝删除），配套 `account_service_test.go` 增补用例
- [X] T026 [US5] `frontend-service/iam-web/src/api/iam.ts` 补 `resetAccessCredentialSecret` / `deleteAccessCredential`；列表页增加"重置 SK"（二次确认 → 成功弹窗一次性展示新 SK + 复制）与"删除"（复用 ConfirmDelete，说明不可恢复）操作，权限显隐

**Checkpoint**: quickstart.md 场景 E 通过；五故事全部独立可用

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: 跨故事收尾与质量门禁

- [X] T027 [P] 在 `iam-service` 服务说明（README 或 conf/config.yaml 注释）链接签名规范 `specs/009-ak-sk-auth/contracts/ak-sk-signature-spec.md`，标注环境变量 `IAM_CREDENTIAL_ENCRYPT_KEY` 必填
- [X] T028 验证失败监控告警接入：确认验签失败结构化日志（AK、失败 reason、来源、traceId）输出路径与既有日志体系一致（FR-014a），补充必要的日志字段
- [ ] T029 按 `specs/009-ak-sk-auth/quickstart.md` 场景 A–F 执行端到端走查（含双轨回归 F），记录结果
- [X] T030 质量门禁：`go vet ./...` + `go test ./...` 全绿；前端 tsc 严格类型检查与 lint 通过；确认无 any 滥用、无 `SELECT *`、审计字段未出现在任何请求体

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 无依赖，立即开始。T001（契约生成）阻塞 T008/T009；T002 阻塞 T003/T016（密钥注入）
- **Foundational (Phase 2)**: 依赖 T001/T002；T003/T004/T005/T006 可并行，T007 独立可并行；**阻塞全部用户故事**
- **US1 (Phase 3)**: 依赖 Foundational。T008 → T009/T010；T011 → T012 → T013 → T014（前端链）
- **US2 (Phase 4)**: 依赖 Foundational，**与 US1 并行无冲突**（仅动 pkg/auth 与其测试；T005 的验签错误为其前置）
- **US3 (Phase 5)**: 依赖 US1（列表页骨架）
- **US4 (Phase 6)**: 依赖 US2（验证实时性）+ US1（列表页操作位）
- **US5 (Phase 7)**: 依赖 US2 + US1
- **Polish (Phase 8)**: 依赖全部用户故事完成

### User Story Dependencies

```text
Phase 1 ──→ Phase 2 ──┬──→ US1 (P1 MVP) ──→ US3 (P2)
                      ├──→ US2 (P1) ──┬──→ US4 (P2)
                      │               └──→ US5 (P3)
                      └──（US1 与 US2 可完全并行）
```

### Parallel Opportunities

- Phase 2 内 T003–T007 全部 [P] 可并行
- **US1 与 US2 触及文件零交集**（service/model vs auth），两线程并行推进
- T011（前端类型）与后端 T008 并行
- US4/US5 的后端 RPC（T021/T023/T024/T025）与各自前端任务在同一故事内顺序执行，但两故事间可并行

---

## Implementation Strategy

### MVP First (US1 + US2)

1. 完成 Phase 1 + Phase 2
2. US1 与 US2 并行推进（两线程、零文件冲突）
3. **STOP and VALIDATE**: quickstart 场景 A/B/F——创建凭证、签名调用、双轨回归
4. 此时已交付需求核心价值：管理能力 + M2M 鉴权链路

### Incremental Delivery

- +US3 → 检索体验完善 → 验证场景 C
- +US4 → 止血能力 → 验证场景 D
- +US5 → 生命周期收尾 → 验证场景 E
- Polish → 端到端走查 + 质量门禁

### Notes

- T004 注意 contrib 生成器丢失 uniqueIndex 常量的已知问题（生成后手工补回）
- 所有写操作审计日志（操作人/AK/动作/时间/traceId）在 T008/T021/T023/T024 内随实现完成，T028 终检
- 前端所有请求体禁止携带审计字段（宪法 XII）；T030 终检
- Commit after each task or logical group; stop at any checkpoint to validate story independently
