# Quickstart: AK/SK 签名鉴权凭证管理 — 验证指南

**Feature**: specs/009-ak-sk-auth | **前置**: 阅读 [spec.md](./spec.md)、[contracts/ak-sk-signature-spec.md](./contracts/ak-sk-signature-spec.md)

## 前置条件

1. `IAM_CREDENTIAL_ENCRYPT_KEY` 环境变量已注入（32 字节 hex），否则 iam-service 启动时报配置错误（预期行为）。
2. iam-service（grpc + http）与前端 iam-web 已启动；MySQL/Redis 可用。
3. 测试者具备管理员账号（权限全量），且存在至少一个普通用户（如"张三"，含基本角色授权）。

## 场景 A：凭证创建与一次性 SK 展示（US1 / SC-001）

1. 以管理员登录 iam-web → 认证管理 → 访问凭证。
2. 点击"新建"，选择用户"张三"，填写备注与（可选）过期时间，提交。
3. **预期**：弹窗进入成功步骤，醒目展示 AK 与明文 SK（含"仅展示一次"警示 + 复制按钮）；关闭后列表出现该凭证，SK 位置不再可见；`created_by/create_time` 等审计字段正确且非手输。
4. 为不存在/已停用用户创建 → **预期**：拒绝并提示"用户不存在或已停用"，列表无新增（FR-004）。

## 场景 B：签名调用验签（US2 / SC-002）

1. 按签名规范用 AK/SK 签名调用任一"张三"已授权的接口（如 `GET /account/accounts`）。
   **预期**：200，返回数据；`t_access_credential.last_used_time` 在 1 分钟内更新（节流）。
2. 篡改签名一位 → **预期**：拒绝，reason=`SIGNATURE_MISMATCH`。
3. 用错误 SK 计算 → 同上拒绝，响应不含任何 SK 信息。
4. 将 `X-Fa-Timestamp` 改为 10 分钟前 → `TIMESTAMP_EXPIRED`。
5. 相同 AK+nonce 原样重放已成功的请求 → `NONCE_REPLAY`（新 nonce 重签则正常）。
6. 用未授权给"张三"的接口签名调用 → 验签通过但 `PERMISSION_DENIED`（证明权限由绑定用户决定，FR-015）。

## 场景 C：列表与筛选（US3）

1. 制造 ≥2 个用户的凭证后打开列表：分页、总数正确；按用户筛选仅出该用户凭证；按 AK 前缀、状态筛选生效；翻页后筛选条件保留。
2. 清空筛选无数据时（可用新环境）→ 空状态友好提示。

## 场景 D：启停实时性（US4 / SC-003）

1. 对启用凭证执行禁用（二次确认弹窗说明影响）→ 立即用其签名调用 → **预期**：`KEY_DISABLED`（秒级内生效）。
2. 重新启用 → 签名调用恢复 200。
3. 观察结构化日志：包含 AK、动作、操作人、traceId（FR-014a / 审计）。

## 场景 E：删除与重置（US5）

1. 重置 SK：确认弹窗 → 新 SK 一次性展示 → 旧 SK 签名调用立即 `SIGNATURE_MISMATCH`，新 SK 正常；AK 不变（FR-008）。
2. 删除凭证：确认弹窗 → 列表消失 → 旧签名调用 `KEY_UNKNOWN`；审计日志含操作人与凭证 AK（FR-007）。
3. 为"张三"创建 20 条凭证后第 21 条 → 拒绝并提示上限（FR-010）。

## 场景 F：双轨共存回归（FR-010a）

1. 浏览器登录态（cookie）继续正常访问全部管理页面（回归：JWT 轨道未受签名分支影响）。
2. 同一接口分别用登录态与签名方式调用均成功——两轨任一通过即可。

## 自动化测试映射

| 场景 | 测试落点 |
| --- | --- |
| B1–B6 全部验签分支 | `pkg/auth/signature_test.go`（纯函数向量）+ `auth_interceptor_test.go`（拦截器分支） |
| A/C/D/E 业务规则 | `pkg/service/access_credential_service_test.go`（内存 SQLite + miniredis，沿用 `setupAccountTestDB` 模式） |
| 模型/迁移/种子 | `pkg/model/access_credential_test.go`（AutoMigrate、唯一索引、审计填充） |
| F 双轨回归 | `pkg/auth/auth_interceptor_test.go` 增补 JWT 分支不回归用例 |
