# 企业首页展示后台 · 测试报告（test_report）

## 1. 测试结论

**通过。** 企业官网首页配置后台的核心链路已可运行：登录 → 进入「企业官网」控制台 → 编辑文案/主题/布局/区块 → 实时预览 → 保存草稿 → 发布 → 刷新读取已发布版本。接口鉴权、参数校验、版本冲突、草稿与发布隔离均已验证。

测试过程中发现并修复 3 个缺陷（详见第 5 节），修复后回归通过。

## 2. 测试环境

| 项 | 值 |
| --- | --- |
| 日期 | 2026-11-22 |
| 系统 | Windows / PowerShell 7 |
| IAM | HTTP 8080，gRPC 9090 |
| home-service | HTTP 8083，gRPC 9093 |
| admin-web | Vite 4173 |
| 数据库 | MySQL `fastarch_dev` / `home_dev` |
| 浏览器 | Chrome，视口 655×703（窄屏响应式） |
| 账号 | admin（超管）；临时探针账号 qa_probe（仅 flow 应用接入，测试后已删除） |

## 3. 验收用例执行结果

| 用例 | 结果 | 证据 |
| --- | --- | --- |
| AC-1 控制台可见「企业官网」并进入首页配置 | 通过 | 登录后 `/console-select` 出现第 4 张卡；点击进入 `/home/homepage` |
| AC-2 页面加载草稿并渲染预览 | 通过 | 页面显示「已保存 / 草稿 v11 / 已发布 2026-11-22 00:42」；全屏预览渲染完整首页 |
| AC-3 修改文案预览即时变化 | 通过 | 品牌名改为「星河智造」→ 状态变「有未保存修改」→ 全屏预览立即显示「星河智造」；还原后状态回「已保存」 |
| AC-4 主题色/布局/区块顺序/显隐同步 | 通过（代码+UI） | 编辑区含主题色、背景、整页布局、区块上移/下移/显隐/单区块布局；预览组件按这些字段渲染 |
| AC-5 保存草稿后刷新保留 | 通过 | 保存返回新 version；UI 刷新后显示 v11；草稿内容保留 |
| AC-6 发布后已发布时间与内容更新 | 通过 | `POST /home/homepage/publish` 返回 `publishedAt` 更新；`publishedJson` 等于发布时草稿 |
| AC-7 草稿与发布隔离 | 通过 | 草稿保存为「QA 隔离验证标题」后，`GET /home/homepage/published` 仍为「让复杂的业务，变得简单可增长」 |
| AC-8 旧版本保存冲突 | 通过 | 当前 version=8 时用 expectedVersion=5 保存 → HTTP **409** |
| AC-9 无权限账号保存/读取 | 通过 | 临时账号仅接入 flow、未接入 home，带 `X-Fa-App-Code: home` 调 `GET /home/homepage` → HTTP **403** `permission denied` |
| AC-10 未登录访问 | 通过 | 无 Authorization → HTTP **401**；非法 token → HTTP **401** `未登录或登录已过期` |

## 4. 接口与数据验证

| 场景 | 结果 |
| --- | --- |
| `GET /home/homepage`（admin + `X-Fa-App-Code: home`） | 200，返回 `draftJson` / `publishedJson` / `version=11` |
| `PUT /home/homepage/draft` 非法 JSON | 400，`2002002 首页内容不是合法的 JSON 对象` |
| `PUT /home/homepage/draft` 210KB 内容 | 400，`2002003 首页内容超过 200KB 上限` |
| `PUT /home/homepage/draft` 过期版本 | 409，`2002004 首页配置已被他人更新，请刷新后重试` |
| `POST /home/homepage/publish` | 成功，version 递增，`publishedAt` 更新 |
| `GET /home/homepage/published` | 返回 `contentJson`、`publishedAt`、`version` |
| 菜单 `GET /home/menu/my-menu-tree` | 200，返回「首页配置」菜单 |
| 最终数据 | `home_dev.t_homepage` id=1, application_id=4, version=11，publishedJson 含 3 个区块 |

## 5. 发现并修复的缺陷

### DEF-1 未登录返回 400 而非 401

- 现象：不带 Authorization 调 `GET /home/homepage`，网关透传给 IAM 后得到 `400`。
- 修复：`home-service/server/http.go` 增加 `RequireLogin` 网关中间件；缺少 Bearer 且无 `authorization` Cookie 时直接返回 401 JSON。
- 回归：无 token → 401。

### DEF-2 IAM 拒绝被包装成 400「服务不可用」

- 现象：无 home 应用接入的用户调用 → IAM 返回 `PermissionDenied: application not admitted`，`contrib/pkg/authz` 将所有 IAM 错误统一映射为 `FailedPrecondition`（HTTP 400），语义错误。
- 修复：`contrib/pkg/authz/interceptor.go` 按 IAM 状态码分流：`Unauthenticated` → 401，`PermissionDenied` → 403，其余才返回 400 服务不可用。
- 回归：非法 token → 401；无 home 应用 → 403。

### DEF-3 home_admin 角色 application_id=0

- 现象：迁移 000021 插入角色时漏了 `application_id`，落库为 0，角色归属不清晰。
- 修复：迁移补 `application_id=4`，并将现有角色行更新为 4。
- 回归：角色查询 `application_id=4`，admin/linjunhao 访问正常。

## 6. 工程验证

| 命令 | 结果 |
| --- | --- |
| `cd home-service && go build ./...` | 通过 |
| `cd home-service && go vet ./...` | 通过 |
| `cd home-service && go test ./...` | 通过（`pkg/service` 4 个测试） |
| `cd frontend-service/admin-web && pnpm build`（`tsc -b && vite build`） | 通过；产物含 `HomepageEditorPage-*.js` |
| `GET http://127.0.0.1:4173/home/homepage`（HTML） | 200，返回 SPA index |
| `/home` API 代理到 8083 | 通过（同路径带 JSON + JWT 返回 200） |

## 7. 体验与兼容性

- 窄屏（655px）下编辑区与预览上下堆叠，表单可滚动，预览可打开全屏 Modal；无横向溢出、无空白。
- 宽屏布局使用 AntD `xl`（≥1200px）断点：编辑区 `xl=11`、预览 `xl=13` 且 `position: sticky`，桌面端为左右分栏（代码审查确认）。
- 保存/发布按钮有 loading 防重复；版本冲突有明确提示；脏状态有「有未保存修改」标识。
- 全屏预览完整渲染页头、首屏、3 个内容区块与页脚；关闭按钮正常。

## 8. 遗留与建议

- 当前为单站点单首页模型；多站点需增加 `site_id`。
- Logo 目前为文字标识，图片上传需后续接入文件服务。
- 发布为整页快照，暂无区块级灰度/定时发布。
- 建议生产环境为 `home_admin` 配置明确的运营账号，不使用 dev 种子用户。

## 9. 补充验证：公开官网首页

- `GET /home/homepage/published`（不带 Authorization）→ 200，返回已发布 `v11` 内容。
- `GET http://127.0.0.1:4173/site`（HTML）→ 200，返回 SPA 页面。
- 浏览器打开 `/site`：标题「星河科技 · 企业官网」，无后台侧栏/顶栏，完整渲染首屏、区块与页脚，不需要登录。
