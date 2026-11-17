# FastArch 微服务脚手架

基于 Go + React 的微服务工程模板，IDL 驱动（Protobuf → gRPC + grpc-gateway + OpenAPI），业务按 DDD 分层组织。内置首个业务服务 **iam-service**（身份与权限管理），新服务（如 sender-service）按照统一模板接入即可。

> 各模块更细的说明见：[iam-service/readme.md](iam-service/readme.md) · [idl/readme.md](idl/readme.md) · [contrib/readme.md](contrib/readme.md)

## 一、总体架构

```
┌───────────────────────────────────────────────────────────────┐
│  前端 frontend-service/admin-web :5173                          │
│  React 19 + Ant Design + Vite + TS                            │
│  按 <服务前缀> 代理 → /account /authorization /sender ...      │
└──────────────────────────┬────────────────────────────────────┘
                           │ HTTP (REST)
┌──────────────────────────▼────────────────────────────────────┐
│  各服务的 HTTP 网关进程  (go run . http, :8080/:8081/...)       │
│  grpc-gateway: 反向代理，REST → gRPC，透传 Authorization       │
└──────────────────────────┬────────────────────────────────────┘
                           │ gRPC
┌──────────────────────────▼────────────────────────────────────┐
│  各服务的 gRPC 进程  (go run . grpc, :9090/:9091/...)          │
│  拦截器链: Recovery → Validation → Error → GormErr → Auth     │
│  service 层(业务) ── model 层(GORM) ── contrib(基础设施)       │
└──────────────────────────┬────────────────────────────────────┘
                           │
              ┌────────────┴────────────┐
              │  MySQL (migration 迁移)  │
              │  Redis (token/权限缓存)  │
              └─────────────────────────┘
```

**仓库 = 3 个 Go module + 1 个前端**

| 目录 | module | 职责 |
|---|---|---|
| `idl/` | `idl`（对外名 `github.com/linjunhao1997/idl`） | **接口层**：proto 定义 + 生成的 pb/gateway/swagger 代码 + 公共拦截器库（`lib/grpclib`、`lib/gwlib`） |
| `contrib/` | `github.com/linjunhao1997/contrib` | **共享基础设施**：配置加载、cobra 应用引导、grpcserver/httpserver 组装、MySQL/Redis 助手、代码生成器 |
| `iam-service/` | `iam-service` | 业务服务（每个业务服务一个这样的目录） |
| `frontend-service/admin-web/` | — | React 管理前端（IAM + sender 等宿主） |

依赖方向：`iam-service → idl + contrib`，`contrib → idl`，均通过 `replace ../` 指向本地。**idl 被所有服务依赖，contrib 被所有服务复用，业务服务之间不互相依赖**（需要跨服务调用走 gRPC）。

## 二、DDD 分层映射

项目采用"轻量 DDD"：接口契约由 proto 承担，领域逻辑收敛在 service 层，持久化收敛在 model 层。

| DDD 概念 | 本项目落点 | 规则 |
|---|---|---|
| 接口层（Interface） | `idl/proto/<domain>/*.proto`（HTTP annotation）+ 生成的 `*_pb.gw.go` | 只定义契约，不含逻辑；校验写在 proto（`buf.validate`） |
| 应用层（Application） | `iam-service/pkg/service/*_service.go` | 实现 pb 生成的 `XxxServiceServer` 接口；做 DTO(model↔pb) 转换、编排、事务边界 |
| 领域/持久层（Domain/Infra） | `iam-service/pkg/model/` + `pkg/helper/` | GORM 模型 + 生成的列名常量；helper 提供分页等横切工具 |
| 共享内核（Shared Kernel） | `idl/lib/*`、`contrib/pkg/*` | 拦截器、错误转换、配置、服务组装——业务服务只"用"不"改" |
| 前端（BFF 视角） | `frontend-service/admin-web/src/{app,shared,iam,sender}` | 三层垂直切片：app 壳 / shared 公用 / 各服务专属域；路由即菜单 |

## 三、目录结构

