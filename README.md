# FastArch 微服务脚手架

基于 Go + React 的微服务工程模板，IDL 驱动（Protobuf → gRPC + grpc-gateway + OpenAPI），业务按 DDD 分层组织。内置首个业务服务 **iam-service**（身份与权限管理），新服务（如 sender-service）按照统一模板接入即可。

> 各模块更细的说明见：[iam-service/readme.md](iam-service/readme.md) · [idl/readme.md](idl/readme.md) · [contrib/readme.md](contrib/readme.md)

## 一、总体架构

```
┌───────────────────────────────────────────────────────────────┐
│  前端 frontend-service/iam-web :5173                          │
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
              │  MySQL (AutoMigrate)    │
              │  Redis (token/权限缓存)  │
              └─────────────────────────┘
```

**仓库 = 3 个 Go module + 1 个前端**

| 目录 | module | 职责 |
|---|---|---|
| `idl/` | `idl`（对外名 `github.com/linjunhao1997/idl`） | **接口层**：proto 定义 + 生成的 pb/gateway/swagger 代码 + 公共拦截器库（`lib/grpclib`、`lib/gwlib`） |
| `contrib/` | `github.com/linjunhao1997/contrib` | **共享基础设施**：配置加载、cobra 应用引导、grpcserver/httpserver 组装、MySQL/Redis 助手、代码生成器 |
| `iam-service/` | `iam-service` | 业务服务（每个业务服务一个这样的目录） |
| `frontend-service/iam-web/` | — | React 前端 |

依赖方向：`iam-service → idl + contrib`，`contrib → idl`，均通过 `replace ../` 指向本地。**idl 被所有服务依赖，contrib 被所有服务复用，业务服务之间不互相依赖**（需要跨服务调用走 gRPC）。

## 二、DDD 分层映射

项目采用"轻量 DDD"：接口契约由 proto 承担，领域逻辑收敛在 service 层，持久化收敛在 model 层。

| DDD 概念 | 本项目落点 | 规则 |
|---|---|---|
| 接口层（Interface） | `idl/proto/<domain>/*.proto`（HTTP annotation）+ 生成的 `*_pb.gw.go` | 只定义契约，不含逻辑；校验写在 proto（`buf.validate`） |
| 应用层（Application） | `iam-service/pkg/service/*_service.go` | 实现 pb 生成的 `XxxServiceServer` 接口；做 DTO(model↔pb) 转换、编排、事务边界 |
| 领域/持久层（Domain/Infra） | `iam-service/pkg/model/` + `pkg/helper/` | GORM 模型 + 生成的列名常量；helper 提供分页等横切工具 |
| 共享内核（Shared Kernel） | `idl/lib/*`、`contrib/pkg/*` | 拦截器、错误转换、配置、服务组装——业务服务只"用"不"改" |
| 前端（BFF 视角） | `frontend-service/iam-web/src/{api,pages,routers}` | api 按服务一个文件，页面三件套，路由即菜单 |

## 三、目录结构

```
fastarch/
├── idl/                          # 接口定义与生成代码（独立 module）
│   ├── proto/
│   │   ├── buf.yaml / buf.gen.yaml / protoc.sh   # buf 代码生成
│   │   ├── common/common.proto   # 公共消息: Page、GetRequest
│   │   └── iam/                  # iam 领域: account / application / authentication / authorization
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
│   │   └── gen/                  # gen_gorm_mysql_repository 代码生成器
│   └── main.go                   # 生成器入口
│
├── iam-service/                  # 业务服务模板（新服务照此复制）
│   ├── main.go                   # → cmd.Execute()
│   ├── cmd/root.go               # app.New[Config] + AddCommand("grpc"/"http")
│   ├── conf/                     # config.go + config.yaml
│   ├── server/
│   │   ├── grpc.go               # RegisterXxxServiceServer + 拦截器链 + AutoMigrate
│   │   └── http.go               # RegisterXxxServiceHandlerFromEndpoint
│   └── pkg/
│       ├── auth/                 # JWT + 鉴权拦截器（按服务可裁剪）
│       ├── constant/             # 错误码前缀、cache key
│       ├── helper/               # page.go 分页封装
│       ├── model/                # 手写模型 + *_generated.go(列名常量) + ddl.sql
│       └── service/              # 业务实现 + service_error.go 业务错误
│
└── frontend-service/iam-web/     # React 前端
    └── src/
        ├── api/                  # request.ts 统一封装 + iam.ts（每服务一个文件: xxxApi）
        ├── pages/<XxxManagement>/ # XxxListPage / CreateXxxPage / EditXxxPage 三件套
        ├── routers/router.tsx    # createBrowserRouter + handle 元数据（菜单/面包屑自动生成）
        ├── components/           # AuthWrapper、AutoBreadcrumb
        └── hooks/                # useNotification、useApiError
```

