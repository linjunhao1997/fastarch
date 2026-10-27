# Implementation Plan: AK/SK 签名鉴权凭证管理

**Branch**: `009-ak-sk-auth` | **Date**: 2026-09-12 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/009-ak-sk-auth/spec.md`

## Summary

为 IAM 增加基于 AK/SK 的机器对机器（M2M）签名鉴权能力：新增访问凭证模型（AK 全局唯一、SK 静态加密存储、绑定用户）、凭证管理 gRPC/HTTP 接口（创建/列表/更新/启停/删除/重置 SK）与前端"访问凭证"页面（认证管理菜单下）；扩展 gRPC 鉴权拦截器支持 HMAC-SHA256 签名验证（时间戳窗口 + nonce 防重放），验签通过后以绑定用户身份进入既有 RBAC 权限判定，实现与登录态鉴权双轨并存。

## Technical Context

**Language/Version**: Go 1.x（iam-service，go.mod 现状）；TypeScript 严格模式（frontend-service/iam-web）

**Primary Dependencies**: gRPC + grpc-gateway（idl/pb 预生成）；GORM（MySQL）；Redis（权限快照 / nonce 去重）；sponge `krand`（ID 与随机串）；React 18 + Ant Design + @tanstack/react-query（前端）

**Storage**: MySQL（`t_access_credential`，经 GORM AutoMigrate）；Redis（`aksk:nonce:*` 防重放键、权限快照复用）

**Testing**: Go 标准 testing + 内存 SQLite（`glebarez/sqlite`）+ miniredis（贴近现有 `pkg/service/*_test.go` 约定）；前端随现有工程约定

**Target Platform**: Linux 服务器（`iam-service grpc/http` 子命令），浏览器（管理端）

**Project Type**: 微服务（`xxx-service`）+ 管理前端（`frontend-service/iam-web`）

**Performance Goals**: 验签新增开销毫秒级（本地 HMAC 计算 + 1 次 Redis nonce SETNX）；凭证管理接口与现有 CRUD 同量级

**Constraints**: 时间戳窗口默认 ±300s；单用户凭证上限默认 20；SK 静态加密密钥经环境变量注入（`IAM_CREDENTIAL_ENCRYPT_KEY`），不入库不入仓；禁用/删除后 5s 内生效（nonce 键与权限快照 TTL 兜底）

**Scale/Scope**: 1 张新表、1 个新 proto 服务（7 个 RPC）、鉴权拦截器扩展、前端 1 个列表页 + 3 个操作弹窗；凭证量级为管理配置数据（千级以内）

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 结论 | 依据 |
| --- | --- | --- |
| I. 类型优先 | ✅ 通过 | 契约先行：proto + buf validate 定义全部 RPC 与校验；前端 types/credential.ts 严格类型 |
| II. 配置驱动 | ✅ 通过 | 时间戳窗口、单用户上限、加密密钥全部外置配置（conf/config.yaml + 环境变量），不硬编码 |
| III. 语义清晰 | ✅ 通过 | 命名统一 `AccessCredential` / `aksk` 前缀（HTTP 头 `X-Fa-*`），全库一致 |
| IV. 分层防御 | ✅ 通过 | 验签错误分类语义化（FR-014），不泄露 SK 信息；失败即记录结构化日志 |
| V. 单一数据源 | ✅ 通过 | 凭证唯一权威为 MySQL；Redis 仅承载可重建的 nonce 去重与权限快照 |
| VI. 关注点分离 | ✅ 通过 | 签名验证收敛在 `pkg/auth`（拦截器层），凭证管理业务在 service 层，互不渗透 |
| VII. 可测试性 | ✅ 通过 | 签名验证函数纯函数化；沿用内存 SQLite + miniredis 测试约定覆盖全部验收场景 |
| VIII. 技术裁决权 | ✅ 通过 | HMAC-SHA256 为标准库能力，不引入第三方依赖；AES-GCM 亦为标准库（crypto/aes + cipher） |
| IX. 文档语言 | ✅ 通过 | 设计产物中文，技术术语保留原文 |
| X. 分布式韧性 | ✅ 通过 | nonce 去重走共享 Redis（多实例一致）；拦截器无状态可水平扩容；last-used-time 节流写入避免写放大 |
| XI. 交互完整 | ✅ 通过 | 列表/创建/启停/重置/删除全部真实可动；无占位入口 |
| XII. 契约对齐 | ✅ 通过 | 7 个 RPC 全部被前端调用；审计字段服务端注入、请求体不携带；创建/重置响应含一次性 SK（业务必需字段） |
| XIII. 前端一致性 | ✅ 通过 | 复用 ListPageShell / ConfirmDelete / useTableQuery / useSubmitLock / 四态模式；高危操作二次确认 |
| XIV. Go 工程规范 | ✅ 通过 | 模型显式归类基础架构型（物理删除 + 应用层引用校验 + 审计日志）；无物理外键、关联字段建索引；事务在 service 层；禁止 SELECT *（使用字段常量）；分页列表遵循统一返回结构（proto 契约为权威） |
| 仓库结构约束 | ✅ 通过 | iam-service / frontend-service/iam-web / idl / contrib 职责不变更 |

**偏差说明（1 项，已在 research.md 裁决）**：规格 FR-003 "不可逆保护"在 HMAC 对称验签下不可实现——服务端必须持有可还原的 SK 才能重算签名。裁决为 AES-256-GCM 静态加密（密钥环境变量注入），达成 FR-003 的本质目标（非明文落库、数据库泄露不可直接还原）。属规格澄清而非宪法违反，已同步标注于 data-model.md。

## Project Structure

### Documentation (this feature)

```text
specs/009-ak-sk-auth/
├── plan.md              # 本文件
├── research.md          # Phase 0 技术裁决
├── data-model.md        # Phase 1 数据模型
├── quickstart.md        # Phase 1 验证指南
├── contracts/
│   ├── credential.proto         # 访问凭证服务契约
│   └── ak-sk-signature-spec.md  # AK/SK 签名规范（对外公开、可复现）
└── tasks.md             # Phase 2 产出（/speckit-tasks）
```

### Source Code (repository root)

```text
idl/
└── proto/iam/credential.proto           # 新增：AccessCredentialService 契约（regenerate pb/gw）

iam-service/
├── pkg/model/
│   ├── access_credential.go             # 新增：凭证实体 + TableName + migrate 注册 + Fields
│   ├── access_credential_generated.go   # go generate 产出列常量（注意手工补回 uniqueIndex 常量）
│   ├── http_routes.go                   # 追加凭证管理 7 条路由
│   ├── grpc_methods.go                  # 追加凭证 gRPC full methods
│   ├── menu_keys.go                     # 追加访问凭证菜单 key
│   └── seed.go                          # 追加内置凭证权限资源绑定（可选，随 RBAC 种子）
├── pkg/auth/
│   ├── signature.go                     # 新增：HMAC-SHA256 规范化串构造 + 验签（纯函数）
│   ├── signature_test.go                # 新增：签名向量与篡改/过期/重放用例
│   ├── signature_store.go               # 新增：SK 解密（AES-256-GCM）+ 凭证读取
│   └── auth_interceptor.go              # 扩展：X-Fa-* 元数据走签名验签分支，复用权限快照判定
├── pkg/service/
│   ├── access_credential_service.go     # 新增：7 个 RPC 实现（直接 GORM，无 repository 层）
│   ├── access_credential_service_test.go# 新增：内存 SQLite + miniredis
│   └── service_error.go                 # 追加凭证域业务错误（1001XXX 系列）
├── server/grpc.go                       # 注册 AccessCredentialService
├── server/http.go                       # 注册 HandlerFromEndpoint
└── conf/config.go                       # 追加 Credential 配置段（窗口/上限/加密密钥 env）

frontend-service/iam-web/src/
├── types/credential.ts                  # 新增：凭证领域类型
├── types/index.ts                       # 追加 barrel 导出
├── types/permissions.ts                 # 追加菜单 key 与 PERMISSIONS 映射
├── api/iam.ts                           # 追加 accessCredentialApi（7 接口全量封装）
├── pages/AuthenticationManagement/
│   └── AccessCredentialListPage.tsx     # 新增：列表页（筛选/分页/四态）+ 创建弹窗（一次性 SK 步骤）
│                                        #   + 编辑弹窗（备注/过期时间）+ 启停/重置/删除确认
└── routers/router.tsx                   # 追加路由（懒加载 + handle 权限 key）

contrib/                                  # 不改动（HMAC/AES 均为标准库能力）
```

**Structure Decision**: 沿用 iam-service 现行分层（service 直接持有 `*gorm.DB`，无独立 repository/handler 层）、idl proto 契约先行、frontend-service/iam-web 页面模式；凭证为认证管理域，前端页面归入 AuthenticationManagement 目录与菜单。
