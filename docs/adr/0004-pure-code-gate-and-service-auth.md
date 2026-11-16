# 菜单纯码门槛（角色绑定退役）+ 服务间鉴权用 AK/SK 服务账号

两项裁决，简化菜单可见性与服务间调用。

## 一、菜单纯码门槛，t_r_role_menu 退役

ADR 0002 把菜单归为纯展示配置（permission_code 为前端展示门槛）。最初保留"服务端角色绑定过滤（基础层）+ 前端码门槛（额外层）"两层。本裁决**砍掉角色绑定基础层**：

- `my-menu-tree` 返回当前应用**全量启用菜单**，服务端不做角色过滤、不区分 admin。
- 可见性**纯由前端**按菜单 `permission_code` 与用户快照码（GetMyPermissions）过滤。
- `t_r_role_menu` 退役（迁移 000011 删表）；contrib 菜单移除 `UpdateRoleMenus`/`ListRoleMenus` RPC 与 `RoleIdsForAccount`/`RoleExists`/`IsAdminFromContext` 钩子；iam 角色页移除菜单绑定 UI。
- **admin 可见性配套**：`GetMyPermissions` 对 admin 返回全量启用码（与 CheckPermission/拦截器 admin 旁路一致），前端码门槛对 admin 全放行，跨应用码（sender.* 等）亦覆盖。

### 为什么

B 之下码门槛已按权限控显隐：有码的菜单已被限制到持码用户（码经角色直授），角色绑定基本冗余，却招来"宿主查 IAM 角色"的跨服务依赖。砍掉后菜单可见性零角色查询，最简，且符合"菜单归展示、码归授权"。

## 二、服务间鉴权用 AK/SK 服务账号（回滚 FederationService）

曾为 sender 查 IAM 角色新建 `iam.v1.FederationService`（app_id+secret 自鉴权）。**已回滚**——过度工程：

- IAM 已有角色查询接口（GetRole/ListRoles/GetAccount）；AK/SK（specs/009）已是现成的机器间鉴权。
- 服务间调用走**服务账号模式**：IAM 建 service-account 用户（如 `sender-service`），授所需码，发 AK/SK；调用方签名 → 现有拦截器签名分支验签 → 按服务账号快照判权 → 调现有接口。零新鉴权 machinery、零新契约。
- 纯码门槛后菜单已不需要角色查询；AK/SK 服务账号留给真正需要的跨服务调用（如 sender 取用户联系方式、第二步通知互调）。

### 为什么

复用既有接口与 AK/SK，而非另造应用凭证鉴权。FederationService 的 app_id+secret 是 reinvent the wheel；AK/SK 挂用户、签名防重放，本就是为机器调用设计。

## Consequences

- contrib 菜单 Config 仅剩 `ApplicationIdFromContext`/`AccountIdFromContext`/`ValidatePermissionCode`，宿主接入更简。
- sender 菜单宿主无需角色钩子；跨服务调用经 AK/SK 服务账号 + 现有 IAM 接口。
- 迁移 000010（sender 应用 + sender.* 码种子）保留；000011 删 t_r_role_menu。
- 非 admin 用户看菜单需角色直授相应页面查看码（码门槛）；admin 全放行。
