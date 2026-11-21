# 企业首页展示后台 · 部署记录（deployment_record）

## 1. 发布信息

| 项 | 值 |
| --- | --- |
| 发布单号 | `home-20261122-001` |
| 环境 | 本地开发/联调环境（Windows） |
| 发布时间 | 2026-11-22 00:41–00:46（UTC+8） |
| 发布对象 | IAM 000021 迁移、home-service、admin-web 企业官网控制台 |
| 发布结果 | **成功**，健康检查全部通过 |
| 最终版本 | 首页草稿/已发布 `version=11`，已发布时间 2026-11-22 00:42:55 |
| 发布内容 | 企业官网首页可视化配置、实时预览、草稿/发布、权限接入 |

## 2. 发布物与运行进程

| 组件 | 产物/地址 | 端口 | 进程 |
| --- | --- | --- | --- |
| IAM HTTP | 既有 IAM HTTP | 8080 | PID 6160（`iam-grpc.exe`，沿用） |
| IAM gRPC | `D:\fastarch\.tmp\iam-grpc-home.exe` | 9090 | PID 36216 |
| home gRPC | `D:\fastarch\.tmp\home-service.exe grpc` | 9093 | PID 4336 |
| home HTTP | `D:\fastarch\.tmp\home-service.exe http` | 8083 | PID 31932 |
| admin-web | Vite dev server，`D:\runtime\nodejs\node.exe` | 4173 | PID 10420 |
| MySQL | `fastarch_dev` / `home_dev` | 3306 | 既有实例 |

前端生产构建产物：`D:\fastarch\frontend-service\admin-web\dist\assets\HomepageEditorPage-DM0OxJer.js`。

## 3. 迁移执行

| 库 | 迁移版本 | dirty | 说明 |
| --- | --- | --- | --- |
| `fastarch_dev`（IAM） | **21** | 0 | 000021_home：应用/权限/角色/接入 |
| `home_dev`（home） | **1** | 0 | 000001_init：`t_homepage` / `t_menu` / 种子 |

## 4. 发布步骤执行记录

1. 生成 home IDL 产物（`idl/pb/home`、`idl/openapiv2/home`）。
2. `home-service`：`go build ./...`、`go vet ./...`、`go test ./...` 通过；构建 `home-service.exe`。
3. `admin-web`：`pnpm build`（`tsc -b && vite build`）通过，产出 `HomepageEditorPage-*.js`。
4. IAM gRPC 重启，000021 迁移自动执行；`home` 应用、8 个权限码、`home_admin` 角色、`linjunhao` 接入关系落库。
5. home gRPC 启动（执行 000001 迁移、建表与种子）；home HTTP 网关启动。
6. Vite 4173 运行，`/home` 代理到 8083；`/home/homepage` SPA 路由可直达。
7. 发布后健康检查与浏览器验收。

## 5. 健康检查结果

| 检查 | 结果 |
| --- | --- |
| 端口监听 8080 / 9090 / 8083 / 9093 / 4173 | 全部 `LISTEN`，无重复监听 |
| IAM 密码登录 `POST /authentication/password-auth` | 200，返回 JWT |
| `GET /home/homepage`（admin + `X-Fa-App-Code: home`） | 200，`version=11`，`publishedJson` 含 3 个区块 |
| `GET /home/homepage/published` | 200，`publishedAt=2026-11-22 00:42:55` |
| `GET /home/menu/my-menu-tree` | 200，`roots[0].menu.path=/home/homepage` |
| 前端 `GET http://127.0.0.1:4173/home/homepage`（HTML） | 200，返回 SPA `index.html` |
| 浏览器 UI | 「企业官网」控制台、编辑表单、全屏预览正常；状态「已保存 / 草稿 v11 / 已发布 2026-11-22 00:42」 |
| 服务日志 | `home grpc ready`、`http server listening`；无 5xx/异常堆栈 |

## 6. 数据状态

```sql
-- home_dev.t_homepage
id=1, application_id=4, version=11,
published_at=2026-11-22 00:42:55,
draft_len=1770, published_len=1770
```

- 草稿与已发布内容一致，均为完整默认首页（3 个内容区块）。
- IAM：`home` 应用 id=4、`home_admin` 角色 application_id=4、home 权限码 8 个、`linjunhao` 已接入 home 应用。
- 临时 QA 探针账号已删除，未残留测试数据。

## 7. 访问入口

- 公开官网首页：`http://127.0.0.1:4173/site`（无需登录，读取已发布版本）
- 控制台：`http://127.0.0.1:4173/console-select` → 选择「企业官网」
- 首页配置：`http://127.0.0.1:4173/home/homepage`
- 账号：`admin / admin123`（本地联调）

## 8. 监控与日志

- home gRPC：`D:\fastarch\.tmp\home-grpc.out.log` / `home-grpc.err.log`
- home HTTP：`D:\fastarch\.tmp\home-http.out.log` / `home-http.err.log`
- IAM：`D:\fastarch\.tmp\iam-grpc-home.out.log` / `.err.log`
- 关注指标：保存/发布 5xx、409 冲突、400 JSON 校验失败、401/403 异常升高。
- 数据巡检：`SELECT application_id, version, published_at FROM home_dev.t_homepage`。

## 9. 回滚准备

| 层级 | 回滚动作 | 已确认 |
| --- | --- | --- |
| 前端 | 回滚 `dist` 或下线 `t_menu` 中 `/home/homepage` 菜单（status=disabled） | 是 |
| home 服务 | 停止 PID 28320 / 17696，释放 9093/8083 | 是 |
| home 数据库 | 保留 `t_homepage` 数据；必要时执行 `000001_init.down.sql` | 是（down 脚本存在） |
| IAM | 执行 `000021_home.down.sql` 撤销接入/角色/权限/应用 | 是（down 脚本存在） |
| 契约 | `idl/pb/home` 为新增包，回滚不影响既有服务 | 是 |

## 10. 备注

- 发布方案文档中前端端口原写 5173，实际 Vite 配置与运行端口为 **4173**；本地 `docs/home-service-release-plan.md` 已修正，Flow 中已归档的 release_plan 版本仍为 5173，以本部署记录为准。
- 本次为本地联调环境发布，未配置生产反向代理/HTTPS/正式域名；生产发布需另行走审批与变更窗口。
- 验收截图：`docs/assets/homepage-editor.png`、`docs/assets/homepage-preview.png`。