## 四、技术栈

| 层级 | 技术 |
|---|---|
| 前端 | React 19 + Ant Design + Vite + TypeScript + React Router v7（createBrowserRouter） |
| 接口 | Protobuf + buf，`google.api.http` annotation，`buf.validate` 参数校验 |
| 通信 | gRPC + grpc-gateway（REST 双协议） |
| 后端 | Go 1.21+，cobra CLI，viper 配置，GORM，JWT，Redis |
| 存储 | MySQL（AutoMigrate）+ Redis |

## 五、快速开始（从零部署）

### 5.0 环境要求

| 依赖 | 版本 | 用途 |
|---|---|---|
| Go | 1.21+ | 后端编译运行 |
| Node | 20+ | 前端构建 |
| pnpm | 8+ | 前端包管理 |
| MySQL | 8 | 业务数据（AutoMigrate 自动建表） |
| Redis | 7 | token/权限快照缓存 |

> buf 工具链（`buf`、`protoc-gen-*`）只在**修改 proto 重新生成代码**时才需要，纯部署可跳过；生成产物（`idl/pb/`、`idl/openapiv2/`）已随仓库提交。

### 5.1 数据库准备

只需建库（建表由服务启动时的 AutoMigrate + 种子数据自动完成）：

```sql
CREATE DATABASE IF NOT EXISTS iam_dev DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

然后确认 `iam-service/conf/config.yaml` 的 `database.dsn`、`redis.addr` 与本机环境一致。

### 5.2 注入凭证加密密钥（必须）

AK/SK 访问凭证功能要求注入 32 字节（hex/base64）加密密钥，**缺失时服务启动直接报错**（fail-fast）：

```powershell
# Windows（用户级，重开终端生效）
[Environment]::SetEnvironmentVariable('IAM_CREDENTIAL_ENCRYPT_KEY', '<32字节hex密钥>', 'User')

# Linux/macOS（写入 ~/.bashrc 或 ~/.zshrc）
export IAM_CREDENTIAL_ENCRYPT_KEY='<32字节hex密钥>'
```

> 密钥可自行生成（如 `openssl rand -hex 32`）。注意：同一套库换密钥后，已落库的旧凭证密文将无法解密，需重新生成凭证。密钥只走环境变量，**禁止写入 config.yaml 入仓**。

### 5.3 启动后端（两个终端）

```bash
cd iam-service
go run . grpc      # gRPC  :9090（启动时自动 AutoMigrate + 种子数据）
go run . http      # 网关  :8080 → 9090（需先启动 gRPC）
```

### 5.4 初始化管理员账号（仅全新库需要一次）

种子数据只建应用/角色/权限，**不建账号**；而 `CreateAccount` 接口又要求登录态，全新库直接调接口会死锁。首次部署请执行以下 SQL 引导一个管理员（之后所有账号在页面上管理）：

```sql
-- 密码 Admin@123456（salt=fastarch-boot-ad，sha256(password+salt)）
INSERT INTO t_account (id, code, name, email, status, salt, password, ak, sk, kind, create_time, update_time, created_by, updated_by, version)
VALUES (1, 'boot-admin', 'Administrator', 'admin@fastarch.local', 'enabled',
        'ZmFzdGFyY2gtYm9vdC1hZA==', '28e29d4f2ab50620e8b291b06e2b744ddc1d2692ff13882a4ec5484d863b0fd1',
        '', '', '0', NOW(3), NOW(3), 0, 0, 1);

-- 绑定内置管理员角色（code=1000，由种子数据创建）
INSERT INTO t_r_account_role (id, account_id, role_id)
VALUES (1, 1, (SELECT id FROM t_role WHERE code = '1000'));