```
fastarch/
├── idl/                          # 接口定义与生成代码（独立 module）
│   ├── proto/
│   │   ├── buf.yaml / buf.gen.yaml / protoc.sh   # buf 代码生成
│   │   ├── common/common.proto   # 公共消息: Page、GetRequest
│   │   ├── iam/                  # iam 领域: account / application / authentication / authorization
│   │   └── sender/               # sender 领域: message / template / menu / dict
│   ├── pb/                       # 生成: *.pb.go / *_grpc.pb.go / *_pb.gw.go
│   ├── openapiv2/                # 生成: *.swagger.json
│   └── lib/
│       ├── grpclib/              # 拦截器: 错误→ErrorInfo、recovery、protovalidate 校验、traceID
│       └── gwlib/                # gateway 工具: cookie 取 Authorization → metadata
│
├── contrib/                      # 工具集与服务启动基础设施（独立 module）
│   ├── pkg/
│   │   ├── app/                  # cobra 引导: New[T], AddCommand, 统一 -c flag, 信号 context
│   │   ├── config/               # 通用配置段(App/Database/Redis/GRpc/Http/Jwt) + viper Load[T]
│   │   ├── database/             # NewMySQL(dsn)
│   │   ├── cache/                # NewRedis(addr)
│   │   ├── server/grpcserver/    # gRPC 组装: 默认拦截器链 + 优雅启停
│   │   ├── server/httpserver/    # gateway mux 组装 + 优雅启停
│   │   ├── authz/                # 入站判权薄客户端：透传 JWT 调 IAM CheckPermission（ADR 0005）
│   │   ├── menu/ dict/           # 每服务自持的菜单/字典通用能力（ADR 0001）
│   │   └── gen/                  # 历史列名常量生成器（已弃用，勿再用）
│   └── main.go
│
├── iam-service/                  # 业务服务模板（新服务照此复制）
│   ├── main.go                   # → cmd.Execute()
│   ├── cmd/root.go               # app.New[Config] + AddCommand("grpc"/"http")
│   ├── conf/                     # config.go + config.yaml
│   ├── server/
│   │   ├── grpc.go               # 迁移 fail-fast + RegisterXxxServiceServer + 拦截器链
│   │   └── http.go               # RegisterXxxServiceHandlerFromEndpoint
│   └── pkg/
│       ├── auth/                 # JWT + 鉴权拦截器（IAM 自有；新服务改用 contrib/authz 薄客户端）
│       ├── constant/             # 错误码前缀、cache key
│       ├── helper/               # page.go 分页封装
│       ├── migration/            # golang-migrate 迁移 SQL（内嵌，建表/变更/种子）
│       ├── model/                # 手写模型 + *_generated.go(列名常量) + ddl.sql + migration.go(模型注册)
│       └── service/              # 业务实现 + service_error.go 业务错误
│
├── sender-service/               # 第二宿主（新服务照此复制）：库/契约/钩子/authz 见第七节
│
└── frontend-service/admin-web/   # 唯一 React 管理前端（IAM + sender 等宿主）
    └── src/
        ├── app/                  # 应用壳：入口/路由聚合/布局页
        ├── shared/               # 公用：api(含动态前缀)、组件、hooks、context、types、菜单/字典配置页
        ├── iam/                  # IAM 专属：api/components/pages/types
        └── sender/               # sender 专属：api/pages/router（路径与菜单种子对齐）
```

## 四、技术栈

| 层级 | 技术 |
|---|---|
| 前端 | React 19 + Ant Design + Vite + TypeScript + React Router v7（createBrowserRouter） |
| 接口 | Protobuf + buf，`google.api.http` annotation，`buf.validate` 参数校验 |
| 通信 | gRPC + grpc-gateway（REST 双协议） |
| 后端 | Go 1.21+，cobra CLI，viper 配置，GORM，JWT，Redis |
| 存储 | MySQL（golang-migrate 迁移）+ Redis |

## 五、快速开始（从零部署）

### 5.0 环境要求

| 依赖 | 版本 | 用途 |
|---|---|---|
| Go | 1.21+ | 后端编译运行 |
| Node | 20+ | 前端构建 |
| pnpm | 8+ | 前端包管理 |
| MySQL | 8 | 业务数据（启动时跑 golang-migrate 迁移自动建表） |
| Redis | 7 | token/权限快照缓存 |

> buf 工具链（`buf`、`protoc-gen-*`）只在**修改 proto 重新生成代码**时才需要，纯部署可跳过；生成产物（`idl/pb/`、`idl/openapiv2/`）已随仓库提交。

### 5.1 数据库准备

