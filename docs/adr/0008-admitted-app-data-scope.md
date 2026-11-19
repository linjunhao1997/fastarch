---
status: accepted
supersedes: ADR-0006（home 单值数据边界条款）, ADR-0007
---

# 统一登录 + 接入集合数据范围：身份、数据范围、权限码、控制台四者正交

> 2026-11-19 用户裁决，推翻 ADR-0006 的「所属应用 = 唯一数据边界」与 ADR-0007 的「登录锁定应用、无运行时切换」。旧的 home 单值边界无法表达「一个用户可跨多个应用操作各自数据」，旧的登录锁定又把功能入口（控制台）与数据归属绑死。本裁决把二者拆开，并统一登录入口。

用户只需一次登录（IAM 为唯一身份权威），登录后先选择**控制台**（决定功能菜单与 API 前缀），进入后在左上角选择**数据范围**（决定列表/详情过滤哪个应用的数据）。四个概念彻底正交：

- **身份**：签名 JWT 中的 `user_id` / `is_admin`，登录后不可变，回答「你是谁」；
- **数据范围**：请求头 `x-fa-app-code` 选择的应用 code，可运行时切换，回答「你现在看哪家的数据」；
- **权限码**：角色勾选的授权词汇，回答「能做什么操作」；
- **控制台**：编译期的路由子树 + 菜单/字典前缀（iam / sender），登录后锁定，回答「用哪套功能」。

核心安全约束：**授权来自签名，header 只是选择信号**。JWT 签发时把用户的接入应用集合快照进 `admittedAppIds` claim；运行时服务端解析 `x-fa-app-code` → appId，校验其 ∈ 接入集合后才作为数据归属注入上下文。请求方无法靠伪造 header 越权访问未接入应用的数据。

## 决策（2026-11-19 用户确认）

- **统一登录**：退役 sender 登录端点与 `NewAuthenticationServiceWithSignature` 机机代登录链路（ADR-0007 产出的这段代码整体删除）。全平台只有 IAM `/login`；JWT 透传由网关/拦截层完成。401 一律回 IAM 登录页，`?redirect=` 保留深链接。
- **令牌不携带应用上下文**：JWT 不含 `applicationId` claim（`admittedAppIds` 是接入集合快照，不是当前应用）。登录时不再解析 home 的目标应用、不做登录准入校验；菜单/字典归属宿主固定 appId（由各服务配置注入 ctx），数据归属由请求头 appCode 选择，控制台由前端登录后选择。三者互不依赖。
- **接入集合（admitted apps）**：定义为 `home`（隐式准入，物理列保留、语义降级为「默认落地应用」与接入集合成员）+ `t_r_user_application` 显式绑定，去重后一次查库（`loadAdmittedAppIds`）。快照写入 JWT `admittedAppIds []int64`；令牌有效期内接入关系变更不生效（与旧模型「不可变上下文」一致的取舍）。`GetMyProfileResponse.applications`（repeated `ApplicationSummary`）返回同一集合，供左上角下拉渲染；admin 返回全部启用应用。
- **数据范围选择**：前端 `X-Fa-App-Code` 头下发选中 code（网关 `WithMetadata` 透传为 gRPC metadata `x-fa-app-code`）。IAM 入站拦截器（JWT 轨）经 `AppResolver` 把 code 解析为启用应用 id，再用 `IsAppAdmitted` 校验 ∈ claim 集合，通过后才 `WithDataAppId` 注入 ctx；失败 `PermissionDenied`（fail-closed，不降级）。AK/SK 轨的数据归属来自凭证自身 `application_id`（见下），不读 header。
- **统一隔离边界**：所有账号/角色/权限/凭证隔离逻辑经 `operatorDataScope(ctx, db)` 取数据归属：admin 与系统调用（`accountId=0`）旁路；非 admin 优先取拦截器注入的 `dataAppId`，**未注入时回落 DB home**——这是刻意的历史兼容设计，使未带 appCode 的旧调用/内部调用行为不变。查不到用户或结果 ≤ 0 时 fail-closed。
- **凭证 = 归属应用 + 权限来源**：`t_access_credential` 增 `application_id` 列（`credential.proto` 的 `CreateAccessCredentialRequest.application_id=4` 必填、`AccessCredential.application=14` 摘要，迁移 000015 按绑定用户 home 回填存量）。凭证的 `application_id` 决定其 AK/SK 机机调用产生的数据归属；`user_id` 决定权限来源（继承绑定用户的角色）。创建时校验归属应用 ∈ 创建者接入集合，越界 `1001069`。二者正交，解决了旧模型「机机调用数据归谁无字段可依」的硬伤。
- **控制台与数据范围解耦**：菜单/字典仍跟控制台走，用宿主固定 appId（`cfg.Iam.ApplicationId` 注入 ctx），与 appCode、令牌均无关。因此只接入 sender 应用的用户也能进 IAM 控制台管理 sender 数据。前端 `consoleAppId` 与 `dataAppId` 分开命名；`/sender/records`、`/sender/templates` 等路由 path 与菜单种子保持一致。
- **前端三正交 Provider**：`ConsoleProvider`（菜单/字典前缀，路由子树锁定）、`DataScopeProvider`（接入集合下拉、localStorage `data_scope_app_code` 持久化、切换时 `queryClient.clear()` 防串数据）、`PermissionProvider`（权限码）。登录成功后跳控制台选择页 `CONSOLE_REGISTRY`；自描述接口（profile / my-permissions / my-menu-tree）带 `skipDataScope`，避免陈旧 appCode 撞接入集合造成启动死锁。

## Considered Options

- **A（旧，ADR-0006）. home 单值数据边界**：数据边界最纯，但一个用户无法跨应用操作数据，被用户否决。
- **B（旧，ADR-0007）. 登录锁定应用**：应用身份进签名、不可篡改，但把功能入口与数据归属绑死，跨应用须重新登录，且机机调用数据归属无载体。否决。
- **C（选定）. 身份/数据范围/权限码/控制台四者正交**：JWT 管身份且承载接入集合快照，header 仅作受校验的选择信号；灵活性、安全边界、机机归属三者兼得。代价是引入 appCode 解析与 claim 快照的复杂度。

## Consequences

- appCode 未带时回落 home 是刻意兼容设计：旧客户端与内部调用无需改造即可工作，但失去了显式数据范围的审计粒度（审计应记录实际选中的 appCode）。
- 接入集合快照在令牌有效期内不变；用户被新增接入应用后须重新登录才能在下拉中看到，被移除接入的应用在令牌到期前仍可访问其数据。若要求实时吊销，需要改为每次查库校验（放弃无状态 JWT），本期不做。
- 展示用的应用摘要在 sender 侧暂以 code 回填 name（sender 无独立应用目录）；完整应用名待目录服务完善后补齐。
- admin/系统调用旁路数据范围校验，其 `dataAppId` 不注入；依赖 admin 的自身约束，与旧模型一致。
- ADR-0006 中关于角色/权限/凭证/绑定隔离、角色码应用内唯一、超管标志的细节裁决继续有效，只是隔离的「home」口径统一改为本 ADR 的 `operatorDataScope`（优先 dataAppId、回落 home）；ADR-0007 的「单工程双控制台」工程结构保留，登录锁定与机机代登录两条裁决作废。
