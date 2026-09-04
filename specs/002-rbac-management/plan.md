# Implementation Plan: RBAC 基础管理（用户 / 角色 / 权限）

**Branch**: `002-rbac-management` | **Date**: 2026-09-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/002-rbac-management/spec.md`

## Summary

RBAC 三域的**目标态**补齐：① 契约与实现为 Account / Role / Permission 新增 Delete RPC（引用约束：被绑定/被引用即拒绝，角色删除级联清除权限关联）；② 新增账号-角色绑定管理 `UpdateAccountRoles`（批量绑定/解绑、幂等、校验角色存在），`Account` 契约回带 `role_code_list`；③ 补齐唯一性（邮箱 / 角色编码 / 权限编码：service 层查重 + DB 唯一索引）；④ 乐观锁（version）从 Account 扩展至 Role / Permission（FR-18）；⑤ 修复既有 `UpdateRole` 早退 bug（仅传权限变更列表时被静默跳过）并对未知权限码显式拒绝；⑥ 前端：三个列表页补删除（二次确认）、EditRolePage 补权限配置、账号列表新增"绑定角色"弹窗。绑定/角色权限变更**立即生效**（v2：主动失效 + 读穿回源 + 短 TTL，见 research.md D6 v2——v1"下次登录生效"裁决与 spec 冲突已纠正）；⑦ 宪法 v1.2.0 原则 X 落地：鉴权链路 Redis 故障降级读穿 DB（D12）、contrib 新增结构化日志拦截器并接线 trace-id（D13）；⑧ 宪法 v1.4.0 原则 XI/XII 落地：菜单源唯一化（删除静态 MenuItems 死代码及其死链接/死菜单项，修复用户反馈的"角色管理菜单模块缺失"）、用户下拉菜单死入口处置、`permissionApi.update` URL 对齐契约、账号列表补角色绑定列展示（D14/D15）；⑨ 审计字段标准化与响应回传（用户补充，D16）：模型层统一 created_by/updated_by/create_time/update_time 四件套、service 层注入（操作者取 `auth.GetAccountId(ctx)`）、契约响应消息只读回传、前端列表只读展示——请求体不含审计字段。

## Technical Context

**Language/Version**: Go 1.23.12（iam-service）；TypeScript 5.8 + React 19.1（app）

**Primary Dependencies**: gRPC v1.73 + grpc-gateway v2.27、GORM v1.30 + MySQL driver、Redis（go-redis v9，登录权限缓存）、contrib/idl（本地 replace）；前端 antd 5.25 + react-router 7.6 + Vite 6.3

**Storage**: MySQL（既有表 `t_account` / `t_role` / `t_permission` + 关联表 `t_r_role_permission` / `t_r_account_role`；本特性加唯一索引与 version 列，GORM AutoMigrate）

**Testing**: Go 标准 `testing` 包（SQLite 内存库，沿用 003 建立的模式；account / authorization 服务域目前零测试，本特性必须补齐）

**Target Platform**: 本地/服务器部署的微服务（gRPC :909x + HTTP 网关 :808x）；前端 Vite dev :5173 按服务前缀代理

**Project Type**: web-service + web 前端（monorepo：`idl/` 契约层、`contrib/` 共享设施、`iam-service/` 业务服务、`app/` 前端）

**Performance Goals**: 管理配置类低频操作；列表首屏 2s 内（SC-002，各域千条以内）；完整授权流程 2 分钟内可完成（SC-001）

**Constraints**: 邮箱/角色编码/权限编码唯一（FR-2/6/10）；角色与权限编码创建后不可变（FR-6/10，契约层已满足）；删除引用约束（FR-3/8/11）；绑定幂等（FR-14）；权限仅经角色获得（FR-15）；并发修改可检测（FR-18）；删除二次确认（FR-17）

**Scale/Scope**: 三管理域 + 两关联表；契约变更集中在 `idl/proto/iam/account.proto` 与 `idl/proto/iam/authorization.proto` 两个既有文件

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 检查结果 |
| --- | --- |
| I. 类型优先，安全第一 | ✅ 通过 — 全部新能力经 proto 契约强类型定义（Delete 三 RPC、UpdateAccountRoles、version 字段、role_code_list）；前端 API 层类型化封装，无 `any` |
| II. 配置驱动，组合优先 | ✅ 通过 — 复用 contrib server 组装、拦截器链与既有 GORM 关联机制（many2many），不新造平行机制 |
| III. 语义清晰，体系一致 | ✅ 通过 — 命名沿用域内惯例（`DeleteAccount` / `UpdateAccountRoles` / `accountApi.updateRoles`），与 003 已落的 Delete 模式语义对齐 |
| IV. 分层防御，用户友好 | ⚠️ 设计后满足 — 三处既有静默失败点必须修复并纳入本特性验收：① `UpdateRole` 早退 bug（仅传权限变更时静默跳过）；② UpdateRole/CreateXXX 对未知编码静默跳过 → 改为显式拒绝；③ Role/Permission 更新无并发检测 → 补乐观锁（FR-18）。详见 research.md D4、D5 |
| V. 单一数据源 | ✅ 通过 — 账号权限的唯一事实源是"绑定角色的权限并集"（FR-15，DB 关联表）；Redis 权限缓存为派生数据（读穿回源自 DB 重建，v2 D6），变更后主动失效，立即生效无不可预期中间态 |
| VI. 关注点分离 | ✅ 通过 — 契约（idl/proto）→ 业务（pkg/service）→ 数据（pkg/model）分层不变；引用约束、唯一性、乐观锁判定均置于 service 层 |
| VII. 可测试性优先 | ⚠️ 现状缺口 — account / authorization 服务域零测试；本特性必须为全部新增与修复路径补自动化测试（research.md D8）：唯一性拒绝、三类删除引用约束、绑定幂等/角色不存在、UpdateRole 权限变更（含早退 bug 回归）、乐观锁冲突、缓存失效与读穿回源（v2） |
| VIII. 技术裁决权 | ✅ 通过 — 沿用 buf/grpc-gateway/GORM/Redis/antd 既有技术路线，无新增平行技术体系（日志沿用 contrib 统一的 log/slog，D13） |
| IX. 文档语言裁决 | ✅ 通过 — 本特性全部文档使用团队主语言中文 |
| X. 分布式韧性，日志即线索（v1.2.0 新增） | ⚠️ 设计后满足 — ① 高可用：服务无状态（状态在 JWT+Redis+MySQL）、Redis 故障降级读穿 DB（D12/D6-②），消除现状"快照过期即全体非 admin 鉴权失败"的级联缺陷；② 分布式并发：D2 唯一索引 + D4 乐观锁 + D7 事务幂等已闭合；③ 异常日志清晰：D13 新增 Logging/TraceID 拦截器（method/code/业务码/duration/trace_id）+ service 层错误路径结构化日志（底层错误仅入日志不透出客户端）。全部转化为强制设计约束 D6 v2/D12/D13 |
| XI. 交互完整，入口即行动（v1.3.0 新增） | ⚠️ 现状缺口，设计后满足 — 用户反馈"角色管理菜单模块缺失"；勘察发现双菜单源（router 派生 + `routers/index.tsx` 静态 MenuItems 死代码，后者含 `/authrization_management/*` 拼写死链接与"部门架构/岗位角色"死菜单项）及 IndexPage 用户下拉三个死入口（个人中心/系统设置/退出登录）。处置（D14）：菜单源唯一化为路由派生、删除死代码与死菜单项、退出登录实装、无功能入口移除；菜单可达性纳入 quickstart 步骤 5 验收 |
| XII. 前后端契约对齐，全量暴露与最小提交（v1.4.0 新增） | ⚠️ 现状缺口，设计后满足 — 逐项对照契约接口与前端调用（D15）：`permissionApi.update` URL 缺 `{id}` 路径段 → 修复；`Account.role_code_list` 已回填但列表未展示 → 补"绑定角色"列；CheckPermission 为服务间调用豁免；审计字段（用户补充 D16）：模型层三域不统一（Account 仅时间戳、Role/Permission 全无）→ 统一四件套并由 service 层注入、契约响应消息只读回传、前端列表只读展示；请求消息与前端请求体不含审计字段 → 原则 XII 完整闭合 |

**结论**：无阻断性违规；⚠️ 项均已转化为本特性的强制设计约束（D4/D5、D8、D6 v2/D12/D13、D14/D15）。

## Project Structure

### Documentation (this feature)

```text
specs/002-rbac-management/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── account-service.proto        # 契约增量（DeleteAccount、UpdateAccountRoles、Account.role_code_list、version）
│   └── authorization-service.proto  # 契约增量（DeleteRole、DeletePermission、Role.permission_code_list、version）
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
idl/
├── proto/iam/account.proto          # [修改] 新增 DeleteAccount / UpdateAccountRoles RPC；Account 增加 role_code_list；UpdateAccountRequest 增加 version；Account 增加 created_by/updated_by/create_time/update_time 只读回传（D16）
├── proto/iam/authorization.proto    # [修改] 新增 DeleteRole / DeletePermission RPC；Role 增加 permission_code_list；UpdateRoleRequest / UpdatePermissionRequest 增加 version；CheckPermission 注释注明服务间调用豁免（D15）；Role/Permission 增加审计字段只读回传（D16）
├── pb/iam/                          # [生成] protoc.sh 重新生成
└── openapiv2/iam/                   # [生成] swagger 同步更新

iam-service/
├── pkg/model/account.go             # [修改] email 唯一索引；RAccountRole 补 (account_id, role_id) 唯一索引并纳入迁移清单；新增 created_by/updated_by 列（D16）
├── pkg/model/role.go                # [修改] code 唯一索引；新增 version 列；新增 create_time/update_time/created_by/updated_by 列（D16）
├── pkg/model/permission.go          # [修改] code 唯一索引；新增 version 列；新增 create_time/update_time/created_by/updated_by 列（D16）
├── pkg/auth/auth_interceptor.go     # [修改] 缓存未命中（redis.Nil）读穿回源：DB 重建权限快照后继续校验（D6-②/D12）；Redis 其他故障 fail-closed + 结构化日志
├── pkg/constant/cache_key.go        # [修改] 权限快照 TTL 常量（30min → 5min，D6-③）
├── pkg/service/account_service.go            # [修改] 邮箱查重；DeleteAccount（引用约束）；UpdateAccountRoles（绑定/解绑、幂等、角色存在校验、提交后失效权限快照）；UpdateAccount 改用客户端 version；Create/Update 注入审计字段（D16）
├── pkg/service/account_service_test.go       # [新增] service 层测试（含缓存失效，miniredis；审计字段注入断言，D16）
├── pkg/service/authorization_service.go      # [修改] 编码查重；DeleteRole（引用约束+级联清权限关联）/ DeletePermission（引用约束）；修复 UpdateRole 早退 bug；未知权限码拒绝；乐观锁；权限集合变更后失效绑定账号快照；CheckPermission 读穿回源；错误路径结构化日志；Create/Update 注入审计字段（D16）
├── pkg/service/authorization_service_test.go # [新增] service 层测试（含早退 bug 回归、缓存失效用例、审计字段注入断言，D16）
└── pkg/service/service_error.go              # [修改] 新增 RBAC 域业务错误码（唯一性 / 引用 / 不存在 / 并发冲突）

idl/ & contrib/
├── lib/grpclib/logging_interceptor.go        # [新增] LoggingUnaryServerInterceptor：slog 记录 method/grpc_code/业务码/duration/trace_id（D13；grpclib 位于 idl 模块）
├── lib/grpclib/response_interceptor.go       # [修改] TraceIDInterceptor 接线（无 trace-id 时生成）并纳入默认链
└── contrib/pkg/server/grpcserver/server.go   # [修改] 默认拦截器链调整为 Recovery → TraceID → Logging → Validation → ErrorDetails（D13）

app/src/
├── api/iam.ts                                             # [修改] accountApi 增 delete/updateRoles；roleApi/permissionApi 增 delete；update 系列携带 version；permissionApi.update URL 修正为 /authorization/permissions/{id}（D15）；三个 interface 增审计只读字段（D16）
├── routers/index.tsx                                      # [删除] 静态 MenuItems 死代码（含 /authrization_management/* 死链接与"部门架构/岗位角色"死菜单项），菜单源唯一化为路由派生（D14）
├── routers/router.tsx                                     # [修改] 菜单派生元数据核对：每组入口 label/路由可达，无功能菜单项不注册（D14）
├── pages/IndexPage.tsx                                    # [修改] 用户下拉菜单死入口处置："退出登录"实装（清令牌跳 /login），"个人中心/系统设置"移除（D14）
├── pages/AccountManagement/AccountListPage.tsx            # [修改] 删除按钮（二次确认）+ "绑定角色"弹窗（多选角色 → 增/减增量提交）+ 列表补"绑定角色"列（role_code_list 展示，D15）+ 审计只读列（D16）
├── pages/AuthorizationManagement/RoleManagementPage.tsx   # [修改] 删除按钮（二次确认）+ 审计只读列（D16）
├── pages/AuthorizationManagement/PermissionListPage.tsx   # [修改] 删除按钮（二次确认）+ 审计只读列（D16）
└── pages/AuthorizationManagement/EditRolePage.tsx         # [修改] 权限配置区块（当前权限回显 + 批量增/减，提交增量列表）
```

> `server/grpc.go` / `server/http.go` **不变**：Account / Authorization 服务已注册，新 RPC 属既有 service 的接口实现，随生成代码自动生效（`var _ iam_v1.XxxServiceServer = (*XxxService)(nil)` 编译期强制实现完整接口）。

**Structure Decision**: 沿用仓库现有三模块结构（宪法·项目结构与仓库约束）：契约变更在 `idl/proto/iam/` 两个既有文件内增量扩展，后端实现在 `iam-service/`（service → model 分层不变），前端接入在 `app/` 既有页面。不新建目录、不新建服务、不新建契约文件。

## Complexity Tracking

> 无宪法违规需要豁免。表格留空。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| （无） | — | — |
