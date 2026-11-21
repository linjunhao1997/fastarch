# flow-service v1.1 上线巡检记录（OPS）

> 对象：commit `4ea3a62f7582fde6e7e3d99e7009a856c42c3d53`（flow-service v1.1）
> 时间：2026-11-21 18:11 CST ｜ 角色：运维（ops）
> 结论：**健康**。8082/9092 监听正常、MCP 15 个工具可用、日志无 ERROR/PANIC、库表齐全；回滚包已就位。

## 1. 进程与端口

| 端口 | 进程 | 命令行 |
| --- | --- | --- |
| 9092（gRPC） | `flow-grpc.exe` PID 10168 | `...\Temp\flow-grpc.exe grpc -c D:\fastarch\flow-service\conf\config.yaml` |
| 8082（HTTP + MCP） | `flow-grpc.exe` PID 15336 | `...\Temp\flow-grpc.exe http -c D:\fastarch\flow-service\conf\config.yaml` |

两个进程工作目录均为 `D:\fastarch\flow-service`，使用同一份二进制、同一份配置。

## 2. 健康检查

- `POST /flow/mcp` → `tools/list` 返回 **15** 个工具（含 `metrics`、`reopen_question`）。
- `metrics`（ops 角色）返回：openTasks=1、readyTasks=1、blockedTasks=0、openQuestions=0、answeredQuestions=0，字段齐全。
- `flow-grpc.err.log` / `flow-http.err.log` 无 `ERROR` / `PANIC` / `FATAL`。
- `flow_dev` 库表：`t_task`、`t_question`、`t_event`、`t_member`、`t_work_role`、`t_r_member_role`、`t_r_task_question`、菜单/字典表、`schema_migrations` 均在；v1.1 **无 schema 变更**，未新增迁移。

## 3. 回滚步骤（3 步）

```powershell
# 1) 停掉当前 grpc/http 两个进程
$pids = Get-NetTCPConnection -State Listen -LocalPort 8082,9092 |
        Select-Object -ExpandProperty OwningProcess -Unique
Stop-Process -Id $pids -Force

# 2) 用上一版二进制覆盖（v1.0 备份在 .tmp）
Copy-Item D:\fastarch\.tmp\flow-grpc-prev.exe "$env:TEMP\flow-grpc.exe" -Force

# 3) 重新拉起两个进程
Start-Process "$env:TEMP\flow-grpc.exe" -ArgumentList 'grpc','-c','D:\fastarch\flow-service\conf\config.yaml' -WorkingDirectory D:\fastarch\flow-service
Start-Process "$env:TEMP\flow-grpc.exe" -ArgumentList 'http','-c','D:\fastarch\flow-service\conf\config.yaml' -WorkingDirectory D:\fastarch\flow-service
```

回滚后验证：8082/9092 重新监听；`tools/list` 回到 12 个工具（无 metrics/reopen_question），确认旧版生效。

## 4. 值班注意事项

- 重启只影响网关与 MCP，不改库；v1.1 无数据迁移，回滚无需回退数据。
- 若 8082 启动失败，先看 `.tmp\flow-http.err.log`；常见原因是端口占用或 IAM `:9090` 未起（判权 fail-closed）。
- MCP 客户端若仍显示 12 个工具，是平台侧工具缓存，不是服务端问题；重启客户端或重新拉 `tools/list` 即可。
- `metrics` 只读；`reopen` 会改变问题状态，值班不要代替业务角色操作，除非老板授权。