每个服务一个独立库。需建 `fastarch_dev`（IAM）与 `sender_dev`（sender），**建表/变更/种子由服务启动时跑内嵌迁移自动完成**（golang-migrate，失败 fail-fast），无需手工执行 DDL：

```sql
CREATE DATABASE IF NOT EXISTS fastarch_dev DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE DATABASE IF NOT EXISTS sender_dev   DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
```

然后确认 `iam-service/conf/config.yaml` 与 `sender-service/conf/config.yaml` 的 `database.dsn`、`redis.addr`、`sender.iam_addr` 与本机环境一致。

### 5.2 注入凭证加密密钥（必须）

AK/SK 访问凭证功能要求注入 32 字节（hex/base64）加密密钥，**缺失时服务启动直接报错**（fail-fast）。密钥只走环境变量，**禁止写入 config.yaml 入仓**。

- Windows：用系统设置或 PowerShell 的用户级环境变量 `IAM_CREDENTIAL_ENCRYPT_KEY`（重开终端生效）
- Linux/macOS：在 `~/.bashrc` 或 `~/.zshrc` 中 `export IAM_CREDENTIAL_ENCRYPT_KEY='<32字节hex密钥>'`

> 密钥可自行生成（如 `openssl rand -hex 32`）。注意：同一套库换密钥后，已落库的旧凭证密文将无法解密，需重新生成凭证。

### 5.3 启动后端（四个终端）

```bash
cd iam-service && go run . grpc      # gRPC  :9090（启动时跑迁移 + 种子）
cd iam-service && go run . http      # 网关  :8080 → 9090
cd sender-service && go run . grpc   # gRPC  :9091（启动时跑迁移 + 种子；入站判权需 IAM 已启动）
cd sender-service && go run . http   # 网关  :8081 → 9091
```

### 5.4 管理员账号（种子自动创建，无需手工 SQL）

迁移 000003 已创建内置管理员 `admin` / 初始密码 `admin123`（bcrypt 摘要），并绑定 `admin` 角色与其权限码；admin 角色在应用准入上旁路，可直接进入所有启用应用（iam / sender），**无需手工引导 SQL**。

**登录后请立即修改 `admin` 密码，并新建日常使用的管理员账号、禁用/删除内置账号。**

### 5.5 启动前端

```bash
cd frontend-service/admin-web
pnpm install
pnpm dev           # :5173，代理 /account /application /authorization /authentication /iam → :8080，/sender → :8081
```

浏览器打开 http://localhost:5173 ，用 `admin` / `admin123` 登录即可。

配置文件示例（`iam-service/conf/config.yaml`，sender 见 §7.3）：

```yaml
app:
  name: iam-service
  mode: debug
database:
  dsn: "root:password@tcp(127.0.0.1:3306)/fastarch_dev?charset=utf8mb4&parseTime=True&loc=Local"
redis:
  addr: "localhost:6379"
grpc:  { addr: "127.0.0.1", port: 9090 }
http:  { addr: "127.0.0.1", port: 8080, endpoint: "127.0.0.1:9090" }
jwt:   { secret: "your-secret-key", expiration_hours: 24 }
```

## 六、核心机制

### 6.1 代码生成（IDL 是唯一事实来源）

`idl/proto/protoc.sh` = `buf dep update && buf generate`，按 `buf.gen.yaml` 一次生成四类产物：

| 产物 | 位置 | 用途 |
|---|---|---|
| `*.pb.go` | `idl/pb/<domain>/` | 消息结构 |
| `*_grpc.pb.go` | `idl/pb/<domain>/` | `XxxServiceServer` 接口 + `RegisterXxxServiceServer` |
| `*_pb.gw.go` | `idl/pb/<domain>/` | `RegisterXxxServiceHandlerFromEndpoint`（gateway） |
| `*.swagger.json` | `idl/openapiv2/<domain>/` | OpenAPI 文档 |

关键插件选项（`buf.gen.yaml`）：`require_unimplemented_servers=false`（业务实现不强制嵌 `UnimplementedXxxServer`，用编译断言代替）、`generate_unbound_methods=true`。

模型列名常量写在 `*_generated.go`（如 `MessageRecordColumnChannel`），由服务自行维护；历史生成器 `gen_gorm_mysql_repository` 已弃用，不要再调用。运行时的建表/变更由迁移 SQL 承担，不是 AutoMigrate。

### 6.2 gRPC 拦截器链

`grpcserver.WithDefaultInterceptors()`（contrib）依序挂载：

