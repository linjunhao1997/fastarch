# Research: 认证策略管理（001）

Phase 0 产出。逐项消解 Technical Context 与 spec 中的决策点。日期：2026-09-05

> 本特性与 003（应用管理）同构，多数技术选型直接沿用仓库既有裁决，仅对策略域特有问题展开论证。

---

## D1: 契约文件位置与服务命名

**Decision**: 新建 `idl/proto/iam/auth_policy.proto`，定义 `AuthenticationPolicyService`，五个 RPC：`CreateAuthenticationPolicy` / `GetAuthenticationPolicy` / `ListAuthenticationPolicies` / `UpdateAuthenticationPolicy` / `DeleteAuthenticationPolicy`；REST 路径前缀取 `/authentication/policies`。

**Rationale**: 与既有四个域契约（account / application / authorization / authentication）的文件组织、service 命名、五 RPC 结构完全一致。HTTP 前缀沿用"`<域>/<资源复数>`"惯例；`/authentication/` 前缀已被 `authentication.proto`（password-auth）占用但路径不冲突，且策略确属认证管理域（与前端路由 `/authentication_management/*` 对应）。

**Alternatives considered**:
- 复用 `common.v1.GetRequest` 作为 Get/Delete 请求：沿用（与既有 Get/Delete 完全一致）。
- 前缀 `/auth_policy/`：破坏"`<域>/`"惯例，且策略归属认证域而非独立域。

---

## D2: 策略类型的契约表示 — proto enum vs 受约束 string

**Decision**: `policy_type` 使用 `string`，以 `buf.validate` 白名单约束：`(buf.validate.field).string = { in: ["sso", "mfa", "otp"] }`。

**Rationale**: 前端路由参数与页签取值已是小写字符串（`:type` = `sso`/`mfa`/`otp`）；既有契约中状态类字段（`enabled_status` 等）均采用字符串惯例；grpc-gateway 对 proto enum 的 JSON 序列化输出枚举名（如 `SSO`），前端需额外映射层。`buf.validate` 白名单使非法类型在生成层即被拒绝（对应 Edge Case"无效的认证策略类型"），类型安全性不弱于 enum。

**Alternatives considered**:
- proto `enum PolicyType`：编译期更强的类型，但引入 JSON 值映射、与全仓库字符串状态惯例不一致，违反宪法 III（体系一致性）。
- 无约束 string + service 层校验：校验后移，非法值进入业务层才被发现，弱于生成层拦截。

---

## D3: 同类型重名唯一性（FR-4）

**Decision**: 双层防护——
1. **DB 层**：`t_authentication_policy` 上建联合唯一索引 `uk_type_name (type, name)`，并发下兜底；
2. **Service 层**：Create（以及 Update 改名时）先按 `(type, name)` 查重，命中即返回业务错误 `ErrPolicyNameDuplicated`（提示"该策略类型下已存在同名策略"，gRPC `AlreadyExists`）。

**Rationale**: 仅 service 查重存在并发竞态（两请求同时通过查重后双双插入）；仅 DB 索引则错误信息不友好（原始约束冲突错误）。双层结合以 service 层保证可理解提示（FR-10）、DB 索引保证最终一致。唯一性以 `(type, name)` 为键符合 spec"同一策略类型下"重名判定。

**Alternatives considered**:
- 仅 service 查重：并发竞态下可产生重名数据，违反 FR-4。
- 仅 DB 唯一索引：需将 gorm 约束错误翻译为友好提示，不如前置查重直观。
- 全局（跨类型）名称唯一：spec 明确重名判定范围是"同一策略类型下"，跨类型允许重名。

---

## D4: 并发编辑冲突处理（Edge Case：后保存方不得静默覆盖）

**Decision**: 乐观锁。模型增加 `version int64` 字段（创建时置 1，每次更新 +1）；`UpdateAuthenticationPolicyRequest` 携带 `version`（必填，来自编辑页读取时的数据）。service 层更新流程：先 `First` 确认存在（不存在 → `NotFound`），再 `UPDATE ... WHERE id = ? AND version = ?`，`RowsAffected == 0` 即并发冲突，返回 `ErrPolicyConflict`（"策略已被他人修改，请刷新后重试"，gRPC `FailedPrecondition`）。

**Rationale**: spec Edge Case 明确要求"后保存的一方不得覆盖先保存方的修改而不留痕迹"，last-write-wins 不满足。乐观锁是无锁化方案中最简单且可测试的实现；`version` 列比 `update_time` 比对更可靠（同秒内两次修改 timestamp 可能相同）。这也是本特性对既有服务惯例的**首个**并发防护，先在策略域落地，不回溯改造其他域（避免范围膨胀）。

**Alternatives considered**:
- `update_time` 等值比对：时间精度不足（同秒并发修改无法区分），不可靠。
- 行级锁（`SELECT ... FOR UPDATE`）：跨请求长事务持有锁，管理页面低频但编辑间隔可能达分钟级，行锁不可行（提交时才锁）。
- 审计日志追溯（spec 允许的替代）：满足"可追溯"但不满足"给出冲突提示"，且审计体系超出本特性边界。
- 不处理（沿用既有 last-write-wins）：违反 spec Edge Case，否决。

---

## D5: 不存在与删除语义（FR-5/FR-6、并发删除）

