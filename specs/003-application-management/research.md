# Research: 应用管理（003）

Phase 0 产出。逐项消解 Technical Context 与 spec 中的决策点。日期：2026-09-05

---

## D1: 删除语义 — 物理删除 vs 软删除

**Decision**: 物理删除（hard delete）。

**Rationale**: spec FR-5 要求删除后"不可再查询"；模型层 `t_application` 无 `gorm.DeletedAt` 字段，引入软删除需要迁移表结构与全套查询语义，成本不成比例。可用性开关已由 `enabled_status`（启停）承担，"停用 ≠ 删除"在 spec Edge Cases 中已明确。

**Alternatives considered**:
- 软删除（`DeletedAt` 字段 + 全局过滤）：可追溯、可恢复，但需表结构迁移且全服务域查询语义都要适配，超出本特性边界；审计追溯需求出现时再立项。

---

## D2: 引用约束的判定范围（FR-6）

**Decision**: service 层删除前查询"持有该应用引用"的从表：以**应用编码（app_code）为纽带**判定——凡表中存在 `application_code` / `source_application_code` / `target_application_code` 类字段且命中待删应用编码的数据，均视为引用（当前至少覆盖权限订阅相关表；实现时以 `iam-service/pkg/model` 中实际存在的应用引用字段全集为准，逐表检查并在代码中集中列出）。

**Rationale**: spec 假设已声明"授权数据以应用编码为纽带引用应用"；判定置于 service 层符合分层原则，且引用清单集中在一处便于后续域扩展时维护。

**Alternatives considered**:
- 数据库外键级联/约束：与"拒绝删除"策略不符，且现模型未建外键，不宜引入。
- 仅检查单一从表：引用面会随域扩展遗漏，违反引用约束的完整性意图。

---

## D3: 契约扩展方式

**Decision**: 在 `idl/proto/iam/application.proto` 的 `ApplicationService` 中新增：

```proto
// 删除应用（仅当未被授权数据引用时可删除）
rpc DeleteApplication(DeleteApplicationRequest) returns (google.protobuf.Empty) {
  option (google.api.http) = { delete: "/application/applications/{id}" };
}

message DeleteApplicationRequest {
  int64 id = 1 [(buf.validate.field).required = true];
}
```

随后在 `idl/proto/` 下运行 `./protoc.sh`（内部执行 `buf dep update && buf generate`），同步再生成 `idl/pb/` 与 `idl/openapiv2/` 产物。

**Rationale**: 与既有 4 个 RPC 的定义风格一致（HTTP 映射、buf.validate 校验、返回 `Empty` 的惯例取自 `UpdateApplication`）；REST 动词取 `DELETE /application/applications/{id}` 与资源语义对齐。

**Alternatives considered**:
- 返回被删实体：无消费方需要，`Empty` 与 Update 惯例一致。
- 以 `app_code` 为删除键：列表/编辑入口均以数字 id 定位资源，id 与既有 Get/Update 一致。

---

## D4: 不存在与冲突错误的返回方式（FR-10、FR-8）

**Decision**:
- `DeleteApplication` 执行 `Where(id).Delete()` 后检查 `RowsAffected`：为 0 时返回与 `GetApplication` 一致的"记录不存在"错误（复用 contrib 的 GORM 错误映射链路，`gorm.ErrRecordNotFound` 已被映射为用户可理解的错误）。
- 引用命中时返回 service 层业务错误（沿用 `iam-service/pkg/service` 中已有的错误构造方式），消息需指明"应用已被引用，请先解除引用"。
- 顺带为既有 `UpdateApplication` 补 `RowsAffected` 检查（宪法 IV：修复静默失败）。

**Rationale**: 现有错误处理模式是"原始错误 + contrib 拦截器映射"，新增能力不引入第二套错误机制；并发场景（A 删后 B 再删）由 RowsAffected 检查天然覆盖。

**Alternatives considered**:
- 先 First 再 Delete（两步查询）：存在 TOCTOU 竞态，单条 DELETE + RowsAffected 更简洁且原子。
- 新建领域错误类型体系：超出本特性边界，保持体系一致性优先。

---

## D5: 前端删除交互

**Decision**: `ApplicationListPage` 每行操作列新增"删除"按钮 → antd `Modal.confirm` 二次确认 → 确认后调 `applicationApi.delete(id)` → 成功后刷新列表并提示成功；失败（被引用/不存在）经 `useApiErrorNotification` 展示后端消息。`app/src/api/iam.ts` 的 `applicationApi` 新增 `delete(id)` 方法（`request` helper 已支持 DELETE 动词，无需改动）。

**Rationale**: 与前端既有列表页交互惯例（编辑按钮 + message 提示 + useApiErrorNotification）完全一致，无新模式引入。

**Alternatives considered**:
- 列表页内嵌 Popconfirm：更轻，但全站删除类操作（后续 002 特性等）尚未定型，先用 Modal.confirm 与现有 Confirm 语义对齐，后续可在 UI 层统一调整。

---

## D6: 服务层测试设施（宪法 VII）

**Decision**: 新增 `iam-service/pkg/service/application_service_test.go`，使用 GORM + **glebarez/sqlite**（纯 Go SQLite 驱动，内存库）构造 `*gorm.DB` 注入 `ApplicationService`，表驱动覆盖：删除成功、删除不存在（RowsAffected=0）、删除被引用（预置引用行）、Update 不存在 id 拒绝。

**Rationale**: 服务层只依赖 `*gorm.DB`，内存 SQLite 可真实执行 SQL 与 RowsAffected 语义，测试无需起 MySQL；纯 Go 驱动避免 Windows 环境 CGO 依赖。本特性首次建立该模式，后续 002 等特性复用。

**Alternatives considered**:
- go-sqlmock：无法验证真实 SQL 行为（如 RowsAffected/关联查询），维护成本高。
- 起 MySQL 容器的集成测试：作为 quickstart 手动验证的替代项可后续加到 CI，不阻塞本特性单测。
- 不写测试：违反宪法 VII（不可测试的实现视为不完整），否决。

---

## D7: 认证与权限控制（FR-9）

**Decision**: 无需新增代码。既有 Auth 拦截器按 `<appCode>:<fullMethodName>` 校验权限，新增的 `DeleteApplication` 方法名自动纳入校验范围（不在 `notAuthMethod` 白名单内）。

**Rationale**: 拦截器按方法全名匹配缓存权限码，新 RPC 零配置即受保护；授权数据的配置（谁能删应用）属运营数据而非代码。

**Alternatives considered**: 无。
