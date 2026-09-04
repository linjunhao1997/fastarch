# Quickstart: 认证策略管理（001）端到端验证

Phase 1 产出。验证日期基准：2026-09-05

本指南验证 spec 的核心验收场景：三类页签按类型展示真实数据（US1）、创建含重名拒绝（US2）、编辑含冲突提示（US3）、删除含二次确认与并发提示（US4）。契约细节见 [contracts/auth-policy-service.proto](contracts/auth-policy-service.proto)，数据模型见 [data-model.md](data-model.md)。

## 前置条件

1. MySQL、Redis 可用（配置见 `iam-service/conf/`）
2. 已安装 Go 1.23、Node.js、`buf`
3. 已有可登录且具备管理权限的账号（现有认证体系）

## 步骤 1：生成契约代码

```bash
cd idl/proto
./protoc.sh          # buf dep update && buf generate → 生成 idl/pb/ 与 idl/openapiv2/
```

预期：出现 `idl/pb/iam/auth_policy.pb.go`（含 `AuthenticationPolicyService` 五个 RPC）与 `idl/openapiv2/iam/auth_policy.swagger.json`。

## 步骤 2：启动服务

```bash
cd iam-service
go run . grpc        # gRPC 进程（启动时 AutoMigrate 自动建表 t_authentication_policy）
go run . http        # grpc-gateway HTTP 进程
```

## 步骤 3：服务层单元测试（宪法 VII）

```bash
cd iam-service
go test ./pkg/service/...
```

预期：`authentication_policy_service_test.go` 全部通过，覆盖：按类型过滤列表 / 同类型重名拒绝 / 跨类型同名允许 / 乐观锁冲突拒绝 / 更新或删除不存在 id 返回 NotFound。

## 步骤 4：REST 接口验证

以下 `$TOKEN` 为登录接口返回的访问令牌。

```bash
# 4.1 创建 SSO 策略（US2 场景 1 同理适用于 mfa / otp）
curl -X POST http://localhost:8080/authentication/policies \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"policyType":"sso","name":"企业微信SSO","description":"对接企业微信扫码登录","implementationId":"wecom-oidc"}'
# 预期：200，返回含 id 与 version=1 的策略对象

# 4.2 同类型重名创建被拒（US2 场景 3）
curl -X POST http://localhost:8080/authentication/policies \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"policyType":"sso","name":"企业微信SSO","description":"重复名称","implementationId":"wecom-oidc"}'
# 预期：错误响应，message 为"该策略类型下已存在同名策略"类可理解提示

# 4.3 跨类型同名允许（FR-4 判定范围是"同一策略类型下"）
curl -X POST http://localhost:8080/authentication/policies \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"policyType":"mfa","name":"企业微信SSO","description":"MFA 类型同名策略","implementationId":"totp"}'
# 预期：200，创建成功

# 4.4 非法策略类型被拒（Edge Case：无效的认证策略类型）
curl -X POST http://localhost:8080/authentication/policies \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"policyType":"abc","name":"X","description":"d","implementationId":"i"}'
# 预期：校验失败，错误提示类型不合法

# 4.5 按类型过滤列表（US1：页签隔离）
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/authentication/policies?policyType=sso&currentPage=1&pageSize=10"
# 预期：200，仅含 sso 类型策略
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/authentication/policies?policyType=otp&currentPage=1&pageSize=10"
# 预期：200，空列表（US1 场景 3：空状态而非报错）

# 4.6 乐观锁冲突被拒（Edge Case：并发编辑）
# 记下 4.1 返回的 id 与 version（此时 version=1）。先正常更新一次使 version 变为 2：
curl -X PATCH http://localhost:8080/authentication/policies/{id} \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"企业微信SSO","description":"第一次修改","implementationId":"wecom-oidc","version":1}'
# 预期：200。再用过期的 version=1 提交：
curl -X PATCH http://localhost:8080/authentication/policies/{id} \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"企业微信SSO","description":"过期版本修改","implementationId":"wecom-oidc","version":1}'
# 预期：错误响应，提示"策略已被他人修改，请刷新后重试"，且描述仍为"第一次修改"

# 4.7 更新/删除不存在的策略（US3 场景 3 / US4 场景 3：并发删除竞态）
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/authentication/policies/999999999
# 预期：404，message 用户可理解
```

## 步骤 5：前端页面验证（SC-003：无模拟数据）

```bash
cd app
npm run dev          # Vite dev :5173
```

以管理员账号登录后进入"认证管理 → 认证策略管理"：

1. **列表**：三个页签各自展示对应类型且仅展示对应类型的策略（US1）；切换页签无需刷新整页；无策略的类型展示空状态
2. **创建**：任一页签点"新增"→ 填写名称/描述/实现方式标识 → 保存 → 返回列表可见该策略（US2）；名称或描述超长时提交前出现校验提示（FR-7）
3. **编辑**：修改描述并保存 → 列表展示新描述（US3）；用两个浏览器标签页同时编辑同一条策略先后保存，后保存方收到冲突提示（Edge Case）
4. **删除**：点删除 → 弹出确认框 → 取消则数据不变、确认后策略从列表消失且刷新后仍不存在（US4）

预期：以上全部操作数据均来自后端真实存储；断开 iam-service 后操作应展示明确失败提示，而非静默失败或残留模拟数据（Edge Case、SC-005）。
