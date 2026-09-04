# Data Model: 应用管理（003）

Phase 1 产出。日期：2026-09-05

## 实体

### Application（应用）— 既有实体，本特性不改动表结构

表：`t_application`（`iam-service/pkg/model/application.go`）

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | int64 | PK，创建时由 `krand.NewID()` 生成 | 唯一标识，对外以字符串呈现 |
| code | string | 创建时取值 = id 字符串，**不可变**（FR-3） | 应用编码，授权数据的引用纽带 |
| name | string | 唯一（FR-2） | 应用名称 |
| description | string | 可空 | 描述 |
| status | string | `enabled` / `disabled`（经 `boolToStatus` 映射） | 启用状态（FR-4） |
| kind | string | — | 预留 |
| create_time / update_time | time.Time | — | 时间戳 |

> 契约层 `Application { id, name, app_code, description, enabled_status }`，映射关系见 `applicationToPb`。

### 引用关系（只读判定，不建外键）

以下从表以应用编码引用 Application（D2），删除时逐表检查：

| 从表 | 引用字段 | 来源域 |
| --- | --- | --- |
| 权限订阅相关表 | `application_code`（申请方/审批方） | 授权域 |
| 其他持有应用引用字段的表 | 实现时以 `pkg/model` 全集为准，集中在 service 层列明 | — |

## 状态转换

```text
            创建 CreateApplication
                 │
                 ▼
           ┌──────────┐   UpdateApplication(enabled_status=disabled)
           │ enabled  │ ────────────────────────────────┐
           └──────────┘ ◄────────────────────────────────┘
                 │                          ┌───────────┐
                 │ UpdateApplication        │ disabled  │
                 │ (name/description)       └───────────┘
                 │ (任意状态下均可)
                 ▼
      无引用？──是──► DeleteApplication ──► 【已删除】（物理删除，不可查询）
                 │
                 否
                 ▼
            拒绝并提示"应用已被引用，请先解除引用"
```

## 校验规则汇总

| 规则 | 来源 | 实施位置 |
| --- | --- | --- |
| 删除请求 id 必填 | FR/契约 | proto `buf.validate.field.required`（生成层自动校验） |
| 删除目标必须存在 | FR-10 | service 层 `RowsAffected` 检查 |
| 删除目标必须无引用 | FR-6 | service 层引用检查（D2） |
| 更新目标必须存在 | 宪法 IV 补强 | service 层 `RowsAffected` 检查（D4） |
| 名称唯一 / 编码不可变 | FR-2 / FR-3 | 既有 Create/Update 行为，本特性不改动 |
