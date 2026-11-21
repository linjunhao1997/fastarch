# 企业首页展示后台 · 验收标准（acceptance_criteria）

## 1. 功能验收

| ID | 前置 | 步骤 | 预期 |
| --- | --- | --- | --- |
| AC-1 | 账号已接入 home 应用并持有 home_admin | 登录 → 控制台选择页 | 可见「企业官网」卡片；进入后侧栏出现「首页配置」 |
| AC-2 | 已进入首页配置页 | 打开页面 | 左侧表单加载默认/已保存草稿，右侧预览渲染完整首页，无空白/报错 |
| AC-3 | 同上 | 修改主标题，观察右侧预览 | 预览标题即时变化，无需保存 |
| AC-4 | 同上 | 切换主题色、整页布局、区块顺序与显隐 | 预览颜色、排版、区块顺序同步变化；隐藏区块不渲染 |
| AC-5 | 已有默认内容 | 点击保存草稿 → 刷新页面 | 草稿修改保留；已发布时间/已发布内容不变 |
| AC-6 | 草稿已保存 | 点击发布 | 发布成功提示；已发布时间更新；已发布内容等于发布时草稿 |
| AC-7 | 发布后 | 再修改文案但只保存草稿 | 读取已发布版本仍是旧内容（草稿与发布隔离） |
| AC-8 | 两个标签页同时打开 | A 保存后，B 用旧版本保存 | B 收到版本冲突提示，不覆盖 A 的内容 |
| AC-9 | 无 home.page.update 的账号 | 直接调用保存接口 | 服务端返回 403（PermissionDenied） |
| AC-10 | 未登录 | 调用任意首页接口 | 返回 401 |

## 2. 数据与接口验收

- `GET /home/homepage` 返回 `draftJson`、`publishedJson`、`publishedAt`、`version`、审计字段。
- `PUT /home/homepage/draft` 仅更新草稿与版本；`publishedJson` 不变。
- `POST /home/homepage/publish` 将当前草稿复制到 `publishedJson`，写 `publishedAt` 并递增版本。
- `GET /home/homepage/published` 只返回已发布 JSON 与发布时间；未发布时返回空内容。
- 传入非法 JSON 或超过 200KB 返回 400；版本不匹配返回 409 语义的业务码。

## 3. 工程验收

- `go build ./...`、`go vet ./...`、`go test ./...` 在 `home-service` 通过。
- `pnpm build`（tsc + vite）在 `admin-web` 通过。
- `home_dev` 迁移成功，`t_homepage` 有默认种子，`t_menu` 有 `/home/homepage` 种子。
- IAM 迁移成功：`home` 应用、权限码、`home_admin` 角色、当前用户接入关系均落库。
- 服务启动：home gRPC 9093、网关 8083；`/home/homepage` 可经 8083 访问。
- 前端 Vite 代理 `/home` → 8083，路由 `/home/homepage` 可直达。

## 4. 体验验收

- 编辑器在 1440px 屏幕下左栏表单可滚动、右栏预览 sticky；全屏预览可关闭。
- 保存/发布按钮有 loading 防重复提交；错误使用统一通知，不出现裸 alert。
- 修改后未保存有明确标识；保存成功提示带版本号。
