# flow-service v1.1 验收记录（QA）

> 验收对象：`4ea3a62f7582fde6e7e3d99e7009a856c42c3d53`（main）
> 验收时间：2026-11-21 18:09 CST ｜ 验收角色：测试（qa）
> 结论：**PASS**。P0（问题重开、证据强校验）与 P1（只读闭环指标）均可用；1 个权限分支（3001025）在单账号环境下未覆盖，已列为残余风险。

## 1. 验收范围

| 项 | 验收点 | 结果 |
| --- | --- | --- |
| 契约 | `ReopenQuestion` / `GetMyMetrics` 已进 proto，pb/gw/openapi 由 buf 生成 | PASS |
| 权限 | 新方法在 `method_permissions.go` 登记，复用 `flow.question.answer` / `flow.task.list` | PASS |
| 重开 | answered→open、`reopen_count+1`、任务派生回 blocked、写 `question.reopen` | PASS |
| 重开保护 | open 问题 3001024；任务已结束 3001026 | PASS |
| 证据 | `done` 返回 3001023；合法文档路径可闭环 | PASS |
| 指标 | `GET /flow/metrics` 字段齐全，数值与任务/问题一致 | PASS |
| 前端 | 问题中心“重开”入口、任务页指标条随构建产物生成 | PASS（构建级） |
| 质量 | `go build/vet/test`、`pnpm build` 全绿 | PASS |

## 2. 构建与测试

```text
go build ./...   exit 0
go vet ./...     exit 0
go test ./...    exit 0  （pkg/service: ok，含 TestValidateEvidence 21 个用例）
pnpm build       exit 0  （tsc -b + vite build，18.92s）
```

## 3. MCP 工具面

`tools/list` 返回 15 个工具，新增：

- `metrics` → `GET /flow/metrics`
- `reopen_question` → `POST /flow/questions/{id}/reopen`

## 4. 指标（qa 角色，验收时）

```json
{
  "roleId": "6",
  "roleName": "测试",
  "openTasks": 1,
  "readyTasks": 1,
  "blockedTasks": 0,
  "openQuestions": 1,
  "answeredQuestions": 0,
  "reopenedQuestions": 0,
  "avgAnswerMinutes": 0,
  "p50AnswerMinutes": 0,
  "reopenRate": 0,
  "selfAnswerRate": 0,
  "stalledTasks": 0
}
```

口径核对：qa 当时有 1 个 open 任务（QA 验收任务）、1 条待答问题（q9），无已闭环问题，故 answered/reopen/avg 均为 0，与库内事实一致。

## 5. 证据门槛回放

| 输入 | 结果 |
| --- | --- |
| `evidence = "done"` | `业务码 3001023：证据格式不合法…`，问题保持 open |
| `evidence = "docs/flow-workbench-implementation.md"` | 闭环成功，q9 变 answered |
| 重开 q9（原提问人） | 成功；`reopenCount=1`，task 14 `state=blocked` |
| 重开 q9（仍为 open） | `业务码 3001024：只有已闭环的问题才能重开` |
| 重开 q7（task 12 已 done） | `业务码 3001026：任务已结束…不能重开其下的问题` |

## 6. 残余风险与未覆盖项

- **3001025（非原提问人且非 boss）未覆盖**：当前环境只有一个账号，所有问题均由同一账号提出。逻辑已按 `asked_by` + `role.code == "boss"` 实现，留待多账号环境补测。
- evidence 只校验形态，不校验 URL 可达性（内网/离线链接会把正常闭环卡死）；可追溯性依赖人审 + 留痕。
- done/discarded 任务不可重开其下问题；需继续跟进时新建任务（避免终态被悄悄翻回）。
- sweep 自动巡检不在 v1.1，已拆 v1.2 独立任务。
- 前端验收到“构建通过 + 组件生成”级；未做浏览器端人工点选回归。

## 7. 复跑命令

```powershell
cd D:\fastarch\flow-service
go build ./...; go vet ./...; go test ./... -count=1

cd D:\fastarch\frontend-service\admin-web
pnpm build

# MCP（AK/SK 只从环境变量取，勿写进仓库）
$h = @{ 'x-fa-access-key' = $env:FLOW_AK; 'x-fa-secret-key' = $env:FLOW_SK }
$body = '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"metrics","arguments":{}}}'
Invoke-RestMethod -Uri 'http://127.0.0.1:8082/flow/mcp' -Method Post -ContentType 'application/json' -Headers $h -Body $body
```
