---
status: superseded
superseded_by: 0008-admitted-app-data-scope
---

# 登录即锁定应用：单工程双控制台，取消运行时切换

> ⚠️ **已取代（2026-11-19）**：本文的「登录锁定应用」「无运行时切换」「双登录页 + 机机代登录」三条裁决已被 ADR-0008 推翻——改为统一 IAM 登录 → 选控制台 → 左上角切换数据范围；sender 登录端点与机机代登录链路整体删除。「单工程双控制台」的工程结构保留。以下保留原文作为历史记录。

> 2026-11-18 用户裁决。取代 ADR-0006「功能边界零改动」一条：`ListMyApplications/SelectApplication` 及 Redis `app:ctx:{jti}` 运行时映射退役，应用上下文在登录时固化为签名 JWT claim。

多应用体系原有两套边界：ADR-0006 的所属应用（数据边界）与准入应用（功能边界）。此前用户在控制台内可经 `SelectApplication` 切换当前应用，服务端用 `app:ctx:{jti}` 映射承载切换结果。本裁决把"当前应用"从会话可变状态改为登录时一次性确定的不可变属性，并以单工程双控制台（IAM / sender）提供各应用的独立入口。

## 决策（2026-11-18 用户确认）

- **登录锁定**：`PasswordAuth` 的目标应用不再由调用方自报：人机登录缺省 `iam`；机机代登录由 AK/SK 验签后的服务账号所属应用推导。服务端按 code 查启用应用（不存在/停用 → 拒绝），再做**用户准入校验**（fail-closed）：admin 旁路；非 admin 须所属应用（home，隐式准入）或 `t_r_user_application` 绑定，否则 `1001063 PermissionDenied`。解析得到的 `application_id` 写入签名 JWT claim。
- **不可变上下文**：拦截器直接取 token claim 的 `application_id`，不再查 Redis `app:ctx:{jti}`。`ListMyApplications`、`SelectApplication`、`GetMyProfile.applications`、`AppContextTTL/GetAppContextKey` 及其测试/契约全部移除；切换应用必须重新登录。
- **双入口**：单一前端 `admin-web` 提供 `/login`（IAM）与 `/sender/login`（sender）两个登录页。sender 宿主自身无用户库，其登录端点 `POST /sender/authentication/password-auth` 委托 IAM `PasswordAuth`；出站携带 AK/SK 机机账号签名，IAM 验签后从服务账号所属应用推导目标应用（不再读取 `application_code` 配置），仅透传 IAM 签发的 token 并回写 cookie。
- **控制台隔离**：前端 `app_console`（iam/sender）决定菜单/字典 API 前缀（`/iam/...` 或 `/sender/...`）、React Query 缓存键与 401 回跳登录页；登出清除控制台标记回 IAM 登录页。sender 业务路由在 `/sender` 子树，与菜单种子 path 保持一致。
- **Profile 语义**：`GetMyProfile` 仅返回基本信息、`home_application_id` 与 `is_admin`，不再返回可切换应用清单。

## 补充裁决：内部服务调用必须走机机账号（2026-11-18 21:26 用户确认）

- 早期实现让 sender 用 metadata `application_code=sender` 声明目标应用，等于把“应用身份”交给调用方自报。任何能连 IAM gRPC 的内部服务都能改写这个 code，配置里的 `application_id`/`application_code` 不是安全凭证，不能作为授权依据。该路径已由 AK/SK 机机账号替换。
- **机机账号**：sender → IAM 的登录委托属于机机调用，必须携带 AK/SK 服务账号签名（ADR-0004 的服务账号轨）。IAM 验签后从该服务账号绑定的应用推导目标 `application_id/code`，忽略调用方传入的 `application_code`。机机账号只证明“调用方是 sender 服务”，不代替用户凭据；用户密码校验与准入校验仍在 IAM 完成。
- **两轨正交**：入站用户请求仍透传用户 JWT 给 IAM `CheckPermission` 做授权（ADR-0005）；出站机机调用用 AK/SK 证明服务身份。前者回答“谁在用”，后者回答“谁在调”，不可互相替代。
- **机机角色的权限码范围（2026-11-18 21:29 用户追问后澄清；21:37 收敛为最小实现）**：
  - 本期**不新增权限码、不拆独立 RPC**：`CheckPermission` 与人机 `PasswordAuth` 均保持无权限码。机机代登录复用 `PasswordAuth`，由 AK/SK 验签区分调用身份。
  - `CheckPermission` 不挂权限码。它是判权原语：sender 入站透传的是**用户 JWT**，要求权限码会让用户先持有“判权权限”才能判权（自举），改用机机账号判权则变成服务身份授权，违背 ADR-0005。AK/SK 验签通过即允许调用，无需角色勾选。
  - 机机账号亦不勾 `sender.*` 等业务码——入站判权用的是用户 JWT，不是机机账号；若将来机机账号需要以服务身份执行业务操作，再单列权限码。
- **仍需补齐**：把应用锁定延伸到 API 边界——IAM 验签后须校验 token claim 的 `application_id` 与宿主 appId 一致；现有 AK/SK 分支尚未注入应用上下文。此校验与机机账号是两道独立边界。

## Considered Options

- **A. 保留 `SelectApplication` 的运行时切换**：一次登录可进入多个应用，但应用上下文变成 JWT 之外的第二数据源，需 Redis 映射与令牌同步失效，且同一 token 的可见数据随请求漂移，审计与缓存复杂。否决。
- **B. 每个宿主独立用户体系**：sender 自建账号与密码校验。否决—— IAM 是唯一身份权威，复制用户库违反单一数据源原则。
- **C（选定）. 登录锁定 + 宿主独立登录入口**：应用身份进入签名声明、不可篡改；宿主只负责选择自身 code 并转发凭据，不持有用户数据；代价是跨应用需重新登录。

## Consequences

- 非 admin 若对目标应用既非所属、也无准入绑定，登录直接失败（而非登录后看到空菜单）；admin 不受准入限制。
- 既有使用旧流程的客户端须改投相应宿主登录端点；IAM 自有控制台登录不带 AK/SK 时默认锁定 iam 应用。sender 服务须在 `conf/config.yaml` 中配置 `sender.iam_access_key` / `sender.iam_secret_key`（部署时填写，仓库示例留空；缺失即启动失败），凭证所属应用决定代登录目标应用。
- 应用切换需重新登录，令牌有效期内不存在上下文漂移；缓存键按控制台隔离，避免跨控制台串数据。
- ADR-0006 的数据边界（home）与功能边界（准入/权限码）继续有效，只是准入的行使时点前移到登录；账号域 home 隔离逻辑不变。
- 服务间调用不再依赖调用方自报 `application_code`：sender 代登录须用 AK/SK 机机账号，由 IAM 从服务账号推导应用；sender 入站透传用户 JWT 判权的机制不变（ADR-0004/0005 + 补充裁决）。
- 机机账号仅约束服务身份，不阻止持有 `sender.*` 权限码的用户用其他控制台的 token 调 sender API；该缺口需由 API 层的 `application_id` 校验封闭，属于待落地项。
