# 企业首页展示后台 · 发布方案（release_plan）

## 1. 发布物

| 组件 | 内容 | 端口 |
| --- | --- | --- |
| idl | `home.v1` proto + 生成的 pb/gateway/swagger | — |
| iam-service | 迁移 000021：home 应用、权限码、home_admin 角色、接入关系 | 8080 / 9090 |
| home-service | 新 Go 服务（首页配置 + 菜单） | 8083 / 9093 |
| admin-web | 企业官网控制台、首页编辑器与预览 | 4173 |
| MySQL | `home_dev` 库与 `t_homepage`、`t_menu` | 3306 |

## 2. 发布前检查

- 工作区改动已 review，`git status` 无意外文件。
- `idl` 已重新生成，`go build ./...` 通过。
- IAM 迁移 000021 待执行（服务启动自动跑）。
- `home_dev` 已创建；若不存在用 `flow-service/tools/dbctl -create home_dev`。
- 4173 端口未被占用；8083/9093 未被占用。
- `IAM_CREDENTIAL_ENCRYPT_KEY` 等 IAM 环境变量已设置（本次不改 IAM 认证逻辑）。

## 3. 发布步骤

1. **生成契约**：`cd D:\fastarch\idl && sh ./proto/protoc.sh`。
2. **编译**：`cd D:\fastarch\home-service && go build ./...`；`cd frontend-service/admin-web && pnpm build`。
3. **启动 IAM**（若未运行）：`cd iam-service && go run . grpc`（执行 000021 迁移），再 `go run . http`。
4. **启动 home**：`cd home-service && go run . grpc`（执行 home 迁移、建表与种子），再 `go run . http`。
5. **启动前端**：`cd frontend-service/admin-web && pnpm dev`。
6. **冒烟**：
   - 登录后控制台选择页出现「企业官网」。
   - 进入 `/home/homepage`，左侧表单、右侧预览正常。
   - 修改文案 → 实时预览；保存 → 刷新保留；发布 → 已发布时间更新。
   - `curl GET /home/homepage`（带 JWT 与 `X-Fa-App-Code: home`）返回 200。
7. **留痕**：记录发布版本、迁移版本、验证截图/命令输出。

## 4. 回滚方案

- 前端：下线 `/home` 菜单种子（`t_menu` status=disabled）或回滚 admin-web 版本；不影响其他控制台。
- 服务：停止 home-service 进程；其他服务不依赖它，停机不影响 IAM/sender/flow。
- 数据库：`t_homepage` 为新增表，保留数据；回滚代码不删表。IAM 000021 可用 down 脚本撤销接入关系/角色/权限/应用（应用被引用时先解绑）。
- 契约：`idl/pb/home` 为新增包，回滚不会破坏既有服务。

## 5. 监控与验证

- 服务日志：`home-service grpc ready`、迁移版本、HTTP 请求错误。
- 关键指标：保存/发布接口 5xx、409 冲突次数、JSON 解析失败次数。
- 数据检查：`SELECT application_id, version, published_at FROM t_homepage`。
- 发布后 24h 内关注 401/403 是否异常升高（权限种子遗漏会 fail-closed）。

## 6. 风险

| 风险 | 缓解 |
| --- | --- |
| IAM 迁移未执行导致 403 | 启动顺序 IAM 先于 home；冒烟检查权限码 |
| 非 admin 未接入 home | 迁移种子绑定当前用户；生产用管理员分配 |
| 前端代理误拦 API | `Accept: text/html` bypass；curl 验证 `/home/homepage` |
| 并发保存覆盖 | 乐观锁 409；前端提示刷新 |
