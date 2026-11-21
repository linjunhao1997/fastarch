# 企业首页展示后台 · 代码变更说明（code_change）

## 1. 交付概述

本次交付在企业现有 FastArch 单体多宿主架构中新增第 4 个宿主 `home-service`，并在 `admin-web` 增加「企业官网」控制台，实现企业官网首页的**可视化配置、实时预览、草稿保存与发布**。

- 运营可在后台动态调整：品牌名称、Logo 文字、导航菜单、首屏文案、主题色、背景风格、整页布局、数据指标、内容区块（标题/正文/布局/显隐/卡片条目/排序）。
- 右侧实时预览随编辑即时更新，支持全屏预览。
- 草稿与已发布内容分离；保存草稿不影响线上，点击发布后 `publishedJson` 更新。
- `version` 乐观锁防止并发覆盖，冲突返回 HTTP 409。

## 2. 代码与配置变更

### 2.1 契约层（idl）

- 新增 `idl/proto/home/homepage.proto`：`home.v1.HomepageService`
  - `GetHomepage`
  - `SaveDraft`
  - `PublishHomepage`
  - `GetPublishedHomepage`
- 新增 `idl/proto/home/menu.proto`：`home.v1.MenuService.ListMyMenuTree`（仅暴露首页配置菜单）。
- 生成 `idl/pb/home/*` 与 `idl/openapiv2/home/*.swagger.json`。

### 2.2 后端（home-service）

- 新增独立 Go 模块 `home-service`：
  - `main.go` / `cmd/root.go`：`grpc` 与 `http` 两个子命令。
  - `conf/config.yaml`：gRPC `127.0.0.1:9093`，HTTP `127.0.0.1:8083`，`application_id: 4`，IAM `127.0.0.1:9090`。
  - `server/grpc.go`：迁移、DB、IAM 鉴权、服务注册。
  - `server/http.go`：HTTP gateway、鉴权中间件、CORS。
- 数据层：
  - `pkg/model/homepage.go`：单行 `t_homepage`，字段 `draft_json`、`published_json`、`version`、`published_at`。
  - `pkg/migration/migrations/000001_init.up.sql`：建 `t_menu`、`t_homepage`、默认首页、菜单种子 `/home/homepage`。
- 业务层：
  - `pkg/service/homepage_service.go`：读取（无数据回落默认）、JSON 合法性校验、保存草稿（乐观锁）、发布、读取已发布。
  - `pkg/service/default_homepage.go`：与前端 `defaultHomepage` 同构的默认完整首页（3 个区块）。
  - `pkg/service/method_permissions.go`：RPC → 权限码映射。
  - `pkg/service/host_hooks.go`：`home` 宿主钩子。
  - `pkg/service/homepage_service_test.go`：4 个测试（默认回落、保存/发布/读取、版本冲突、非法 JSON）。

### 2.3 IAM

- 新增 `iam-service/pkg/migration/migrations/000021_home.up.sql` / `.down.sql`：
  - 注册应用 `home`（id=4，名称「企业官网」）。
  - 权限码：`home.page.get`、`home.page.update`、`home.page.publish`、`home.menu.*`、`home.dict.*`。
  - 角色 `home_admin`。
  - 当前用户 `linjunhao` 接入 home 应用并授予 `home_admin`。
- 已通过重启 IAM gRPC 触发迁移，数据库确认写入成功。

### 2.4 前端（admin-web）

- 新增：
  - `src/home/types/homepage.ts`：`HomepageContent` 类型、默认完整内容、normalize/parse。
  - `src/home/api/home.ts`：`getHomepage` / `saveDraft` / `publishHomepage` / `getPublishedHomepage`，固定 `X-Fa-App-Code: home`。
  - `src/home/pages/HomepagePreview.tsx`：纯展示官网预览，支持主题、背景、整页布局、区块布局与响应式。
  - `src/home/pages/HomepageEditorPage.tsx`：站点设置、首屏、主题与布局、数据指标、区块编辑（增删/上下移/显隐/单区块布局/卡片条目）、实时预览、全屏预览、保存草稿、发布、脏状态、版本冲突提示。
  - `src/home/pages/PublicHomepagePage.tsx`：公开官网首页，读取已发布版本，不经过后台登录与侧栏。
  - `src/home/router.tsx`：`/home/homepage` 路由与 `home.page.get` 权限。