1. **Recovery**（`idl/lib/grpclib`）— panic → Internal 错误
2. **Validation** — `protovalidate` 自动校验 proto 里的 `buf.validate` 规则
3. **ErrorDetails** — 业务错误（实现 `BusinessError` 接口）→ gRPC status，`ErrorInfo{reason=业务码, domain=服务名}`
4. **GormErrorMapping** — gorm `ErrRecordNotFound` → NotFound
5. **业务鉴权** — 因服务而异：IAM 自有 `pkg/auth`（JWT + Redis 权限快照）；新宿主服务用 contrib `pkg/authz` 薄客户端透传 JWT 调 IAM `CheckPermission`（ADR-0005），见 §7.6

业务错误统一定义在 `pkg/service/service_error.go`：`Error{Code 业务码, grpcCode HTTP语义码, Err}`，业务码按服务分段（iam 用 1001xxx），前端从 HTTP 错误响应的 `message` / `details` 中取文案。

### 6.3 分页

proto 列表请求统一带 `current_page` / `page_size`，响应统一 `common.v1.Page`；service 层用 `helper.GetLimitAndOffset` + `helper.GetPage`（默认 page=1/size=10/max=100）。

### 6.4 HTTP 网关

每个服务一个独立网关进程（`go run . http`），`RegisterXxxServiceHandlerFromEndpoint` 反向代理到自己的 gRPC。`gwlib` 负责 cookie 中的 `Authorization` → gRPC metadata 透传。前端通过 Vite 代理按 **服务前缀** 转发到对应网关。

## 七、新增一个微服务（以 sender-service 为例）

> 以 sender-service 为实战蓝本，完整对照其落地结构。sender 是平台第二个宿主（code `sender`、服务段 `sender`、应用 id 固定 `2`），验证 ADR-0001~0005 的「每应用自持、多应用隔离」。gRPC `:9091`、网关 `:8081`、独立库 `sender_dev`、HTTP 前缀 `/sender/`。

### 7.1 全局视角：接入一个宿主要做的四件事

1. **注册应用与权限码（IAM 侧）**：应用是权限码服务段命名空间。在 IAM 迁移里登记应用与 `<service>.*` 权限码，并绑定管理员角色（见 `iam-service/pkg/migration/migrations/000010_sender.up.sql`）。**授权词汇归 IAM，业务数据归宿主自己的库**。
2. **契约（idl 侧）**：新增 `<domain>.v1` proto，业务服务 + 菜单/字典服务（后者引用共享消息 `common.menu.v1`/`common.dict.v1`，service 自持），HTTP 路径统一带 `/<service>/` 前缀（ADR-0003）；buf 生成 pb/gateway/openapi，**禁止手改生成物**。
3. **宿主服务（本服务）**：迁移建自己的库表与种子（golang-migrate）；模型 + 业务服务；由 contrib 嵌入菜单/字典能力（注入宿主钩子）；入站判权经 contrib 薄客户端透传 JWT 调 IAM `CheckPermission`（ADR-0005）；服务装配迁移 fail-fast。
4. **前端（admin-web）**：在对应域目录加 api 与页面、挂路由子树；菜单/字典配置页走共享层，按当前 app code 动态拼前缀，切应用即读写对应服务的库。

### 7.2 第 1 步：契约 — `idl/proto/sender/`

四份 proto：

| 文件 | 服务 | 路径前缀 | 说明 |
|---|---|---|---|
| `message.proto` | `MessageService` | `/sender/messages`、`/sender/records` | 发送（模拟投递 pending→sent）、记录 Get/List/Delete |
| `template.proto` | `TemplateService` | `/sender/templates` | 消息模板 CRUD |
| `menu.proto` | `MenuService` | `/sender/menu/...` | 引用 `common.menu.v1` 消息，宿主自持实现 |
| `dict.proto` | `DictService` | `/sender/dict/...` | 引用 `common.dict.v1` 消息，宿主自持实现 |

```protobuf
service MessageService {
  rpc SendMessage(SendMessageRequest) returns (MessageRecord) {
    option (google.api.http) = { post: "/sender/messages" body: "*" };
  }
  rpc GetMessage(common.v1.GetRequest) returns (MessageRecord) {
    option (google.api.http) = { get: "/sender/records/{id}" };
  }
  rpc ListMessages(ListMessagesRequest) returns (ListMessagesResponse) {
    option (google.api.http) = { get: "/sender/records" };
  }
  rpc DeleteMessage(common.v1.GetRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = { delete: "/sender/records/{id}" };
  }
}
```

