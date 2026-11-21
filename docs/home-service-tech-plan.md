# 企业首页展示后台 · 技术方案（tech_plan）

## 1. 总体架构

沿用仓库的 IDL 驱动微服务模板，新增第 4 个宿主 `home-service`：

```
React admin-web（企业官网控制台 /home）
        │  REST /home/...
        ▼
home-service HTTP 网关 :8083  (grpc-gateway)
        │  gRPC
        ▼
home-service gRPC :9093
  ├─ authz 拦截器（透传 JWT 调 IAM CheckPermission）
  ├─ HomepageService（草稿/发布/已发布读取）
  └─ MenuService（仅 ListMyMenuTree，contrib 实现）
        │
        ▼
MySQL home_dev（t_homepage + t_menu + schema_migrations）
```

IAM 侧：应用 `home`（id=4）+ 权限码 `home.page.*` + 角色 `home_admin` + 当前用户接入关系与角色绑定。IAM 仍是唯一授权权威，home-service 不复制判权逻辑。

## 2. 契约（idl/proto/home）

### 2.1 homepage.proto

```proto
service HomepageService {
  rpc GetHomepage(GetHomepageRequest) returns (Homepage) {
    option (google.api.http) = { get: "/home/homepage" };
  }
  rpc SaveDraft(SaveDraftRequest) returns (Homepage) {
    option (google.api.http) = { put: "/home/homepage/draft" body: "*" };
  }
  rpc PublishHomepage(PublishHomepageRequest) returns (Homepage) {
    option (google.api.http) = { post: "/home/homepage/publish" body: "*" };
  }
  rpc GetPublishedHomepage(GetPublishedHomepageRequest) returns (PublishedHomepage) {
    option (google.api.http) = { get: "/home/homepage/published" };
  }
}
```

`Homepage` 返回 `id / draftJson / publishedJson / publishedAt / version / updatedBy / updateTime / application`。`SaveDraftRequest` 带 `contentJson` 与 `expectedVersion`；`PublishHomepageRequest` 可带 `expectedVersion`。

### 2.2 menu.proto

只声明 `ListMyMenuTree`（GET `/home/menu/my-menu-tree`，仅登录态），复用 `common.menu.v1` 消息；contrib `menu.MenuService` 的方法集是其超集，可直接注册。

### 2.3 生成

`cd idl && sh ./proto/protoc.sh` 生成 `idl/pb/home`、`idl/openapiv2/home`。禁止手改生成物。

## 3. 服务实现

### 3.1 模型与迁移

`t_homepage`（单应用单行）：

| 列 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint PK | 雪花/自增 |
| application_id | bigint unique | 固定 4，隔离 |
| draft_json | longtext | 草稿 JSON |
| published_json | longtext | 已发布 JSON |
| published_at | datetime(3) nullable | 发布时间 |
| version | bigint | 乐观锁版本 |
| created_by / updated_by | varchar(64) | 审计 |
| create_time / update_time | datetime(3) | 审计 |

迁移 `000001_init` 同时建 `t_menu`（contrib 菜单表）与 `t_homepage`，并插入默认首页 JSON、菜单种子 `/home/homepage`（icon `BookOutlined`，permission_code `home.page.get`）。`000002` 可留空占位。

### 3.2 业务规则

- **GetHomepage**：按 `application_id` 查单行；不存在返回内置默认 JSON（fail-soft，不 500）。
- **SaveDraft**：校验 `contentJson` 是 JSON 对象且 ≤200KB；`expectedVersion > 0` 时以 `WHERE version=?` 更新，`RowsAffected=0` 返回 `3002004`（Conflict）。更新 `draft_json`、`version+1`、`updated_by`。`published_json` 不动。
- **Publish**：锁行读取草稿 → 复制到 `published_json` → `published_at=now` → `version+1`；可带 `expectedVersion` 做二次校验。
- **GetPublished**：返回 `published_json` + `published_at`；未发布返回空串。权限码 `home.page.get`。

### 3.3 判权

`method_permissions.go`：