-- 绑定种子应用，登录后可进入应用上下文
INSERT INTO t_r_account_application (id, account_id, application_id)
VALUES (1, 1, (SELECT id FROM t_application LIMIT 1));
```

**登录后请立即在「账号管理」中修改该账号密码，并新建日常使用的管理员账号、禁用/删除引导账号。**

### 5.5 启动前端

```bash
cd frontend-service/iam-web
pnpm install
pnpm dev           # :5173，代理 /account /application /authorization /authentication → :8080
```

浏览器打开 http://localhost:5173 ，用引导账号登录即可。

配置文件示例（`iam-service/conf/config.yaml`）：

```yaml
app:
  name: iam-service
database:
  dsn: "root:password@tcp(127.0.0.1:3306)/iam_dev?charset=utf8mb4&parseTime=True&loc=Local"
redis:
  addr: "localhost:6379"
grpc:  { addr: "127.0.0.1", port: 9090 }
http:  { addr: "127.0.0.1", port: 8080, endpoint: "127.0.0.1:9090" }
jwt:   { expiration_hours: 24 }
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

模型侧还有一个生成器（列名常量）：

```bash
cd <service目录>
go run ../contrib/main.go gen_gorm_mysql_repository -m pkg/model/account.go
# 生成 account_generated.go: AccountColumnId / AccountColumnName / ...
# 传 -i <接口文件> 还可生成 Repository 实现（当前 iam-service 未用，service 直接操作 *gorm.DB）
```

### 6.2 gRPC 拦截器链

`grpcserver.WithDefaultInterceptors()`（contrib）依序挂载：

1. **Recovery**（`idl/lib/grpclib`）— panic → Internal 错误
2. **Validation** — `protovalidate` 自动校验 proto 里的 `buf.validate` 规则
3. **ErrorDetails** — 业务错误（实现 `BusinessError` 接口）→ gRPC status，`ErrorInfo{reason=业务码, domain=服务名}`
4. **GormErrorMapping** — gorm `ErrRecordNotFound` → NotFound
5. **业务 AuthInterceptor** — JWT 校验 + Redis 权限表（`account:<id>:permissions`）按 `domain:<fullMethodName>` 鉴权，白名单方法放行（如登录）

业务错误统一定义在 `pkg/service/service_error.go`：`Error{Code 业务码, grpcCode HTTP语义码, Err}`，业务码按服务分段（iam 用 1001xxx），前端从 HTTP 错误响应的 `message` / `details` 中取文案。

### 6.3 分页

proto 列表请求统一带 `current_page` / `page_size`，响应统一 `common.v1.Page`；service 层用 `helper.GetLimitAndOffset` + `helper.GetPage`（默认 page=1/size=10/max=100）。

### 6.4 HTTP 网关

每个服务一个独立网关进程（`go run . http`），`RegisterXxxServiceHandlerFromEndpoint` 反向代理到自己的 gRPC。`gwlib` 负责 cookie 中的 `Authorization` → gRPC metadata 透传。前端通过 Vite 代理按 **服务前缀** 转发到对应网关。

## 七、新增一个微服务（以 sender-service 为例）

> 完整对照 iam-service 的实际模式。目标：消息发送服务，实体 `MessageRecord`，gRPC `:9091`，网关 `:8081`，URL 前缀 `/message`。

### 第 1 步：定义 proto — `idl/proto/sender/message.proto`

```protobuf
syntax = "proto3";
package sender.v1;
option go_package = "github.com/linjunhao1997/idl/pb/sender;sender_v1";

import "common/common.proto";
import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "google/protobuf/empty.proto";
import "buf/validate/validate.proto";

service MessageService {
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse) {
    option (google.api.http) = { post: "/message/messages" body: "*" };
  }
  rpc GetMessageRecord(common.v1.GetRequest) returns (MessageRecord) {
    option (google.api.http) = { get: "/message/records/{id}" };
  }
  rpc ListMessageRecords(ListMessageRecordsRequest) returns (ListMessageRecordsResponse) {
    option (google.api.http) = { get: "/message/records" };
  }
}

message SendMessageRequest {
  string channel = 1 [(buf.validate.field).string = {in: ["email", "sms"]}];
  string receiver = 2 [(buf.validate.field).string.min_len = 1];
  string subject = 3;
  string content = 4 [(buf.validate.field).string.min_len = 1];
}
message SendMessageResponse { string record_id = 1; }

message ListMessageRecordsRequest {
  int64 current_page = 1;
  int64 page_size = 2;
  string channel = 3;
}
message ListMessageRecordsResponse {
  common.v1.Page page = 1;
  repeated MessageRecord records = 2;
}

message MessageRecord {
  string id = 1;
  string channel = 2;
  string receiver = 3;
  string status = 4;      // pending / sent / failed
  string content = 5;
}
```

