# contrib

工具集与各服务共用的启动基础设施（独立 Go module：`github.com/linjunhao1997/contrib`）。

## 代码生成器

GORM MySQL Repository 代码生成器，根据模型接口自动生成 repository 实现：

```bash
go run main.go gorm_mysql_repository -i <interface_file_path> -m <model_file_path>
```

## 服务启动基础设施

各微服务共用的配置加载、命令引导、资源初始化与服务启停能力。新服务接入示例见 `iam-service`。

| 包 | 用途 |
|---|---|
| `pkg/config` | 通用配置段（App/GRpc/Http/Database/Redis/Jwt）与泛型加载器 `Load[T]`；基于 viper，支持默认值、多路径兜底查找 |
| `pkg/app` | 基于 cobra 的应用引导：统一 `-c/--config` flag（只注册一次，避免子命令默认值覆盖）、启动前加载配置、为命令注入信号感知 context（SIGINT/SIGTERM 优雅退出） |
| `pkg/database` | gorm MySQL 连接助手 `NewMySQL(dsn)`，建连后 Ping fail-fast |
| `pkg/cache` | Redis 客户端助手 `NewRedis(addr)` |
| `pkg/server/grpcserver` | gRPC 服务组装：默认拦截器链（Recovery + Validation + 业务错误 ErrorInfo + 可选 gorm 错误映射，来自 `idl/lib/grpclib`）、监听、Serve、信号驱动的 GracefulStop（10s 超时强杀）。选项：`WithErrorDomain(服务名)` 业务错误码透传、`WithGormErrorMapping()` 记录不存在→404 |
| `pkg/server/httpserver` | grpc-gateway mux 组装（header matcher / metadata 来自 `idl/lib/gwlib`）与 HTTP 优雅启停（`http.Server.Shutdown`） |

### 新服务怎么接

```go
// 1. 服务自己的 conf 包：Config 嵌入通用段 + 自有配置段
type Config struct {
    config.App      `mapstructure:"app"`
    config.Database `mapstructure:"database"`
    config.Redis    `mapstructure:"redis"`
    config.GRpc     `mapstructure:"grpc"`
    // ...按需嵌入 config.Http / config.Jwt，以及自己的段
}

// 2. main.go / cmd：一条引导链路
a := app.New[conf.Config]("foo-service")
a.AddCommand("grpc", "启动gRPC服务器", func(ctx context.Context, cfg *conf.Config) error {
    rdb := cache.NewRedis(cfg.Redis.Addr)
    srv, err := grpcserver.New(cfg.GRpc.AddrWithPort(), grpcserver.WithDefaultInterceptors())
    if err != nil {
        return err
    }
    // srv.Grpc() 上注册自己的服务……
    return srv.Run(ctx)
})
a.Execute()
```

### 配置文件查找顺序

`-c/--config` 默认 `conf/config.yaml`；找不到时依次尝试：

1. `-c` 指定路径（或当前工作目录下的 `conf/config.yaml`）
2. `<服务名>/conf/config.yaml`（monorepo 根目录启动，如 IDE 默认工作目录）
3. `WithConfigSearchPaths` 指定的额外路径
4. 可执行文件同目录的 `conf/config.yaml`（部署时二进制旁放配置）

> 注意：本模块对 `github.com/linjunhao1997/idl` 的依赖通过 `replace => ../idl` 解析；使用方服务（如 iam-service）同样需要在自己的 go.mod 中配置 replace。
