# Data Model: AK/SK 签名鉴权凭证管理

**Feature**: specs/009-ak-sk-auth | **Date**: 2026-09-12

## 实体：AccessCredential（访问凭证）

**模型归类**（宪法 XIV 显式声明）：**基础架构型模型**（治理配置数据）——不强制软删除，允许物理删除；删除前应用层引用完整性校验 + 审计日志。

### 表：`t_access_credential`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | bigint | PK，雪花 ID（krand.NewID） | 主键 |
| `access_key` | varchar(64) | **唯一索引** `uk_access_key` | AK，公开标识，`FAK` 前缀，创建后不可变 |
| `secret_key_cipher` | varchar(256) | NOT NULL | SK 的 AES-256-GCM 密文（随机 nonce 前置拼接），永不返回 |
| `account_id` | bigint | NOT NULL，**普通索引** `idx_account_id` | 绑定用户 ID，业务层维护关联（无物理外键） |
| `remark` | varchar(255) | NOT NULL DEFAULT '' | 备注用途说明，可编辑 |
| `status` | tinyint | NOT NULL DEFAULT 1 | 1=启用 2=禁用 |
| `expire_time` | datetime | NULL | 过期时间，NULL=永不过期 |
| `last_used_time` | datetime | NULL | 最后验签成功时间（节流更新，精度分钟） |
| `version` | bigint | NOT NULL DEFAULT 0 | 乐观锁版本号 |
| `create_time` | datetime | NOT NULL，插入自动填充 | 审计字段 |
| `update_time` | datetime | NOT NULL，更新自动更新 | 审计字段 |
| `created_by` | varchar(64) | NOT NULL，业务代码赋值 | 审计字段（服务端注入，请求体禁止携带） |
| `updated_by` | varchar(64) | NOT NULL，业务代码赋值 | 审计字段 |

- 无 `is_deleted`（基础架构型豁免）。
- 生成器注意：`//go:generate contrib gen_gorm_mysql_repository` 后需**手工补回 uniqueIndex 列常量**（项目已知问题）。
- 查询一律使用字段常量（禁止 `SELECT *`，宪法 XIV 六）。

### 关系

- `AccessCredential.account_id → Account.id`：N 凭证 : 1 用户，业务层维护。单用户上限 20（配置项）。
- 账户删除的引用校验：删除用户前必须确认名下凭证数为 0（引用完整性检查点追加到既有账户删除校验）。

### 状态机

```text
            创建(Create)                
  [不存在] ──────────────→ [启用 status=1]
                              │  │
              ChangeStatus(2) │  │ ChangeStatus(1)
                              ▼  ▲
                          [禁用 status=2]
                              │
       ResetSecret(任一状态下) │→ SK 轮换，状态不变，AK 不变
       Delete(任一状态下)      │→ 物理删除，[不存在]
       expire_time 已过        →→ 任何状态下验签均拒绝（状态字段不翻转，验签时判定）
```

约束：启用/禁用为幂等动作（重复设置同状态返回成功）；删除幂等（二次删除返回 NotFound 语义错误，不视为异常）。

### 契约字段映射（proto `AccessCredential`）

| proto 字段 | 来源 | 备注 |
| --- | --- | --- |
| `id` | id | 字符串化（雪花 ID 超 JS 安全整数，前端以 string 处理） |
| `access_key` | access_key | 明文展示 |
| `account_id` / `account_name` | 关联查询 | 列表展示绑定用户（批量 IN 查询内存组装，禁 N+1） |
| `remark` / `status` / `expire_time` | 同名 | 全量展示（宪法 XII） |
| `last_used_time` / `create_time` / `update_time` / `created_by` / `updated_by` | 审计字段 | 只读展示，请求体禁止携带 |
| `secret_key` | 仅 Create/ResetSecret 响应 | **一次性明文**；列表/详情响应不含此字段 |

## 派生数据（Redis，可重建，非权威）

| 键 | 类型 | TTL | 用途 |
| --- | --- | --- | --- |
| `aksk:nonce:<ak>:<nonce>` | string (SETNX) | 时间戳窗口（默认 300s） | 防重放去重 |
| `aksk:lastused:<ak>` | string (SETNX) | 60s | last_used_time 节流标记 |
| `account:<id>:permissions` | 既有权限快照 | 既有策略 | 验签通过后的权限判定（复用，不新建体系） |

## 配置项（conf/config.go 追加 `Credential` 段）

| 配置 | 默认值 | 环境变量优先 |
| --- | --- | --- |
| `timestamp_window_seconds` | 300 | — |
| `max_credentials_per_account` | 20 | — |
| `encrypt_key` | — | `IAM_CREDENTIAL_ENCRYPT_KEY`（32 字节 hex/base64，必填，缺失时服务启动报错） |

## 业务错误（pkg/service/service_error.go，1001XXX 系列）

| 错误 | 语义 | gRPC 码 |
| --- | --- | --- |
| `ErrAccessCredentialNotFound` | 凭证不存在 | NotFound |
| `ErrAccessCredentialAccountInvalid` | 绑定用户不存在或已停用 | FailedPrecondition |
| `ErrAccessCredentialLimitExceeded` | 超出单用户凭证上限 | ResourceExhausted |
| `ErrAccessCredentialSkDecrypt` | SK 解密失败（配置/数据异常） | Internal |
| （验签路径错误）`ErrSignatureInvalid` / `ErrSignatureExpired` / `ErrSignatureReplay` / `ErrSignatureKeyDisabled` / `ErrSignatureKeyExpired` / `ErrSignatureKeyUnknown` | pkg/auth 域，语义化区分（FR-014） | Unauthenticated |