**proto 约束**（见第八节规范）：包名 `<domain>.v1`；HTTP 路径 `/<服务前缀>/<资源复数>`；Get 复用 `common.v1.GetRequest`；列表响应必带 `common.v1.Page`；Update 返回 `google.protobuf.Empty`；校验用 `buf.validate` 写在 proto。

### 第 2 步：生成代码

```bash
cd idl && sh ./proto/protoc.sh
# 产出 idl/pb/sender/ 与 idl/openapiv2/sender/
```

### 第 3 步：创建服务骨架

```
sender-service/
├── main.go                  # package main → cmd.Execute()
├── cmd/root.go
├── conf/{config.go, config.yaml}
├── server/{grpc.go, http.go}
└── pkg/
    ├── model/{migration.go, message_record.go, ddl.sql}
    ├── helper/page.go       # 拷贝 iam-service，或后续下沉到 contrib
    └── service/{message_service.go, service_error.go}
```

`go.mod`：

```bash
cd sender-service
go mod init sender-service
go mod edit -replace github.com/linjunhao1997/idl=../idl
go mod edit -replace github.com/linjunhao1997/contrib=../contrib
go get github.com/linjunhao1997/idl github.com/linjunhao1997/contrib && go mod tidy
```

**main.go + cmd/root.go**（contrib 统一引导，端口默认值在这里给）：

```go
// cmd/root.go
package cmd

import (
    "github.com/linjunhao1997/contrib/pkg/app"
    "sender-service/conf"
    "sender-service/server"
)

func Execute() {
    a := app.New[conf.Config]("sender-service",
        app.WithConfigDefaults(map[string]any{
            "grpc.addr": "127.0.0.1", "grpc.port": 9091,
            "http.addr": "127.0.0.1", "http.port": 8081,
        }),
    )
    a.AddCommand("grpc", "启动gRPC服务器", server.RunGrpcServer)
    a.AddCommand("http", "启动HTTP服务器", server.RunHttpServer)
    a.Execute()
}
```

**conf/config.go**：按需嵌入 contrib 通用配置段（`config.App/Database/Redis/GRpc/Http`），服务特有配置（如 SMTP）自己加段。`conf/config.yaml` 里 `http.endpoint` 指向自己的 gRPC 端口。

### 第 4 步：模型层 — `pkg/model/message_record.go`

```go
package model

import "time"

//go:generate contrib gen_gorm_mysql_repository -m message_record.go
type MessageRecord struct {
    Id         int64     `gorm:"column:id;primaryKey"`
    Channel    string    `gorm:"column:channel"`   // email / sms
    Receiver   string    `gorm:"column:receiver"`
    Subject    string    `gorm:"column:subject"`
    Content    string    `gorm:"column:content"`
    Status     string    `gorm:"column:status"`    // pending / sent / failed
    CreateTime time.Time `gorm:"column:create_time"`
    UpdateTime time.Time `gorm:"column:update_time"`
}

func (m *MessageRecord) TableName() string { return "t_message_record" }

func init() { migrate(MessageRecord{}) } // 注册进 Models，供 AutoMigrate
```

生成列名常量（含 `//go:generate` 可直接 `go generate ./...`）：

```bash
go run ../contrib/main.go gen_gorm_mysql_repository -m pkg/model/message_record.go
```

`ddl.sql` 按 iam-service 规范同步维护：主表 `t_<资源>`、关联表 `t_r_<左>_<右>`、主键 `id bigint AUTO_INCREMENT`、时间 `create_time/update_time datetime(3)`。

### 第 5 步：业务层 — `pkg/service/`

**service_error.go**：错误码换段（sender 用 2001xxx），结构照抄 iam-service（实现 `BusinessError` + `GRPCCode`，交给 `ErrorInterceptor` 转换）：

```go
var (
    ErrSendFailed   = NewErrorWithCode("2001001", codes.Internal, errors.New("send message failed"))
    ErrChannelNotSupported = NewErrorWithCode("2001002", codes.InvalidArgument, errors.New("channel not supported"))
)
```