契约要点：包名 `<domain>.v1`；HTTP 路径 `/<服务前缀>/<资源复数>`；Get 复用 `common.v1.GetRequest`；列表响应必带 `common.v1.Page`；id 一律 string（雪花 ID）；校验用 `buf.validate` 写在 proto；审计字段只读回传、请求消息不含（宪法 XII）。

```bash
cd idl && sh ./proto/protoc.sh   # 产出 idl/pb/sender/ 与 idl/openapiv2/sender/
```

### 7.3 第 2 步：服务骨架与配置

```
sender-service/
├── main.go                       # package main → cmd.Execute()
├── cmd/root.go                   # app.New[Config] + AddCommand("grpc"/"http") + 端口默认值
├── conf/{config.go, config.yaml}
├── server/{grpc.go, http.go}
└── pkg/
    ├── migration/                # golang-migrate 迁移（migrations/*.sql 内嵌）
    ├── model/                    # 手写模型 + *_generated.go 列名常量 + ddl.sql + migration.go(模型注册)
    └── service/                  # 业务实现 + 宿主钩子 + 方法权限映射 + service_error.go
```

`go.mod` 经本地 replace 依赖 idl / contrib（不复制代码到服务私有目录）：

```bash
cd sender-service
go mod edit -replace github.com/linjunhao1997/idl=../idl
go mod edit -replace github.com/linjunhao1997/contrib=../contrib
```

`conf/config.yaml` 关键段——sender 有自己的库，`sender.iam_addr` 指向 IAM gRPC 供入站判权：

```yaml
database:
  dsn: "root:...@tcp(127.0.0.1:3306)/sender_dev?charset=utf8mb4&parseTime=True&loc=Local"
grpc: { addr: "127.0.0.1", port: 9091 }
http: { addr: "127.0.0.1", port: 8081, endpoint: "127.0.0.1:9091" }
sender:
  application_id: 2                 # IAM 注册表中 sender 应用固定 id
  iam_addr: "127.0.0.1:9090"        # 入站判权透传 JWT 调 IAM CheckPermission
```

### 7.4 第 3 步：迁移建表与种子（非 AutoMigrate）

**运行时的建表/变更/种子全部由 golang-migrate 迁移 SQL 承担，失败 fail-fast；`AutoMigrate` 只用于 sqlite 内存库测试夹具，禁止出现在启动路径。** `pkg/migration/migration.go` 用 `//go:embed migrations/*.sql` 内嵌 SQL，启动时 `m.Up()`。

`sender-service/pkg/migration/migrations/000001_init.up.sql` 建：菜单/字典四表（从 contrib model 固化）、`t_message_record`、`t_message_template`，以及种子（菜单展示码、`channel`/`send_status` 字典、默认模板）。`application_id` 固定填 sender 应用 id。业务型表（记录/模板）用软删除 + 服务端审计字段；基础架构型表（菜单/字典）按 contrib 模型。

模型注册见 `pkg/model/migration.go`：`Models` 仅登记供测试夹具用的模型（contrib 菜单/字典 + 业务模型）。

### 7.5 第 4 步：业务服务与 contrib 钩子

业务服务（`message_service.go`/`template_service.go`）：编译断言 `var _ sender_v1.MessageServiceServer = (*XxxService)(nil)` + 构造函数 + model→pb 转换 + CRUD；唯一性冲突、字段校验、审计注入、按当前应用隔离。

菜单/字典不自研，用 contrib 能力 + 宿主钩子（`host_hooks.go`）：

```go
func NewMenuService(db *gorm.DB, applicationId int64) (*menu.MenuService, error) {
    return menu.NewMenuService(menu.Config{
        DB:                       db,
        ApplicationIdFromContext: fixedApplicationId(applicationId), // 固定 sender app id
        AccountIdFromContext:     authz.GetAccountId,                // 入站解出的身份仅作审计
        ValidatePermissionCode:   validateSenderPermissionCode,      // 服务段隔离：必须是 sender.*
    })
}
// 字典同理，无 ValidatePermissionCode。
```

