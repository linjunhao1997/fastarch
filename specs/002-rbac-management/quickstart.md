# Quickstart: RBAC 基础管理（002）端到端验证

Phase 1 产出。验证日期基准：2026-09-05

本指南验证 spec 的核心验收场景：三域增查列改 + 新增删除（引用约束、二次确认、并发删除提示）、角色-权限配置（含早退 bug 回归）、账号-角色绑定（幂等、角色存在校验、生效时机）。契约细节见 [contracts/account-service.proto](contracts/account-service.proto) 与 [contracts/authorization-service.proto](contracts/authorization-service.proto)，数据模型见 [data-model.md](data-model.md)。

## 前置条件

1. MySQL、Redis 可用（配置见 `iam-service/conf/`）
2. 已安装 Go 1.23、Node.js、`buf`
3. 已有可登录且具备管理权限的账号（现有认证体系；非 admin 账号需先授予对应权限码）
4. **存量数据检查**（唯一索引迁移的前置条件，D2）：AutoMigrate 建唯一索引会因存量重复数据失败，先执行检查，如有重复须先人工合并/清理：

```sql
SELECT email, COUNT(*) c FROM t_account GROUP BY email HAVING c > 1;
SELECT code,  COUNT(*) c FROM t_role   GROUP BY code  HAVING c > 1;
SELECT code,  COUNT(*) c FROM t_permission GROUP BY code HAVING c > 1;
SELECT account_id, role_id, COUNT(*) c FROM t_r_account_role GROUP BY account_id, role_id HAVING c > 1;
```

## 步骤 1：生成契约代码

```bash
cd idl/proto
./protoc.sh          # buf dep update && buf generate → 更新 idl/pb/ 与 idl/openapiv2/
```

预期：`idl/pb/iam/account.pb.go` 出现 `DeleteAccount` / `UpdateAccountRoles`；`idl/pb/iam/authorization.pb.go` 出现 `DeleteRole` / `DeletePermission`；两份 swagger 同步更新。

## 步骤 2：启动服务

```bash
cd iam-service
go run . grpc        # 启动时 AutoMigrate：t_account.email / t_role.code / t_permission.code 唯一索引，
go run . http        # t_role / t_permission 增加 version 列，t_r_account_role 增加 (account_id, role_id) 唯一索引
```

## 步骤 3：服务层单元测试（宪法 VII）

```bash
cd iam-service
go test ./pkg/service/...
```

预期：`account_service_test.go` 与 `authorization_service_test.go` 全部通过，覆盖：邮箱/编码重复拒绝、version 冲突拒绝、三类删除的引用约束与并发删除 NotFound、UpdateRole 仅传权限列表时变更生效（早退 bug 回归）、未知权限码拒绝、绑定幂等与角色不存在拒绝、GetRole/GetAccount 关联列表回填。

## 步骤 4：REST 接口验证

以下 `$TOKEN` 为 admin 登录返回的访问令牌。

### 4.1 唯一性（FR-2/6/10、US1/US2/US3 重复场景）

```bash
# 重复邮箱创建被拒
curl -X POST http://localhost:8080/account/accounts \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"张三","email":"already@existed.com","kind":"0"}'
# 预期：错误响应，提示邮箱已存在

# 重复角色编码被拒
curl -X POST http://localhost:8080/authorization/roles \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"访客","role_code":"guest"}'   # guest 为已存在编码
# 预期：错误响应，提示角色编码重复
```

### 4.2 角色-权限配置（FR-7、US2 场景 3/4/5 + 早退 bug 回归）

```bash
# 仅传权限变更（不传 name/enabledStatus）——回归用例：修复前此调用被静默跳过
curl -X PATCH http://localhost:8080/authorization/roles/{roleId} \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"addedPermissionCodeList":["perm.a","perm.b"],"removedPermissionCodeList":[],"version":1}'
# 预期：200；GET 后 permissionCodeList 包含 perm.a、perm.b

# 增删同时生效
curl -X PATCH http://localhost:8080/authorization/roles/{roleId} \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"addedPermissionCodeList":["perm.c"],"removedPermissionCodeList":["perm.a"],"version":2}'
# 预期：200；GET 后含 perm.b、perm.c，不含 perm.a

# 未知权限编码整体拒绝（FR-19）
curl -X PATCH http://localhost:8080/authorization/roles/{roleId} \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"addedPermissionCodeList":["perm.not-exist"],"version":3}'
# 预期：错误响应，指明编码不存在，且角色权限集合不变

# 乐观锁冲突（FR-18）：用过期 version 重复上一步
# 预期：错误响应，提示"已被他人修改，请刷新后重试"
```