**Decision**: 物理删除；Delete 与 Update 均检查 `RowsAffected`：为 0 时返回"记录不存在"（`gorm.ErrRecordNotFound` 经 contrib `GormErrorInterceptor` 已映射为用户可理解的 `NotFound`）。A 删后 B 再删（spec US4 场景 3）由该检查天然覆盖。策略类型不可变（FR-5）通过**契约层不提供 `policy_type` 更新字段**实现——Update 请求消息中根本不含该字段，编辑表单也无此输入，不可能被修改。

**Rationale**: 与 003 D1（物理删除）、D4（RowsAffected 检查）裁决一致；"契约不出现即不可变"是最强的不可变保障，优于"接收字段但忽略"。

**Alternatives considered**:
- 软删除：管理配置数据、无审计需求，成本不成比例（同 003 D1）。
- Update 接收 `policy_type` 并校验不变：契约允许传入即引入歧义，弱于不提供。

---

## D6: 长度与必填约束（FR-7）

**Decision**: 约束值在契约与前端两侧保持一致，定义如下（buf.validate 于生成层执行，antd Form rules 于提交前执行）：

| 字段 | 约束 |
| --- | --- |
| name | 必填，1–64 字符 |
| description | 必填，1–255 字符 |
| implementation_id | 必填，1–64 字符 |

**Rationale**: spec 要求创建时三项必填（FR-3）且提交前完成长度校验（FR-7）；上限取管理配置类数据的常规量级（名称 64 / 描述 255 / 实现方式标识 64），与 DB 列 `varchar` 长度对齐，DB 层不构成第二套约束。双侧同值避免"前端放行、后端拒绝"或反之。

**Alternatives considered**:
- description 可选：spec FR-3 明确"创建时 MUST 提供名称、描述、实现方式标识"，必填。
- 更宽松上限（如 1024）：超出管理配置数据量级预期（spec Assumptions），无必要。

---

## D7: 列表查询按类型过滤（FR-2、US1）

**Decision**: `ListAuthenticationPoliciesRequest { policy_type（必填，白名单校验）, current_page, page_size }`，响应 `ListAuthenticationPoliciesResponse { page, authentication_policies }`；service 层 `WHERE type = ?` 查询。前端三个页签各自携带对应 `policy_type` 调用同一接口，切换页签即重新拉取，无需整页刷新（US1 场景 2）。

**Rationale**: 分页参数与响应结构沿用 `common.v1.Page` 与既有 List 惯例；`policy_type` 必填使"查全部类型"不可达，与页面按页签展示的领域语义一致，也天然满足"页签间数据隔离"的验收场景。

**Alternatives considered**:
- policy_type 可选（缺省查全部）：页面无此场景，放宽契约反而引入非法缺省语义。
- 三个独立 RPC（ListSsoPolicies 等）：类型当前固定三种但未来可扩展（spec Assumptions），参数化优于方法爆炸。

---

## D8: 服务层测试（宪法 VII）

**Decision**: 新增 `iam-service/pkg/service/authentication_policy_service_test.go`，沿用 003 建立的模式：GORM + `glebarez/sqlite` 内存库注入 service，表驱动覆盖——
- List：按类型过滤正确（SSO 页签不见 MFA 数据）、空类型返回空列表、分页字段正确；
- Create：成功（id 生成、version=1）、同类型重名被拒、**跨类型同名允许**、缺必填被生成层校验拒绝；
- Update：成功（version 递增、描述更新）、version 不匹配冲突被拒、原数据不被覆盖、id 不存在返回 NotFound；
- Delete：成功、重复删除返回 NotFound。

**Rationale**: 与 `application_service_test.go` 完全同构（同一测试基建、零新依赖）；乐观锁与重名拒绝是本特性新增的复杂分支，必须自动化验证。

**Alternatives considered**: 同 003 D6（go-sqlmock / MySQL 容器 / 不写测试，均否决）。

---

## D9: 认证与权限控制（FR-9）

**Decision**: 无需新增代码。既有 Auth 拦截器按 `<appCode>:<fullMethodName>` 校验权限，`AuthenticationPolicyService` 的全部方法名自动纳入校验（不在 `notAuthMethod` 白名单内）。前端沿用现有登录态（Bearer token）。

**Rationale**: 与 003 D7 相同——新 RPC 零配置即受保护；授权数据配置属运营数据而非代码。

**Alternatives considered**: 无。

---

## D10: 前端接入方式

**Decision**: `app/src/api/iam.ts` 新增 `authPolicyApi`（create/get/list/update/delete，与 `applicationApi` 同构，`list` 额外携带 `policy_type`）。三个既有页面移除全部模拟数据：
- **列表页**：三个页签各自以 `authPolicyApi.list(type, 1, 100)` 拉取；删除按钮 → `Modal.confirm` 二次确认（FR-6）→ 确认后调 delete → 成功刷新页签列表，失败经 `useApiErrorNotification` 展示后端消息（含"策略不存在"）；
- **创建页**：从路由 `:type` 取策略类型随表单提交；antd Form rules 实施必填与长度上限（D6，提交前校验）；
- **编辑页**：`get(id)` 回填（含 `version`），提交时携带 version 供乐观锁判定（D4）；收到冲突错误时提示刷新。

**Rationale**: 与前端既有模式（applicationApi + Form + Modal.confirm + useApiErrorNotification）完全一致，无新模式引入；页面骨架与路由均已存在，改动收敛在数据源替换。

**Alternatives considered**:
- 全局状态（redux）缓存策略列表：管理配置数据低频，直接拉取即可满足 FR-8（不展示过期数据），缓存反而引入失效复杂度。
- 乐观锁在前端做 diff 提示：冲突判定必须以服务端 version 为准（单一数据源，宪法 V）。