关键点：`application_id` 取固定配置而非 token（token 的 app 是 IAM 控制台运行时切换语义，ADR-0005）；缺钩子即报错 fail-closed；菜单纯展示配置，可见性按权限码由前端过滤，服务端不做角色过滤（ADR-0004，`t_r_role_menu` 已退役）。

### 7.6 第 5 步：入站判权（薄客户端，透传 JWT）

不自建判权 machinery：contrib `pkg/authz` 薄客户端把入站 metadata（含用户 JWT）透传给 IAM `CheckPermission`，以 IAM 的 `ok` 为唯一授权来源。解出的 `accountId`/`jti` 仅作审计，不验签、不做本地角色判断（ADR-0005）。

在 `method_permissions.go` 声明 full method → 权限码映射（服务段隔离、种子见 IAM 000010）：

```go
var methodPermissions = map[string][]string{
    "/sender.v1.MessageService/SendMessage":  {"sender.record.send"},
    "/sender.v1.TemplateService/CreateTemplate": {"sender.template.create"},
    // ... 未登记方法返回 nil → fail-closed
}
var authOnlyMethods = map[string]struct{}{
    "/sender.v1.MenuService/ListMyMenuTree": {}, // 纯码门槛：服务端返回全量启用菜单，前端按码过滤
}
```

装配（`server/grpc.go`）：先跑迁移 → 建库 → 建 authz 客户端 → 挂 `authz.Interceptor` → 注册服务。出站 AK/SK 服务账号（ADR-0004）本期不实际调用，留待后续事件通知。

```go
if err := migration.Run(cfg.Database.DSN); err != nil { return err }  // fail-fast
authzClient, _ := authz.New(cfg.Sender.IamAddr)
authInterceptor := authz.Interceptor(authzClient, authz.Config{
    ApplicationId: appId, AuthOnlyMethods: service.AuthOnlyMethods(),
    MethodPerms: service.MethodPermsFor, ErrorDomain: "sender-service",
})
srv, _ := grpcserver.New(addr, grpcserver.WithDefaultInterceptors(),
    grpcserver.WithServerOptions(grpc.ChainUnaryInterceptor(authInterceptor)))
```

### 7.7 第 6 步：前端接入（admin-web 三层分区）

单一前端 `frontend-service/admin-web/`，`src` 垂直切片：`app/`（壳）/ `shared/`（公用）/ `iam`、`sender`（各服务专属域）。归属规则：只服务一个域的进该域目录，两个以上共用的进 `shared`；import 走 `@/` 别名。

- **业务 api 与页面**：`src/sender/api/sender.ts`（`messageApi`/`templateApi`，路径 `/sender/...`）、`src/sender/types/sender.ts`、`pages/RecordListPage`（发送记录+发送表单）、`pages/TemplateManagementPage`（模板 CRUD）。
- **路由子树**：`src/sender/router.tsx`，path 与菜单种子写死一致（`/sender/records`、`/sender/templates`）——菜单树按 `menu.path` 精确匹配路由，不能改；`handle.permission` 挂 `sender.*` 码。
- **菜单/字典配置页**：放 `shared/pages`，切应用后读写对应服务的库——`shared/api/app-context.ts` 存当前 app code，`ApplicationContext` 绑定成功后设置，菜单树请求以 code 门控防竞态；`menuApi`/`dictApi` 按 code 动态拼 `/iam` 或 `/sender` 前缀。
- **Vite 代理**：`/sender/` → `:8081`；因路由 path 与 API 前缀重合，`bypass` 放过 `text/html` 导航请求给 SPA。

### 7.8 第 7 步：启动验证

```bash
# 三个终端：IAM gRPC、IAM 网关、sender gRPC；sender 网关再一个终端
cd iam-service   && go run . grpc   # :9090（启动跑迁移）
cd iam-service   && go run . http   # :8080
cd sender-service && go run . grpc  # :9091（启动跑迁移）
cd sender-service && go run . http  # :8081
cd frontend-service/admin-web && pnpm dev   # :5173

# 发消息（需带登录态 JWT，经 sender 透传 IAM 判 sender.record.send 码）
curl -X POST http://127.0.0.1:8081/sender/messages \
  -H "Content-Type: application/json" -H "Authorization: Bearer <token>" \
  -d '{"recipient":"a@b.com","content":"hello","channel":"email"}'
```

## 八、新增一个接口的标准流程（防止项目失控的约束）

**每个新接口都必须走完这条链，任何一环不允许跳过或绕行：**

