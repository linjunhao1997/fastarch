# Implementation Plan: 认证策略管理

**Branch**: `001-auth-policy-management` | **Date**: 2026-09-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-auth-policy-management/spec.md`

## Summary

为前端已存在的"认证策略管理"页面（SSO / MFA / OTP 三页签的列表 / 创建 / 编辑 / 删除）补齐契约与后端能力：新建 `AuthenticationPolicyService` 契约（`idl/proto/iam/auth_policy.proto`，REST 前缀 `/authentication/policies`），按既有分层（proto 契约 → service 层 GORM 操作 → grpc-gateway REST 暴露）实现完整 CRUD，其中策略类型创建后不可变、同类型下名称唯一、更新采用乐观锁防并发覆盖；前端三页全部从本地模拟数据切换为真实 API 调用。策略在认证流程中的**执行**不在本特性范围。

## Technical Context

**Language/Version**: Go 1.23.12（iam-service）；TypeScript 5.8 + React 19.1（app）

**Primary Dependencies**: gRPC v1.73 + grpc-gateway v2.27（HTTP 暴露）、GORM v1.30 + MySQL driver（存储）、sponge v1.14（krand 等）、contrib/idl（本地 replace 共享模块）；前端 antd 5.25 + react-router 7.6 + Vite 6.3

**Storage**: MySQL（新表 `t_authentication_policy`，GORM AutoMigrate；测试用 SQLite 内存库）

**Testing**: Go 标准 `testing` 包（service 层测试，SQLite 内存库，沿用 003 建立的模式）；前端无测试设施，验收走 quickstart 手动场景

**Target Platform**: 本地/服务器部署的微服务（gRPC :909x + HTTP 网关 :808x）；前端 Vite dev :5173 按服务前缀代理

**Project Type**: web-service + web 前端（monorepo：`idl/` 契约层、`contrib/` 共享设施、`iam-service/` 业务服务、`app/` 前端）

**Performance Goals**: 管理配置类低频操作；列表首屏 2s 内（SC-002，单类型百条以内）

**Constraints**: 策略类型创建后不可变（FR-5）；同类型下名称唯一（FR-4）；名称/描述长度约束且提交前校验（FR-7）；并发编辑不得静默覆盖（Edge Case，乐观锁）；删除需二次确认（FR-6）；所有错误用户可理解（FR-10）

**Scale/Scope**: 单管理域、单表操作；契约变更集中在 `idl/proto/iam/auth_policy.proto` 一个新文件 + 服务注册两处

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 检查结果 |
| --- | --- |
| I. 类型优先，安全第一 | ✅ 通过 — 全部经 proto 契约强类型定义；策略类型以 `buf.validate` 白名单（`in: ["sso","mfa","otp"]`）约束非法值；前端 API 层类型化封装，无 `any` |
| II. 配置驱动，组合优先 | ✅ 通过 — 复用 contrib 的 server 组装、配置加载与拦截器链，不新造平行机制 |
| III. 语义清晰，体系一致 | ✅ 通过 — 命名沿用域内惯例（`AuthenticationPolicyService` / `authPolicyApi`），RPC 五件套与既有服务语义对齐 |
| IV. 分层防御，用户友好 | ⚠️ 设计后满足 — spec 明确要求并发编辑不得静默覆盖（既有服务均为 last-write-wins）；本特性以乐观锁（version 字段）+ 冲突错误响应，Delete/Update 均 RowsAffected 检查（见 research.md D4、D5） |
| V. 单一数据源 | ✅ 通过 — 策略数据仅存于 `t_authentication_policy`；前端三页全部移除本地模拟数据（SC-003），以服务端返回为唯一事实 |
| VI. 关注点分离 | ✅ 通过 — 契约（idl/proto）→ 业务（pkg/service）→ 数据（pkg/model）分层不变；重名校验、乐观锁判定置于 service 层 |
| VII. 可测试性优先 | ⚠️ 现状满足中 — 沿用 003 建立的 SQLite 内存库 service 层测试模式，本特性必须为全部 5 个 RPC（含重名拒绝、乐观锁冲突）补自动化测试（research.md D8），否则视为不完整实现 |
| VIII. 技术裁决权 | ✅ 通过 — 沿用 buf/grpc-gateway/GORM/antd 既有技术路线，无新增平行技术体系 |
| IX. 文档语言裁决 | ✅ 通过 — 本特性全部文档使用团队主语言中文 |

**结论**：无阻断性违规；两处 ⚠️ 均已转化为本特性的强制设计约束（D4/D5、D8）。

## Project Structure

### Documentation (this feature)

```text
specs/001-auth-policy-management/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── auth-policy-service.proto    # 完整契约（AuthenticationPolicyService 五 RPC + 消息定义）
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
idl/
├── proto/iam/auth_policy.proto            # [新增] AuthenticationPolicyService 契约（5 RPC + 消息 + buf.validate 规则）
├── pb/iam/                                # [生成] protoc.sh 重新生成（auth_policy.pb.go 等）
└── openapiv2/iam/auth_policy.swagger.json # [生成] swagger 同步生成

iam-service/
├── pkg/model/authentication_policy.go            # [新增] AuthenticationPolicy 模型（t_authentication_policy，(type,name) 唯一索引）
├── pkg/model/authentication_policy_generated.go  # [生成] go generate（contrib gen_gorm_mysql_repository）
├── pkg/service/authentication_policy_service.go   # [新增] CRUD 实现（重名校验、乐观锁更新、RowsAffected 检查）
├── pkg/service/authentication_policy_service_test.go # [新增] service 层测试（SQLite 内存库）
├── pkg/service/service_error.go                   # [修改] 新增策略域业务错误（重名 / 乐观锁冲突 / 类型非法）
└── server/grpc.go、server/http.go                 # [修改] 注册 AuthenticationPolicyService（gRPC server + HTTP gateway）

app/src/
├── api/iam.ts                                        # [修改] 新增 authPolicyApi（create/get/list/update/delete）
├── pages/AuthenticationManagement/AuthenticationPolicyListPage.tsx  # [修改] 三页签接入真实列表 API + 删除（二次确认）
├── pages/AuthenticationManagement/CreateAuthenticationPolicyPage.tsx # [修改] 移除模拟数据，POST 创建（含类型与长度校验）
└── pages/AuthenticationManagement/EditAuthenticationPolicyPage.tsx   # [修改] 移除模拟数据，get 回填 + 乐观锁更新
```

**Structure Decision**: 沿用仓库现有三模块结构（宪法·项目结构与仓库约束）：契约在 `idl/proto/iam/`（与 account/application/authorization/authentication 同级），后端实现在 `iam-service/`（service → model 分层不变），前端接入在 `app/`（页面已存在，仅替换数据源）。不新建目录、不新建服务。

## Complexity Tracking

> 无宪法违规需要豁免。表格留空。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| （无） | — | — |
