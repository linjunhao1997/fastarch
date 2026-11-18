# FastArch 管理前端（admin-web）

单一 React + TypeScript + Vite 前端，承载所有服务宿主（IAM、sender…）的管理界面。

## 目录结构（垂直切片）

```
src/
├── app/      应用壳：入口(main/App)、路由聚合(router)、布局页(Index/Home)、全局样式（登录页在 shared/pages）
├── shared/   公用：跨服务复用的基建
│   ├── api/          request（含 token/错误处理）、app-context（登录锁定的 console）、menu-dict-api（按 console 拼前缀）
│   ├── components/   通用组件（布局壳、AuthWrapper、删除确认、错误态、Suspense 包装…）
│   ├── context/      ConsoleContext（当前 console 标识与展示名，无切换）
│   ├── hooks/        通用 hooks（表格查询、错误处理、提交锁、权限/通知 Provider…）
│   ├── pages/        通用页面（登录页生成器、菜单管理、字典管理——按 console 读写对应服务的库）
│   ├── theme/        主题 token
│   ├── types/        公用类型（应用、菜单、字典、权限码、文案）
│   └── utils/        工具函数
├── iam/      IAM 服务专属（自包含）
│   ├── api/iam.ts
│   ├── components/   IAM 专属组件（分配应用、绑定角色…）
│   ├── pages/        用户/应用/认证/授权管理
│   ├── types/        IAM 专属类型
│   └── （路由挂在 app/router.tsx 的 IAM 子树）
└── sender/   sender 服务专属（自包含）
    ├── api/sender.ts
    ├── types/sender.ts
    ├── pages/        发送记录、消息模板
    └── router.tsx    sender 路由子树（path 与菜单种子对齐：/sender/records、/sender/templates）
```

**归属判定**：某段代码只服务一个业务域 → 放该域目录（iam/ 或 sender/）；两个及以上域复用 → 放 shared/。
import 统一走 `@/` 别名（指向 src/），从路径即可看出归属。

## 关键机制

- **登录锁定控制台（ADR-0007）**：`/login`（IAM）与 `/sender/login`（sender）分别登录，成功后 `setConsole` 写入 `app_console`，应用上下文不再变化；跨控制台必须重新登录。`shared/api/app-context.ts` 读取 console，菜单/字典 API 据此拼 `/iam/...` 或 `/sender/...` 前缀，React Query 缓存键带 console 隔离。
- **双控制台路由**：`/` 与 `/sender` 是两棵受保护子树（`AuthWrapper consoleKind=...`），共用 `IndexPage` 布局；sender 业务路由在 `/sender/records`、`/sender/templates`。
- **Vite 代理**：`/iam/`、`/account/` 等 → :8080；`/sender/` → :8081。`/sender/` 用 bypass 放过 HTML 导航（路由 path 与 API 前缀重合）。
- **权限码**：`shared/types/permissions.ts`，与后端各服务的 `method_permissions.go` 词汇对齐。菜单可见性为纯码门槛（ADR 0002）。

## 开发

```bash
pnpm install
pnpm dev      # http://localhost:5173
pnpm build
pnpm test
```
