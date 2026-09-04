# Tasks: 应用管理（003）

**Input**: Design documents from `/specs/003-application-management/`

**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: 包含测试任务——宪法 VII（可测试性优先）要求新增服务层逻辑必须可自动化验证（research.md D6）。

**Organization**: 按用户故事分组。注意：US1–US3（列表/创建/编辑）为**已实现能力的现状核验**（spec 目标态以现状纳入），US4（删除）为本特性**实际开发增量**。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 基线确认，无需项目初始化（工程已存在，宪法·项目结构约束：不新建目录）

- [ ] T001 验证工程基线：`iam-service` 执行 `go build ./...` 与 `go test ./pkg/...` 通过；`app` 执行 `npm run build` 通过（记录基线结果，后续任务不得破坏）

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: US4 后端与前端的前置依赖

**⚠️ CRITICAL**: T003 完成前不得开始 US4 的后端/前端实现；T002 完成前不得编写服务层测试

- [X] T002 [P] 在 `iam-service/go.mod` 引入 glebarez/sqlite 纯 Go SQLite 驱动（`github.com/glebarez/sqlite`）用于服务层内存库测试，执行 `go mod tidy`（research.md D6）
- [X] T003 将 `specs/003-application-management/contracts/application-service.proto` 的契约增量（DeleteApplication RPC + DeleteApplicationRequest 消息）合入 `idl/proto/iam/application.proto`，在 `idl/proto/` 下运行 `./protoc.sh` 重新生成 `idl/pb/` 与 `idl/openapiv2/`，确认 `application.swagger.json` 出现 `delete /application/applications/{id}`（research.md D3）

**Checkpoint**: 契约与测试设施就绪，US4 可开始实现

---

## Phase 3: User Story 1 - 查看应用列表 (Priority: P1)

**Goal**: 确认既有列表能力满足 spec 验收场景（分页、真实数据、空列表）

**Independent Test**: 预置应用数据后打开列表页，分页内容与存储一致即通过

### Implementation for User Story 1（现状核验）

- [ ] T004 [P] [US1] 按 quickstart 步骤 4.2 / 5-1 核验列表验收场景（spec US1 场景 1–3）：分页数据一致、翻页稳定、空列表展示空状态；发现偏差记录并最小修复 `app/src/pages/ApplicationManagement/ApplicationListPage.tsx`

**Checkpoint**: US1 验收通过

---

## Phase 4: User Story 2 - 创建应用 (Priority: P2)

**Goal**: 确认既有创建能力满足 spec 验收场景（必填校验、重名拒绝、编码生成）

**Independent Test**: 表单创建一个应用，列表可见且编码已生成即通过

### Implementation for User Story 2（现状核验）

- [ ] T005 [P] [US2] 按 quickstart 步骤 4.1 核验创建验收场景（spec US2 场景 1–3）：名称必填拒绝、重名拒绝提示、创建后 app_code 生成且唯一；发现偏差记录并最小修复 `iam-service/pkg/service/application_service.go` 或 `app/src/pages/ApplicationManagement/CreateApplicationPage.tsx`

**Checkpoint**: US2 验收通过

---

## Phase 5: User Story 3 - 编辑应用 (Priority: P3)

**Goal**: 补齐更新静默失败缺陷（宪法 IV）并确认既有编辑/启停能力满足验收场景

**Independent Test**: 修改应用描述并停用，列表确认变更即通过

### Implementation for User Story 3

- [X] T006 [US3] 在 `iam-service/pkg/service/application_service.go` 的 `UpdateApplication` 中补充 `RowsAffected` 检查：更新不存在 id 时返回与 `GetApplication` 一致的"记录不存在"错误，消除静默成功（research.md D4，依赖 T002 已就绪的测试设施做验证）
- [X] T007 [US3] 在 `iam-service/pkg/service/application_service_test.go` 新增表驱动用例：更新不存在 id 返回不存在错误、更新存在 id 成功且 update_time 变化（依赖 T002、T006）
- [ ] T008 [P] [US3] 按 quickstart 步骤 5 核验编辑验收场景（spec US3 场景 1–4）：描述修改可见、启停切换生效、名称清空被拒、编码不可修改；发现偏差记录并最小修复 `app/src/pages/ApplicationManagement/EditApplicationPage.tsx`

**Checkpoint**: US1–US3 全部验收通过（既有能力 + Update 缺陷修复）

---

## Phase 6: User Story 4 - 删除应用 (Priority: P4) 🎯 开发增量

**Goal**: 交付 DeleteApplication 全链路：契约（T003 已完成）→ 后端实现（引用约束 + 不存在提示）→ 服务层测试 → 前端删除交互

**Independent Test**: 删除未引用应用成功且列表移除；删除被引用应用被拒并提示；删除不存在应用得到明确提示（quickstart 4.3–4.5、5-2–5-4）

### Implementation for User Story 4

