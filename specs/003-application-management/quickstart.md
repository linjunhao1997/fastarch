# Quickstart: 应用管理（003）端到端验证

Phase 1 产出。验证日期基准：2026-09-05

本指南验证 spec 的核心验收场景：应用删除（含引用约束、不存在提示、二次确认）及既有 CRUD 未被破坏。契约细节见 [contracts/application-service.proto](contracts/application-service.proto)，数据模型见 [data-model.md](data-model.md)。

## 前置条件

1. MySQL、Redis 可用（配置见 `iam-service/conf/`）
2. 已安装 Go 1.23、Node.js、`buf`
3. 已有可登录且具备管理权限的账号（现有认证体系）

## 步骤 1：生成契约代码

```bash
cd idl/proto
./protoc.sh          # buf dep update && buf generate → 更新 idl/pb/ 与 idl/openapiv2/
```

预期：`idl/pb/iam/application.pb.go` 中出现 `DeleteApplication`；`idl/openapiv2/iam/application.swagger.json` 出现 `delete /application/applications/{id}`。

## 步骤 2：启动服务

```bash
cd iam-service
go run . grpc        # gRPC 进程
go run . http        # grpc-gateway HTTP 进程
```

## 步骤 3：服务层单元测试（宪法 VII）

```bash
cd iam-service
go test ./pkg/service/...
```

预期：`application_service_test.go` 全部通过，覆盖：删除成功 / 删除不存在（rows-affected=0）/ 删除被引用被拒 / 更新不存在 id 被拒。

## 步骤 4：REST 接口验证

以下 `$TOKEN` 为登录接口返回的访问令牌；`{id}` 为步骤 4.1 创建返回的应用 id。

```bash
# 4.1 创建应用
curl -X POST http://localhost:8080/application/applications \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"验证应用","description":"quickstart"}'
# 预期：200，返回含 id 与 app_code 的应用对象

# 4.2 列表可见
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/application/applications?currentPage=1&pageSize=10"
# 预期：200，列表包含"验证应用"

# 4.3 删除不存在的应用（并发冲突模拟）
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/application/applications/999999999
# 预期：404，错误 message 用户可理解

# 4.4 删除真实应用
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/application/applications/{id}
# 预期：200 空体；重复 4.4 得到 404（幂等拒绝）；4.2 列表不再包含该应用

# 4.5 引用约束（需先通过前端/订阅接口构造一条引用该应用的授权数据）
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/application/applications/{被引用的id}
# 预期：非 2xx，message 提示"应用已被引用，请先解除引用"
```

## 步骤 5：前端交互验证

```bash
cd app
npm run dev          # http://localhost:5173
```

1. 登录后进入「应用管理」列表 → 确认数据与后端一致（无模拟数据）
2. 点击某行「删除」→ 弹出确认框 → 取消 → 列表不变
3. 再次点击「删除」→ 确认 → 提示"删除成功"，列表移除该行
4. 对被引用的应用执行删除 → 确认 → 展示后端错误提示（引用约束），列表不变

## 验收对照

| Spec 验收场景 | 对应步骤 |
| --- | --- |
| FR-5 / US4-1 删除未引用应用成功 | 4.4、5-3 |
| FR-6 / US4-2 被引用应用拒绝删除 | 4.5、5-4 |
| FR-10 / US4-6 不存在对象明确提示 | 4.3、4.4 重复执行 |
| FR-7 / US4-5 二次确认 | 5-2 |
| FR-1/2/4 既有 CRUD 未破坏 | 4.1、4.2 |
| SC-003 数据全部真实 | 5-1 |
| 宪法 VII 服务层可自动化验证 | 步骤 3 |
