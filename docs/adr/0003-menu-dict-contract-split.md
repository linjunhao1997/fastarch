# 菜单/字典契约重构：消息共享 common.menu.v1/common.dict.v1，service 每服务自持，路径前缀路由

菜单/字典的**消息**迁到中性共享包 `common.menu.v1` / `common.dict.v1`；**service 定义每服务自持**（`iam.v1.MenuService`、`order.v1.MenuService`…），引用共享消息，HTTP 路径以各自服务码为前缀（`/iam/menu/…`、`/order/menu/…`）写进各自 proto 注解。多宿主按 URL 路径前缀路由（vite/nginx），URL 与 proto 注解一致。修订 ADR 0001"契约共享 iam.v1.MenuService/DictService"条款。

## 为什么

ADR 0001 让各宿主共享 `iam.v1.MenuService` 契约。第二宿主（order）接入时暴露两个问题：

- **命名拧巴**：order 的 METHOD_PERMISSIONS 须把 `/iam.v1.MenuService/CreateMenu` 映射到 `order.menu.create`——iam 的方法名配 order 的码。
- **路由 wrinkle**：共享 service 即共享 HTTP 路径，多宿主区分只能靠 header 或网关挂载前缀（URL ≠ proto 注解）。

## 决策（2026-11-17 用户确认）

- **消息共享**：`common/menu/menu.proto`（package `common.menu.v1`）、`common/dict/dict.proto`（package `common.dict.v1`）。Menu/DictType 等消息及校验规则迁此。
- **service 每服务自持**：iam/order 各自声明 MenuService/DictService，rpc 引用共享消息，HTTP 路径带服务前缀。
- **contrib 共享不破**：contrib 按共享消息实现；各宿主 service 接口方法签名相同（消息类型一致），Go 结构化类型使 contrib 同时满足 iam/order 接口（前提 `require_unimplemented_servers=false`，本项目正是），无需适配代码。
- **路由按路径前缀**：vite（dev）/nginx（prod）按 `/iam/`→iam 网关、`/order/`→order 网关转发。无独立联邦网关进程，无 header。

## 命名考量

- `common.menu.v1`/`common.dict.v1`：common 伞下、v1 收尾，合本项目 `<domain>.v1` 约定（版本化叶子域）。
- 否决 `common.v1` 平铺：common 保持通用原语（Page/GetRequest），不混入领域消息。
- 否决 `common.v1.menu`：v1 夹在中间，版本落在 common 而非菜单域，违约定。
- 否决共享 `common.v1.MenuService`（service 也共享）：路径共享会退回路由 wrinkle；service 自持才能把路径写进 proto。

## Consequences

- iam 菜单/字典 HTTP 路径 `/menu/…`→`/iam/menu/…`、`/dict/…`→`/iam/dict/…`；前端 api 与 vite 代理已同步。
- **gRPC full method 名不变**（`/iam.v1.MenuService/…`）：METHOD_PERMISSIONS、鉴权白名单零改动。
- order 接入：声明 `order.v1.MenuService/DictService`（引用共享消息，路径 `/order/…`）+ 注册 contrib 实现即可，消息与前端类型复用。
- 代价：每服务须重声明 service 块（rpc 清单 + 路径注解），iam 加 RPC 时 order 需镜像（漂移风险，可用代码生成模板缓解，MVP 手抄）。
- 消息包目录须与 go_package 目录一致（buf `paths=source_relative`）：故 proto 置于 `common/menu/`、`common/dict/` 子目录。