**message_service.go**（固定骨架：编译断言 + 构造函数 + model→pb 转换函数 + CRUD 实现）：

```go
type MessageService struct {
    db *gorm.DB
}

var _ sender_v1.MessageServiceServer = (*MessageService)(nil)

func NewMessageService(db *gorm.DB) sender_v1.MessageServiceServer {
    return &MessageService{db: db}
}

func messageRecordToPb(m model.MessageRecord) *sender_v1.MessageRecord { /* 字段映射 */ }

func (s *MessageService) ListMessageRecords(ctx context.Context, req *sender_v1.ListMessageRecordsRequest) (*sender_v1.ListMessageRecordsResponse, error) {
    limit, offset := helper.GetLimitAndOffset(req.CurrentPage, req.PageSize)
    var count int64
    var res []model.MessageRecord
    err := s.db.WithContext(ctx).Model(model.MessageRecord{}).Scopes(func(db *gorm.DB) *gorm.DB {
        if len(req.Channel) > 0 {
            db = db.Where(model.MessageRecordColumnChannel, req.Channel) // 用生成的列名常量
        }
        return db
    }).Count(&count).Limit(limit).Offset(offset).Find(&res).Error
    if err != nil {
        return nil, err
    }
    records := make([]*sender_v1.MessageRecord, 0, len(res))
    for _, v := range res {
        records = append(records, messageRecordToPb(v))
    }
    return &sender_v1.ListMessageRecordsResponse{
        Records: records,
        Page:    helper.GetPage(req.CurrentPage, req.PageSize, count),
    }, nil
}
```

### 第 6 步：装配 gRPC + 网关 — `server/grpc.go` / `server/http.go`

```go
// server/grpc.go
func RunGrpcServer(ctx context.Context, cfg *conf.Config) error {
    srv, err := grpcserver.New(cfg.GRpc.AddrWithPort(),
        grpcserver.WithDefaultInterceptors(),
        grpcserver.WithErrorDomain(cfg.App.Name),
        grpcserver.WithGormErrorMapping(),
        // 如需鉴权：grpcserver.WithServerOptions(grpc.ChainUnaryInterceptor(auth.Interceptor(rdb)))
    )
    if err != nil {
        return err
    }
    db, err := database.NewMySQL(cfg.Database.DSN)
    if err != nil {
        return err
    }
    if err := db.AutoMigrate(model.Models...); err != nil {
        return err
    }
    s := srv.Grpc()
    sender_v1.RegisterMessageServiceServer(s, service.NewMessageService(db))
    return srv.Run(ctx)
}

// server/http.go
func RunHttpServer(ctx context.Context, cfg *conf.Config) error {
    dialOptions := []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
    }
    mux := httpserver.NewMux()
    if err := sender_v1.RegisterMessageServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
        return err
    }
    return httpserver.Run(ctx, cfg.Http.AddrWithPort(), mux)
}
```

### 第 7 步：启动验证

```bash
go run . grpc   # 127.0.0.1:9091
go run . http   # 127.0.0.1:8081
curl -X POST http://127.0.0.1:8081/message/messages \
  -H "Content-Type: application/json" \
  -d '{"channel":"email","receiver":"a@b.com","subject":"hi","content":"hello"}'
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
| 7 | 前端 API | `frontend-service/iam-web/src/api/<domain>.ts` | 每服务一个文件；复用统一 `request<T>`；类型与 proto 响应一一对应 |
| 8 | 前端页面 | `frontend-service/iam-web/src/pages/<XxxManagement>/` | 三件套：`XxxListPage` / `CreateXxxPage` / `EditXxxPage`；错误统一 `useApiErrorNotification()` |
| 9 | 路由 | `frontend-service/iam-web/src/routers/router.tsx` | handle 元数据驱动菜单/面包屑（见 9.3）；**不要新建静态菜单配置** |
| 10 | 代理 | `frontend-service/iam-web/vite.config.ts` | 仅新服务前缀时加一条 |

> 在已有服务上加接口 = 步骤 1/2/3/5 + 前端 7/8/9；新服务 = 第七节 + 上述全部。

### 8.2 命名与约定速查

| 维度 | 约定 | 示例 |
|---|---|---|
| proto 包 | `<domain>.v1`，且 `go_package` 必须写完整路径：`github.com/linjunhao1997/idl/pb/<domain>;<domain>_v1` | `iam.v1` / `sender.v1` |
| HTTP 前缀 | `/<服务单词>`（与 Vite 代理、菜单路由一致） | `/account`、`/message` |
| 表名 | `t_<资源>`、`t_r_<左>_<右>` | `t_account`、`t_r_account_role` |
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

### 9.1 API 层

统一 `request<T>()`（fetch 封装：Bearer token 注入、`credentials: 'include'`、非 2xx 抛 `Error(message)`，消息取自 gateway 错误体的 `message`/`error`）。每个服务一个文件，导出类型 + `xxxApi` 对象：

```ts
// src/api/sender.ts
export interface Sender { /* 与 proto 响应对应 */ }