```
proto 契约 → 代码生成 → model(如涉新表/新列) → service 实现 → 网关(自动) → 前端 api → 页面 → 路由 handle
```

### 8.1 操作清单（Checklist）

| # | 动作 | 位置 | 强制约定 |
|---|---|---|---|
| 1 | 定义 RPC + message | `idl/proto/<domain>/xxx.proto` | 包 `<domain>.v1`；路径 `/<服务前缀>/<资源复数>[/{id}]`；CRUD 动词映射：Create=`POST /资源复数`、Get=`GET /资源复数/{id}`、List=`GET /资源复数`、Update=`PATCH /资源复数/{id} body:*` |
| 2 | 参数校验 | 同上 | 校验写 proto（`buf.validate`），不在 service 重复写 |
| 3 | 生成 | `cd idl && sh ./proto/protoc.sh` | **禁止手改 `idl/pb/`、`idl/openapiv2/` 下任何文件** |
| 4 | 模型 | `pkg/model/` | 新表才动 model：手写结构体 + `TableName()` + `init(){migrate}` + `ddl.sql` 同步；查询条件只用生成的列名常量 |
| 5 | 业务实现 | `pkg/service/xxx_service.go` | `var _ xxx_v1.XxxServiceServer = (*Service)(nil)` 编译断言；model↔pb 转换用 `xxxToPb` 函数；分页必走 `helper`；错误必走 `service_error.go` 业务码 |
| 6 | 注册（仅新 service 需要） | `server/grpc.go` / `server/http.go` | 只加 `Register` 两行；已有 service 新增方法**不需要**动 server 层 |
| 7 | 前端 API | `frontend-service/admin-web/src/<domain>/api/<domain>.ts` | 每服务一个文件；复用 `shared/api/request`；类型与 proto 响应一一对应 |
| 8 | 前端页面 | `frontend-service/admin-web/src/<domain>/pages/` | 三件套：`XxxListPage` / `CreateXxxPage` / `EditXxxPage`；错误统一 `useApiErrorNotification()` |
| 9 | 路由 | `frontend-service/admin-web/src/app/router.tsx`（各域子树单独文件） | handle 元数据驱动菜单/面包屑（见 9.3）；**不要新建静态菜单配置** |
| 10 | 代理 | `frontend-service/admin-web/vite.config.ts` | 仅新服务前缀时加一条 |

> 在已有服务上加接口 = 步骤 1/2/3/5 + 前端 7/8/9；新服务 = 第七节 + 上述全部。

### 8.2 命名与约定速查

| 维度 | 约定 | 示例 |
|---|---|---|
| proto 包 | `<domain>.v1`，且 `go_package` 必须写完整路径：`github.com/linjunhao1997/idl/pb/<domain>;<domain>_v1` | `iam.v1` / `sender.v1` |
| HTTP 前缀 | `/<服务单词>`（与 Vite 代理、菜单路由一致） | `/account`、`/message` |
| 表名 | `t_<资源>`、`t_r_<左>_<右>` | `t_user`、`t_r_user_role` |
| 模型文件 | 小写单实体 `entity.go` + `entity_generated.go` | `account.go` |
| 错误码 | 每服务一段：`<服务号><模块号><序号>` 7 位 | iam=1001xxx、sender=2001xxx |
| 前端 api 文件 | 按服务一文件 | `api/iam.ts`、`api/sender.ts` |
| 页面目录 | `<Domain>Management` | `SenderManagement/` |
| 前端路由 | `<domain>_management/<entity>_list|create_<entity>|edit_<entity>/:id` | `sender_management/sender_list` |

### 8.3 明确禁止的事

- 禁止绕过 proto 直接在 gateway 或 service 加 HTTP 路由/接口
- 禁止手写或手改 `idl/pb/`、`*_generated.go` 生成产物
- 禁止在 service 层拼接裸字符串列名（用生成常量）、裸抛 `errors.New` 代替业务错误码
- 禁止前端页面绕过 `api/` 层直接 fetch、绕过路由 handle 手写菜单
- 禁止服务之间 import 彼此的业务包（跨服务走 gRPC + 各自的 pb）

## 九、前端规范

唯一前端入口 `frontend-service/admin-web/`，`src/` 按垂直切片分四层：