- [X] T009 [US4] 在 `iam-service/pkg/service/application_service.go` 实现 `DeleteApplication`（依赖 T003）：单条 `Where(id).Delete()` 物理删除（research.md D1）；执行前按 D2 逐表检查应用编码引用（以 `pkg/model` 中实际持有 `application_code` 类字段的表为准，引用清单集中在方法内列明），命中则返回"应用已被引用，请先解除引用"业务错误（research.md D2）；`RowsAffected == 0` 时返回记录不存在错误（research.md D4）
- [X] T010 [US4] 在 `iam-service/pkg/service/application_service_test.go` 新增 DeleteApplication 表驱动用例（依赖 T002、T009）：删除成功后不可查询、删除不存在 id 返回不存在错误、删除被引用应用返回引用错误、重复删除幂等拒绝（spec US4 场景 1/2/6）
- [X] T011 [P] [US4] 在 `app/src/api/iam.ts` 的 `applicationApi` 中新增 `delete(id)` 方法（`request` helper 已支持 DELETE，见 contracts/application-service.proto 前端 API 契约；依赖 T003 生成的网关路由）
- [X] T012 [US4] 在 `app/src/pages/ApplicationManagement/ApplicationListPage.tsx` 操作列新增"删除"按钮：`Modal.confirm` 二次确认（标题"确认删除"，内容"确定要删除此应用吗？删除后不可恢复。"）→ 确认调 `applicationApi.delete(id)` → 成功 `message.success("删除成功")` 并刷新列表；失败经 `useApiErrorNotification` 展示后端消息（research.md D5；依赖 T011）
- [ ] T013 [US4] 端到端核验删除流程（依赖 T009、T012）：按 quickstart 步骤 4.3–4.5 与 5-2–5-4 验证删除成功/取消/引用约束/并发不存在提示

**Checkpoint**: US4 全链路可用，spec FR-5~FR-8 验收通过

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: 全特性收尾

- [ ] T014 [P] 执行 `go vet ./...`（iam-service）与 `npm run build`（app），确认无回归
- [ ] T015 按 `specs/003-application-management/quickstart.md` 末尾验收对照表逐项运行并记录结果（步骤 1–5 全量）

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup（T001）**: 无依赖，立即执行
- **Foundational（T002/T003）**: 依赖 T001（基线确认）；T002 与 T003 相互独立可并行
- **US1/US2（T004/T005）**: 仅依赖 T001，与 Foundational 无关，可立即并行
- **US3（T006–T008）**: T006 依赖 T002（测试设施）；T007 依赖 T006；T008 独立可并行
- **US4（T009–T013）**: T009/T011 依赖 T003；T010 依赖 T002+T009；T012 依赖 T011；T013 依赖 T009+T012
- **Polish（T014/T015）**: 依赖全部所需故事完成

### User Story Dependencies

- **US1 / US2**: 相互独立，可与所有阶段并行（纯核验）
- **US3**: 仅依赖 Foundational 的 T002；与 US4 无耦合
- **US4**: 依赖 T003（契约生成）；是唯一包含后端/前端新代码的故事

### Parallel Opportunities

- T002 ∥ T003（互不相干的文件）
- T004 ∥ T005 ∥ T008（不同核验对象，互不相干）
- T011（前端 API）与 T009（后端实现）可由不同人并行——T011 仅依赖 T003 的路由定义
- T010（测试）与 T012（前端交互）可并行（不同文件、不同层）

### 并行示例: US4

```text
# T003 完成后：
Task: "T009 实现 DeleteApplication（iam-service/pkg/service/application_service.go）"   ← 开发 A
Task: "T011 applicationApi.delete（app/src/api/iam.ts）"                                ← 开发 B（并行）
# T009 完成后：
Task: "T010 服务层测试（application_service_test.go）"                                  ← 开发 A
Task: "T012 列表页删除交互（ApplicationListPage.tsx）"                                   ← 开发 B（并行，依赖 T011）
```

---

## Implementation Strategy

### MVP First

本特性 US1–US3 为现状核验（无新代码，仅 T006 缺陷修复），实际价值增量集中在 US4：

1. T001 基线确认
2. T002 ∥ T003（并行完成 Foundational）
3. T004 ∥ T005（US1/US2 核验可随时穿插）
4. US4 全链路（T009→T010 / T011→T012→T013）
5. **STOP and VALIDATE**: 按 quickstart 验收对照表全量验证

### Single-Developer 建议顺序

T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008 → T009 → T010 → T011 → T012 → T013 → T014 → T015

---

## Notes

- [P] 任务 = 不同文件、无未完成依赖
- [Story] 标签映射 spec.md 用户故事（US1=查看列表, US2=创建, US3=编辑, US4=删除）
- US1–US3 的"核验"任务发现偏差时：记录偏差 → 最小修复 → 复验；禁止借机重构
- 每个 Checkpoint 处可独立验证；建议按逻辑组提交（commit）
- 禁止：模糊任务、同文件并行冲突、破坏故事独立性的跨故事依赖