```go
"/home.v1.HomepageService/GetHomepage":           {"home.page.get"},
"/home.v1.HomepageService/SaveDraft":             {"home.page.update"},
"/home.v1.HomepageService/PublishHomepage":       {"home.page.publish"},
"/home.v1.HomepageService/GetPublishedHomepage":  {"home.page.get"},
```

`authOnlyMethods`: `/home.v1.MenuService/ListMyMenuTree`。未登记方法 nil → fail-closed。

### 3.4 错误码

`3002001` 配置不存在（通常不触发，回默认）；`3002002` JSON 非法；`3002003` 内容超限；`3002004` 版本冲突；`3002005` 状态不允许发布。

## 4. IAM 迁移（000021_home）

- 注册应用 id=4 `home` / `企业官网`。
- 权限码：`home.page.get`、`home.page.update`、`home.page.publish`、`home.menu.*`（5 个，菜单服务预留）。
- 角色 `home_admin`（application_id=4，绑定全部 home 权限码）+ 将 `linjunhao` 接入 home 应用并授予该角色（dev 种子；生产改由管理员分配）。
- 幂等写法：`INSERT ... SELECT ... WHERE NOT EXISTS`。

## 5. 前端

### 5.1 控制台接入

- `ConsoleKinds` 增加 `home`；`CONSOLE_REGISTRY` 增加「企业官网」。
- `app/router.tsx` 增加 `/home` 子树，`AuthWrapper consoleKind="home"`。
- `AuthWrapper` prop 联合类型扩展；`ConsoleContext` 标签扩展。
- `DataScopeProvider` 增加可选 `preferredAppCode`：若该 code 在接入集合中则优先选中，用于进入 home 控制台时默认数据范围为 home；仍可左上角切换。
- `vite.config.ts` 增加 `/home` → 8083，`Accept: text/html` 的 GET 导航 bypass 给 SPA（与 sender/flow 同款）。

### 5.2 页面

`src/home/pages/HomepageEditorPage.tsx` + `components/HomepagePreview.tsx`：

- `useQuery(['home','homepage'])` 拉取配置。
- 表单状态 `HomepageContent`，JSON 序列化为请求体；`dirty` 通过对比最近一次保存 hash/序列化判断。
- `homeApi` 对 `/home/...` 请求显式带 `X-Fa-App-Code: home` 并 `skipDataScope`，避免左上角数据范围影响本控制台配置读写。
- 保存/发布 mutation + `useSubmitLock`；409 冲突单独提示。
- 预览组件为纯展示，接受 `content` prop，编辑器与全屏 Modal 复用。

### 5.3 默认内容

前后端各有一份同构默认 JSON（后端种子 + 前端 fallback）。字段以 TS 类型 `HomepageContent` 为准，解析时做最小校验（对象、theme、sections 数组），失败回落。

## 6. 风险与对策

| 风险 | 对策 |
| --- | --- |
| `/home` 路由与 API 前缀冲突 | Vite 代理按 `Accept: text/html` bypass 到 SPA |
| 非 admin 进入 home 控制台但数据范围是 flow | homeApi 显式 `X-Fa-App-Code: home`；DataScope 优先 home |
| 菜单接口缺实现导致侧栏空 | home-service 注册 contrib MenuService，仅暴露 ListMyMenuTree |
| 两人同时编辑覆盖 | `expectedVersion` 乐观锁 + 409 提示 |
| JSON 契约漂移 | 前后端共享字段约定 + 最小校验 + 默认回落 |
| 预览与线上不一致 | 预览基于同一份 draft JSON；发布复制 draft，不做二次转换 |

## 7. 验证

- `go build ./...`、`go vet ./...`、`go test ./...`（home-service）。
- `pnpm build`（admin-web）。
- 迁移在 `home_dev` 成功；默认行与菜单种子存在。
- 启动 gRPC 9093 + 网关 8083，curl 验证 GET/PUT/POST 与权限（401/403/200）。
- 前端开发服务器直达 `/home/homepage`，编辑器实时预览、保存、发布、冲突场景。