### 4.3 账号-角色绑定（FR-12/13/14、US4）

```bash
# 绑定角色 R 到账号 U
curl -X PATCH http://localhost:8080/account/accounts/{accountId}/roles \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"addedRoleCodeList":["guest"],"removedRoleCodeList":[]}'
# 预期：200；GET /account/accounts/{accountId} 的 roleCodeList 含 "guest"

# 重复绑定幂等（FR-14）
# 再次提交相同 addedRoleCodeList → 预期：200（视为成功），无重复绑定数据

# 绑定不存在的角色被拒（FR-13）
curl -X PATCH http://localhost:8080/account/accounts/{accountId}/roles \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"addedRoleCodeList":["no-such-role"],"removedRoleCodeList":[]}'
# 预期：错误响应，提示角色不存在

# 解绑幂等：对未绑定的编码解绑 → 预期：200

# 生效时机（D6 v2，立即生效）：以账号 U 登录并保持会话，确认其权限包含 guest 所含权限；
# 解绑 guest 后**不重新登录**，U 的下一次受权限保护请求即不再具备 guest 权限（无需重新登录）；
# 重新绑定后，U 无需重新登录即可再次通过对应权限校验

# 读穿回源（D6-②）：删除 U 的权限快照键（redis-cli DEL account:<uid>:permissions）
# 或直接重启 Redis → U 不重新登录继续请求：预期校验正常（从 DB 重建快照），
# 而非鉴权失败；快照键被重建（TTL 5 分钟）

# Redis 故障降级（D12）：停掉 Redis → U 请求：预期得到 fail-closed 的明确错误
# 提示（不得崩溃、不得静默放行、不得裸 Redis 错误透出）
```

### 4.4 删除与引用约束（FR-3/8/11、FR-17、US5）

```bash
# 删除被角色引用的权限 → 预期：错误响应，提示需先解除引用
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/authorization/permissions/{permBId}

# 删除被账号绑定的角色 → 预期：错误响应，提示需先解除绑定
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/authorization/roles/{roleId}

# 删除被绑定角色的账号 → 预期：错误响应，提示需先解除绑定
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/account/accounts/{accountId}

# 解除引用后删除成功：
#   1) UpdateRole 移除 perm.b → Delete permission perm.b → 200
#   2) UpdateAccountRoles 解绑 guest → Delete role → 200，且 t_r_role_permission 中该角色关联已被级联清除
#   3) Delete account → 200

# 并发删除（US5 场景 6）：对同一 id 执行两次 DELETE
# 第二次 → 预期：404，"对象不存在"明确提示
```

## 步骤 5：前端页面验证（SC-003：无模拟数据）

```bash
cd app
npm run dev          # Vite dev :5173
```

以管理员账号登录后：

1. **账号管理**：列表分页翻页结果稳定不重复（Edge Case 1）；行操作"删除"弹二次确认，取消不变、确认后消失；"绑定角色"弹窗内多选角色保存后，重新打开回显正确，并提示"权限变更已生效"
2. **角色管理**：编辑角色可添加/移除权限并保存（US2）；删除未绑定角色成功、被绑定角色被拒并收到明确提示
3. **权限管理**：删除未被引用权限成功、被引用权限被拒并提示
4. **并发**：两个浏览器标签页同时编辑同一对象先后保存，后保存方收到冲突提示（FR-18）
5. **失败提示**：停掉 iam-service 后操作展示明确失败提示，无静默失败、无模拟数据（Edge Case 5、SC-005）
6. **生效即时性（D6 v2）**：为账号 U 绑定角色后，U 已登录会话不重新登录即可访问新授权的功能；解绑后 U 的受保护请求立即被拒
7. **日志可诊断（D13，宪法 X）**：查看 iam-service 日志——每个请求含 `method` / `grpc_code` / `business_code` / `duration_ms` / `trace_id` 结构化字段；触发一次业务错误（如重复邮箱），日志中可见错误码与上下文（账号/角色 id），响应 header 返回 trace-id 与日志可关联；客户端仅见用户可理解消息，无裸 SQL/Redis 错误透出
