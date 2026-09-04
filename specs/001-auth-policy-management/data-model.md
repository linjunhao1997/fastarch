# Data Model: 认证策略管理（001）

Phase 1 产出。日期：2026-09-05

## 实体

### AuthenticationPolicy（认证策略）— 新增实体

表：`t_authentication_policy`（`iam-service/pkg/model/authentication_policy.go`）

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | int64 | PK，创建时由 `krand.NewID()` 生成 | 唯一标识，对外以字符串呈现 |
| type | string | `sso` / `mfa` / `otp`；联合唯一索引 `uk_type_name(type, name)` 之一 | 策略类型，**创建后不可变**（FR-5，契约层不提供更新字段） |
| name | string | varchar(64)，非空；`uk_type_name` 之二 | 策略名称，同类型下唯一（FR-4） |
| description | string | varchar(255)，非空 | 策略描述 |
| implementation_id | string | varchar(64)，非空 | 实现方式标识（本特性仅保存与展示，不校验其指向对象） |
| version | int64 | 创建时置 1，每次更新 +1 | 乐观锁版本号（D4） |
| create_time / update_time | time.Time | — | 时间戳 |

> 契约层 `AuthenticationPolicy { id, policy_type, name, description, implementation_id, version }`；时间戳仅存 DB，不出契约（与既有实体契约风格一致——契约只暴露前端需要的字段）。

### PolicyType（策略类型）— 值域而非实体

固定三种：`sso`（单点登录认证）、`mfa`（多重身份认证）、`otp`（一次性密码认证）。以契约白名单 + 前端页签硬编码存在，不落表（spec Assumptions：当前固定集合，扩展另行立项）。

## 关系

无外键、无从表引用（本特性为叶子实体；策略与认证实现（implementation_id 指向对象）的关联属后续特性）。删除无引用约束检查（与 Application 不同）。

## 状态转换

```text
              创建 CreateAuthenticationPolicy
              （type 创建后终身绑定，version=1）
                        │
                        ▼
                  ┌───────────┐  UpdateAuthenticationPolicy
                  │ 存活策略   │  (name/description/implementation_id,
                  │ (version n)│   需携带 version=n；成功后 version=n+1)
                  └───────────┘
                        │
       ┌────────────────┼─────────────────────┐
       │                │                     │
       ▼                ▼                     ▼
  version 不匹配    id 不存在          DeleteAuthenticationPolicy
  → 409 语义冲突    → 404 不存在       （前端 Modal.confirm 二次确认）
  "已被他人修改，    （含并发删除        → 物理删除，不可查询
   请刷新后重试"      竞态场景）
```

## 校验规则汇总

| 规则 | 来源 | 实施位置 |
| --- | --- | --- |
| `policy_type` ∈ {sso, mfa, otp} | FR-1 / Edge Case | proto `buf.validate.string.in`（生成层自动校验，对应"无效的认证策略类型"） |
| name / description / implementation_id 必填 | FR-3 | proto `buf.validate`（Create/Update 均约束）+ 前端 antd Form rules（提交前，FR-7） |
| name ≤ 64、description ≤ 255、implementation_id ≤ 64 | FR-7 | proto `buf.validate` + 前端 Form rules（双侧同值，D6） |
| 同类型下名称唯一 | FR-4 | service 层前置查重（`ErrPolicyNameDuplicated`）+ DB 联合唯一索引 `uk_type_name` 兜底（D3） |
| 更新必须携带匹配 version | Edge Case（并发编辑） | service 层 `WHERE id AND version` 乐观锁，失败返回 `ErrPolicyConflict`（D4） |
| 更新/删除目标必须存在 | FR-10 / US3 场景 3 / US4 场景 3 | service 层 First 检查 + `RowsAffected` 检查（D5） |
| 删除前二次确认 | FR-6 | 前端 `Modal.confirm` |
| 策略类型不可变 | FR-5 | Update 契约不提供 `policy_type` 字段（D5） |
