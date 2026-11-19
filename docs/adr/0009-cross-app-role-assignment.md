---
status: accepted
supersedes: ADR-0006（按码绑定角色以目标用户 home 解析条款）
---

# 跨应用角色分配：超管不切数据范围、绑定按 roleId、角色列表按读侧数据范围隔离

> 2026-11-20 用户裁决。澄清 ADR-0008 落地后暴露的三处不一致：① 超管被 hostAppId 收敛，无法给账号分配 sender 应用角色（"角色码未找到"）；② sender 数据范围的操作者能删掉目标账号上的 iam 角色（removed 路径无校验）；③ sender 操作者在绑定弹窗里能看到 iam 角色码的裸标签。三者同根：**接口用「应用内唯一」的角色码承载跨应用语义，且读写两侧都没有按操作者数据范围做对称隔离**。本裁决把绑定键改为全局唯一 roleId，并把数据范围边界同时贯彻到读（角色列表）写（绑定）两侧。

## 背景与问题

角色码自 000014 起为应用内唯一（复合唯一 `application_id+code`），不同应用可各有同名 `admin`。现状 `UpdateAccountRoles` 按码解析角色：

- 超管（`isAdmin`）→ `operatorDataScope` 旁路 → 码解析按宿主 appId（iam=1）收敛 → **提交 sender 角色码查不到，报 `ErrRoleNotFound`**。超管无法为任何账号分配跨应用角色，与其"能看到全部角色"的界面能力矛盾。
- 非 admin（如 sender 数据范围操作者）→ 码解析按 `dataAppId=2` → 加不进 iam 角色（正确），但 **`removed` 分支不经过码解析与 `requireRoleBindScope`**：它直接用目标账号全量已绑角色 `boundRoles` 按码取 id 删除（`account_service.go:444-449`），导致 sender 能删掉 iam 角色——fail-open 越权。
- 前端 `BindRolesModal` 把目标账号全量 `roleCodeList` 填进多选值（`BindRolesModal.tsx:30`），而下拉 options 只有本应用角色，iam 角色渲染成裸 code tag，用户一关即触发越界 `removed`。

## 决策（2026-11-20 用户确认）

- **超管不切数据范围**：超管在 IAM 控制台不通过左上角切换数据范围，也不下发 `x-fa-app-code`。后端无需为此改动——`operatorDataScope` 对 admin/系统调用本就旁路，注入的 `dataAppId` 对 admin 不参与隔离；菜单/字典用宿主固定 appId，与 appCode 无关。前端对 `isAdmin` 隐藏数据范围切换器，并让请求层不带范围头（见 Consequences 的兼容说明）。
- **绑定键 roleCodeList → roleIds**：`UpdateAccountRolesRequest` 的 `added_role_code_list` / `removed_role_code_list` 改为 `added_role_ids` / `removed_role_ids`（全局唯一的 `t_role.id`），旧字段号/名 `reserved`。角色按 id 加载，同名码歧义从根上消失。超管/系统调用按 id 加载**不加应用过滤**，可为账号分配任意应用角色；非 admin 仍按 `operatorDataScope` 取 `dataAppId`，对每个待绑角色校验 `application_id == dataAppId`，并禁绑 `is_super_admin` 角色。
- **读写两侧对称 fail-closed**：`added` 与 `removed` 走同一套按 id 解析 + `requireRoleBindScope` 校验；任何越界 id（不存在 / 归属不在操作者数据范围 / 超管角色）一律 `ErrRoleOutOfScope`（写侧 fail-closed，不静默忽略）。既杜绝越权删除，也保留审计可追溯性。
- **读侧角色列表隔离**：`Account.role_code_list` 与新增的 `Account.role_ids`（field 16，与前者同源同序，GetAccount / ListAccounts 回填）均按操作者数据范围过滤——非 admin 只返回 `application_id ∈ dataAppId` 的角色；超管返回全部。sender 操作者看不到目标账号的 iam 角色，用户界面上不存在可删的 iam 角色标签（读侧消除误操作入口，与写侧 fail-closed 互补）。新增 `role_ids` 是因为绑定接口改按 id 承载后，同名码无法反查唯一 id，前端须由详情直接回填已绑角色 id。
- **不新增授权维度**：不引入"角色分配管理员"之类的专门角色。分配权继续由 `iam.user.assign_roles` 权限码 + `dataAppId` 边界承载；iam 应用角色仍是操作码载体，跨应用职责靠改键与读写隔离解决，而非新角色类型。
- **前端联动**：`BindRolesModal` 下拉 value 从 `role.roleCode` 改为 `role.id`，提交 `addedRoleIds` / `removedRoleIds`；下拉 options 与已绑初值都来自按范围过滤后的角色列表 / 账号详情，不再出现跨应用裸码标签。

## Considered Options

- **A（选定）. 绑定按 roleId + 读写对称隔离**：全局唯一键消解同名歧义，超管获跨应用分配能力，sender 侧读写都只见本应用角色。代价是 IDL/pb/swagger、前后端绑定链路一次性改造。
- **B. 放弃超管跨应用分配**（超管只能配 iam 角色，sender 角色全权下放 sender 自治）：改动最小，但推翻"超管为各应用配各来源角色"的既定诉求，被否决。
- **C. 超管仅为码解析传 appCode**（不切数据、只为消歧义）：与"超管不传 x-fa-app-code"的裁决直接冲突，且复用同一请求头承载两种语义，被否决。
- **D. 保留按码绑定，仅修 removed 校验**：能堵住越权删除，但超管跨应用分配（"角色码未找到"）无解，问题只修一半，被否决。

## Consequences

- 已签发的令牌 / 已持久化的 `data_scope_app_code` 不受影响：非 admin 的 `dataAppId` 语义不变；admin 本就旁路范围校验，前端额外做到不发头后，即使 localStorage 残留旧 code 也不会造成越权（后端对 admin 不按 dataAppId 隔离）。
- 绑定接口为破坏性变更：`added_role_code_list` / `removed_role_code_list` 退役，`reserved` 旧字段号/名；所有调用方（前端、AK/SK 机机调用、测试）须改为传 roleId。存量绑定数据无需迁移（关联表存的就是 roleId）。
- `Account.role_code_list` 由读侧过滤产生，是"当前操作者视角"的角色，不再是目标账号的全量角色；需要全量的场景（超管审计）由超管身份天然满足。任何依赖该字段做全量判断的内部逻辑须复核。
- ADR-0008 的四正交边界不变：本裁决只在"角色"这一资源上把边界贯彻到读写两侧，身份/数据范围/权限码/控制台的划分与 appCode→dataAppId 的校验链路均不调整。