| 目录 | 归属 | 内容 |
|---|---|---|
| `app/` | 应用壳 | main、App、路由聚合、IndexPage/HomePage、布局 |
| `shared/` | 两域及以上共用 | `api/`（request + 动态前缀 menu/dict）、`components/`、`hooks/`、`context/`、`types/`、`pages/`（菜单/字典管理页） |
| `iam/` | IAM 专属 | `api/iam.ts`、`components/`、`pages/`、`types/`、`router` 片段 |
| `sender/` | sender 专属 | `api/sender.ts`、`types/`、`pages/`、`router.tsx` 子树 |

**归属判定**：只服务一个服务域的放该域目录；两个及以上复用的放 `shared`。import 统一走 `@/` 别名（`@/shared/...`、`@/iam/...`、`@/sender/...`），看路径即知归属。

### 9.1 API 层

统一 `request<T>()`（`shared/api/request.ts`：Bearer token 注入、`credentials: 'include'`、非 2xx 抛 `Error(message)`）。每服务一个 api 文件，导出类型 + `xxxApi` 对象，路径按 proto 的 `/<service>/` 前缀：

```ts
// src/sender/api/sender.ts（节选）
export const messageApi = {
  send: (data: SendMessageRequest) =>
    request<MessageRecord>('/sender/messages', { method: 'POST', body: data }),
  list: ({ currentPage = 1, pageSize = 10, keyword = '', sendStatus }: ListMessagesParams = {}) =>
    request<ListMessagesResponse>(buildListUrl('/sender/records', currentPage, pageSize, keyword, { send_status: sendStatus ?? '' })),
  delete: (id: string) => request<void>(`/sender/records/${id}`, { method: 'DELETE' }),
};
```

**菜单/字典 api 例外**：`shared/api/menu-dict-api.ts` 的 `menuApi`/`dictApi` 前缀由当前 app code 动态决定——`shared/api/app-context.ts` 存 code，`ApplicationContext` 绑定成功后 `setCurrentAppCode`，菜单树请求以 code 门控防首屏竞态。因此同一套菜单/字典配置页，切到 iam 读写 IAM 库、切到 sender 读写 sender 库。

### 9.2 页面模式

复用 `shared/components/ListPageShell` + `shared/hooks/useTableQuery`（query key 与 `useSearchParams` 筛选）+ `useApiErrorNotification`。列表页典型结构：`useTableQuery` 拉数据 + `Input.Search` + `Table` + 操作列 + 创建 Modal/页面；表单 `Form.useForm()` + `useSubmitLock` 防重复提交。错误处理一律 `showNotification(error, "xxx失败")`；不引 Redux。

### 9.3 路由与菜单（服务端下发 + 前端按码过滤）

`app/router.tsx` 用 `createBrowserRouter`，根路由 `/`（`AuthWrapper` + `IndexPage` 布局）下聚合各域子树；各服务的子树独立成文件（如 `sender/router.tsx`）。

菜单数据来自服务端 `menuApi.listMyTree()`（纯码门槛，服务端返回当前应用全量启用菜单，不做角色过滤，ADR-0004）；前端按菜单 `permission_code` 与用户权限快照过滤可见性。菜单树按 `menu.path` **精确匹配**路由 path，故路由 path 必须与菜单种子写死一致（sender 即 `/sender/records`、`/sender/templates`，不可改）。

`handle` 元数据驱动面包屑与权限门槛：`label`/`breadcrumb` 用于展示，`permission` 挂权限码（如 `PERMISSIONS.senderRecordList`）。新增服务模块 = 在对应域建 `router.tsx` 子树并挂到 `app/router.tsx`，同时在 Vite 配代理：

```ts
// sender 路由 path 与 API 前缀重合，bypass 放过 text/html 导航给 SPA
'/sender/': { target: 'http://127.0.0.1:8081', changeOrigin: true, bypass(req) { /* text/html → /index.html */ } },
```

## 十、常用命令速查

| 目的 | 命令 |
|---|---|
| 生成 pb/gateway/swagger | `cd idl && sh ./proto/protoc.sh` |
| 启动某服务 gRPC / 网关 | `cd <service> && go run . grpc` / `go run . http` |
| 指定配置/端口 | `go run . grpc -c conf/config.yaml` / `go run . http --addr 127.0.0.1 --port 8080 --endpoint 127.0.0.1:9090` |
| 前端开发 | `cd frontend-service/admin-web && pnpm dev` |
