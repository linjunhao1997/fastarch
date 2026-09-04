# Implementation Plan: 应用管理

**Branch**: `003-application-management` | **Date**: 2026-09-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/003-application-management/spec.md`

## Summary

为应用管理域补齐目标态能力：在既有增/查/列/改（含启停）基础上，新增 **DeleteApplication**（带引用约束与二次确认的前端交互）能力。后端按既有 DDD 分层（proto 契约 → service 层 GORM 操作 → grpc-gateway REST 暴露）扩展契约与实现；前端在应用列表页接入删除入口（确认弹窗 + 错误提示），并为删除流程补充服务层自动化测试。

## Technical Context

**Language/Version**: Go 1.23.12（iam-service）；TypeScript 5.8 + React 19.1（app）

**Primary Dependencies**: gRPC v1.73 + grpc-gateway v2.27（HTTP 暴露）、GORM v1.30 + MySQL driver（存储）、sponge v1.14（krand 等）、contrib/idl（本地 replace 共享模块）；前端 antd 5.25 + react-router 7.6 + @reduxjs/toolkit 2.8 + Vite 6.3

**Storage**: MySQL（表 `t_application`，GORM AutoMigrate）

**Testing**: Go 标准 `testing` 包（新增 service 层测试，SQLite 内存库驱动，见 research.md D6）；前端无测试设施，验收走 quickstart 手动场景

**Target Platform**: 本地/服务器部署的微服务（gRPC :909x + HTTP 网关 :808x）；前端 Vite dev :5173 按服务前缀代理

**Project Type**: web-service + web 前端（monorepo：`idl/` 契约层、`contrib/` 共享设施、`iam-service/` 业务服务、`app/` 前端）

**Performance Goals**: 管理配置类低频操作；列表首屏 2s 内（SC-002），千条以内数据量

**Constraints**: 应用编码不可变（FR-3）；被引用应用禁止删除（FR-6）；删除需二次确认（FR-7）；所有错误用户可理解（FR-10）

**Scale/Scope**: 单管理域、单表操作；契约变更集中在 `idl/proto/iam/application.proto` 一个文件

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 检查结果 |
| --- | --- |
| I. 类型优先，安全第一 | ✅ 通过 — 新能力全部经 proto 契约强类型定义（`DeleteApplicationRequest`），前端 API 层类型化封装，无 `any` |
| II. 配置驱动，组合优先 | ✅ 通过 — 复用 contrib 的 server 组装与配置加载，不新造平行机制 |
| III. 语义清晰，体系一致 | ✅ 通过 — 命名沿用域内惯例（`DeleteApplication` / `applicationApi.delete`），与既有 CRU 方法语义对齐 |
| IV. 分层防御，用户友好 | ⚠️ 设计后补强 — 既有 `UpdateApplication` 未检查 rows-affected（更新不存在 id 时静默成功，违反 FR-10 精神）；本特性 `DeleteApplication` 必须检查 rows-affected 并返回可理解错误（见 research.md D4），顺带为 Update 补齐同款检查 |
| V. 单一数据源 | ✅ 通过 — 应用生命周期状态仅存于 `t_application`，前端不维护独立事实 |
| VI. 关注点分离 | ✅ 通过 — 契约（idl/proto）→ 业务（pkg/service）→ 数据（pkg/model）分层不变，引用校验置于 service 层 |
| VII. 可测试性优先 | ⚠️ 现状缺口 — 服务层目前零测试；本特性必须为 Delete（含引用拒绝、不存在提示）补服务层自动化测试（research.md D6），否则视为不完整实现 |
| VIII. 技术裁决权 | ✅ 通过 — 沿用 buf/grpc-gateway/GORM 既有技术路线，无新增平行技术体系 |
| IX. 文档语言裁决 | ✅ 通过 — 本特性全部文档（spec/plan/research/quickstart）使用团队主语言中文 |

**结论**：无阻断性违规；两处 ⚠️ 均已转化为本特性的强制设计约束（D4、D6）。

## Project Structure

### Documentation (this feature)

```text
specs/003-application-management/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── application-service.proto   # 契约增量（DeleteApplication RPC + REST/前端契约）
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
idl/proto/iam/application.proto      # [修改] 新增 DeleteApplication RPC + 请求消息
idl/pb/iam/                          # [生成] protoc.sh 重新生成
idl/openapiv2/iam/application.swagger.json  # [生成] swagger 同步更新

iam-service/
├── pkg/service/application_service.go   # [修改] 新增 DeleteApplication（含引用校验、rows-affected 检查）；Update 补 rows-affected 检查
├── pkg/service/application_service_test.go  # [新增] Delete/Update 服务层测试
└── pkg/model/application.go             # [不变] 现有模型已满足（无软删字段，物理删除）

app/src/
├── api/iam.ts                           # [修改] applicationApi 新增 delete
└── pages/ApplicationManagement/ApplicationListPage.tsx  # [修改] 删除按钮 + 二次确认弹窗 + 失败提示
```

**Structure Decision**: 沿用仓库现有三模块结构（宪法·项目结构与仓库约束）：契约变更在 `idl/`，后端实现在 `iam-service/`（DDD 分层：service → model），前端接入在 `app/`。不新建目录。

## Complexity Tracking

> 无宪法违规需要豁免。表格留空。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| （无） | — | — |