- 修改：
  - `src/shared/api/app-context.ts`：注册 `home` 控制台与菜单。
  - `src/shared/components/AuthWrapper.tsx`：支持 `home` 与 `preferredAppCode`。
  - `src/shared/context/DataScopeContext.tsx`：进入 home 控制台时优先选择 home 数据范围，避免被 flow 污染。
  - `src/shared/types/permissions.ts`：补 `home` 权限码与菜单/字典模块码。
  - `src/app/router.tsx`：挂载 `/home` 子树与 `/site` 公开官网路由。
  - `src/app/ConsoleSelectPage.tsx`：新增「企业官网」控制台卡片。
  - `vite.config.ts`：`/home` 代理到 `8083`，并保留 HTML 请求 bypass 给 SPA。

### 2.5 文档

- `docs/home-service-requirement.md`
- `docs/home-service-feature-design.md`
- `docs/home-service-acceptance.md`
- `docs/home-service-tech-plan.md`
- `docs/home-service-release-plan.md`

## 3. 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/home/homepage` | 读取当前草稿与发布信息 |
| PUT | `/home/homepage/draft` | 保存草稿，body `{contentJson, expectedVersion}` |
| POST | `/home/homepage/publish` | 发布，body `{expectedVersion}` |
| GET | `/home/homepage/published` | 读取已发布内容（公开读，无需登录） |
| GET | `/home/menu/my-menu-tree` | 当前用户首页菜单 |

除登录外均需 `Authorization: Bearer <JWT>` 与 `X-Fa-App-Code: home`。

## 4. 验证记录

- `home-service`: `go build ./...` 通过；`go test ./...` 通过（4/4）。
- `admin-web`: `pnpm build` 通过，产物包含 `HomepageEditorPage-*.js`。
- IAM 迁移：`home` 应用、8 个权限码、`home_admin` 角色、`linjunhao` 接入与角色绑定均已查库确认。
- home DB：`t_homepage`、`t_menu`、`schema_migrations` 存在；默认行 `id=1, application_id=4`。
- HTTP 冒烟（admin JWT + `X-Fa-App-Code: home`）：
  - `GET /home/homepage` → 200
  - `PUT /home/homepage/draft` → 成功，version 递增
  - `POST /home/homepage/publish` → 成功，`publishedAt` 更新
  - 用过期版本再次保存 → 409，版本冲突生效
- 前端联调：Vite `4173`；`GET /home/homepage` HTML 正常；`/home` API 代理 8083 正常。
- 浏览器 UI：
  - admin 登录后控制台选择页出现第 4 张卡「企业官网」。
  - 进入 `/home/homepage` 渲染站点设置、主题与布局、区块编辑、状态条。
  - 修改品牌名称后状态变为「有未保存修改」，全屏预览即时显示新名称；还原后恢复「已保存」。
  - 全屏预览完整展示页头、首屏、3 个内容区块与页脚。
  - 刷新后显示「草稿 v5 / 已发布 2026-11-22 00:33」。
- 当前线上发布内容已恢复为完整默认首页（3 个区块），version=5。

## 5. 运行与访问

- IAM gRPC `9090`、IAM HTTP `8080`。
- home gRPC `9093`、home HTTP `8083`。
- admin-web `http://127.0.0.1:4173/home/homepage`。
- 数据库 `home_dev`。

## 6. 已知边界

- 首页配置为单应用单行模型，适用于单企业官网；多站点需后续扩展 `site_id`。
- 当前未实现图片上传，Logo 使用文字标识；后续可接入文件服务。
- 发布为整页快照，不做区块级灰度。