export const senderApi = {
  create: (data: CreateSenderReq) => request<Sender>('/message/messages', { method: 'POST', body: data }),
  list:   (current_page = 1, page_size = 10) =>
            request<ListSendersResponse>(`/message/records?current_page=${current_page}&page_size=${page_size}`),
  get:    (id: number) => request<Sender>(`/message/records/${id}`),
  update: (id: number, data: Partial<Sender>) => request<void>(`/message/records/${id}`, { method: 'PATCH', body: { id, ...data } }),
};
```

> `request` 统一定义在 `src/api/request.ts`（公共封装：token 注入、错误解析），各服务的 api 文件直接 import 使用。

### 9.2 页面三件套

- **ListPage**：`useState(data/loading/filters)` + `useEffect` 拉取 + `Input.Search`（内存过滤）+ `Table` + 操作列 `Button type="link"` 跳编辑 + 创建按钮 `navigate` 跳创建
- **CreatePage**：`Form.useForm()` + `layout="vertical"` + `Card` 包裹 + `submitting` 状态 + 成功 `message.success` 后 `navigate` 回列表
- **EditPage**：同 CreatePage，外加 `useParams<{id}>()` 取 id、`useEffect` 调 `api.get` 后 `form.setFieldsValue` 回显

错误处理一律：`const showNotification = useApiErrorNotification();` → `catch (error) { showNotification(error, "xxx失败"); }`。状态管理用 `useState`（不引 Redux）。

### 9.3 路由与菜单（handle 元数据驱动）

`src/routers/router.tsx` 用 `createBrowserRouter`，根路由 `/`（`AuthWrapper` + `IndexPage` 布局）下按模块挂子树。**菜单和面包屑由 `handle` 自动生成**（`convertRoutesToMenuItems` + `AutoBreadcrumb`），规则：

| handle 字段 | 效果 |
|---|---|
| `key` + `label` (+`icon`) | 出现在侧边栏菜单 |
| 仅 `breadcrumb` | 只出现在面包屑（创建/编辑页用这个） |

新增模块 = 在 children 里加一棵子树：

```tsx
{
  path: "sender_management",
  handle: { key: "sender_management", icon: <SendOutlined />, label: "发送管理", breadcrumb: "发送管理" },
  children: [
    { path: "record_list", handle: { key: "record_list", label: <Link to="/sender_management/record_list">发送记录</Link>, breadcrumb: "发送记录" }, element: <SenderListPage /> },
    { path: "create_record",  handle: { breadcrumb: "发送消息" }, element: <CreateSenderPage /> },
    { path: "edit_record/:id", handle: { breadcrumb: "编辑记录" }, element: <EditSenderPage /> },
  ],
}
```

最后在 `vite.config.ts` 加代理（仅新服务需要）：

```ts
'/message': { target: 'http://127.0.0.1:8081', changeOrigin: true },
```

## 十、常用命令速查

| 目的 | 命令 |
|---|---|
| 生成 pb/gateway/swagger | `cd idl && sh ./proto/protoc.sh` |
| 生成模型列名常量 | `go run ../contrib/main.go gen_gorm_mysql_repository -m pkg/model/xxx.go`（`go generate ./...` 亦可） |
| 启动某服务 gRPC / 网关 | `cd <service> && go run . grpc` / `go run . http` |
| 指定配置/端口 | `go run . grpc -c conf/config.yaml` / `go run . http --addr 127.0.0.1 --port 8080 --endpoint 127.0.0.1:9090` |
| 前端开发 | `cd frontend-service/iam-web && pnpm dev` |
