# 任务工作台（flow-service）实现说明与验证手册

> 配套方案：`docs/flow-workbench-proposal.md`（设计与取舍）。本文只讲**怎么跑、怎么验**。

## 1. 已落地的东西

| 层 | 内容 |
| --- | --- |
| 契约 | `idl/proto/flow/{member,task,question,log,menu,dict}.proto`，已 buf 生成网关与 swagger |
| IAM | `000017_flow.up.sql`：注册应用 `flow`(id=3) + 22 个 `flow.*` 权限码 |
| 后端 | `flow-service/`：6 个服务、6 张业务表 + 1 张关系表、种子数据、MCP stdio 服务 |
| 前端 | `admin-web/src/flow/`：任务工作台 `/flow/tasks`、问题中心 `/flow/questions`，并注册为第三个控制台 |
| 菜单 | flow 库内种子：`/flow/tasks`、`/flow/questions`（前端按菜单树渲染侧边栏） |

**状态分层**：`t_task.status` 只保留终态 `open/done/discarded`；`ready/blocked` 由 `EXISTS(open question)` 在查询期派生；v1.4 的执行态 `in_progress/submitted` 由 `started_at/submitted_at` 派生。闭环只改 `t_question` 一行，任务自动解冻，无需同步代码。

## 2. 启动顺序（本地）

前置：MySQL 3306、Redis（当前服务未强依赖）、IAM 已初始化过 `fastarch_dev`。

```powershell
# 0) 建 flow 自己的库（只需一次）
cd D:\fastarch\flow-service
go run ./tools/dbctl -create flow_dev

# 1) IAM（应用迁移 000017 + 判权 + 登录）
cd D:\fastarch\iam-service
go run . grpc      # :9090
go run . http      # :8080（另开一个终端）

# 2) flow
cd D:\fastarch\flow-service
go run . grpc      # :9092，首次启动自动建表 + 种子
go run . http      # :8082（另开一个终端）

# 3) 前端
cd D:\fastarch\frontend-service\admin-web
pnpm dev           # 浏览器打开 http://localhost:5173
```

> 前端是单工程多控制台：登录后进入「控制台选择」页，能看到第三张卡「任务工作台」。
> 若 5173 报 `EACCES`（Windows 保留端口/权限问题），换端口：`pnpm dev --port 5180 --host 127.0.0.1`。
> `/flow/` 代理已配置到 8082；`/flow/tasks` 同时是页面路由，vite 已按 `Accept: text/html` 放行给 SPA。

## 3. 我（单人兼全部角色）怎么用

`conf/config.yaml` 默认：

```yaml
flow:
  bootstrap_all_roles: true   # 首次登录自动建档并授予全部 7 个角色
  default_role_code: "dev"
```

所以用 admin 登录后第一次访问 `/flow/me` 会自动建档：持有
`boss / analyst / pm / arch / dev / qa / ops`，当前角色 `dev`。

**注意**：正式环境把 `bootstrap_all_roles` 改成 `false`，角色改由管理员分配（接口已按"只允许切到自己持有的角色"实现）。

## 4. 已插入的“我这条需求”

`000001_init.up.sql` 里把它作为 **任务 #1（dev 角色）** 落库，brief 就是完整需求与 4 条验收。

| 数据 | 内容 |
| --- | --- |
| 任务 #1 | 实现任务工作台前后端并接入 MCP 验证（dev）→ 被 2 个问题阻塞 |
| 问题 #1 | 首屏展示范围？→ 问 **pm**，3 个预填选项，A 推荐 |
| 问题 #2 | MCP 身份方案？→ 问 **arch**，2 个预填选项，A 推荐 |
| 任务 #2 | 补齐接口联调记录（dev）→ **ready**，可开工 |
| 任务 #3 | 回归主流程（qa）→ 被问题 #3 阻塞，问 **dev** 且 **必须给证据** |
| 任务 #4 | 汇总需求与技术方案（arch）→ ready |

这组数据就是给「一人多角色 + 阻塞/解冻 + 证据门槛」准备的最小验证集。

## 5. 接 MCP 验证（重点）

MCP 接入点是 flow-service 自己的接口：`POST /flow/mcp`，与业务网关同端口（8082），无状态、
多人共用。**身份由每个请求自己的个人凭证（AK/SK）决定，服务端不保存任何人的 SK 明文。**

### 5.1 创建「你自己的」个人凭证

在 IAM 控制台「访问凭证」页（`/iam/...` 访问凭证列表）点新建：绑定你自己、归属应用选 `flow`，
创建成功后**一次性显示 SK**（之后只能重置，不再显示）。等价 API：

```powershell
$body = @{ userId = <你的用户id>; remark = 'mcp-agent'; applicationId = 3 } | ConvertTo-Json
Invoke-RestMethod -Uri "http://127.0.0.1:8080/authentication/access-credentials" `
  -Method Post -Body $body -ContentType 'application/json' `
  -Headers @{ Authorization = "Bearer <admin 或你自己的登录 token>" }
```

**接入权限是 `flow.mcp.connect`**（迁移 `iam-service/.../000020_flow_mcp_connect.up.sql`）：
没这个码的账号连 MCP 都进不来（403）。它与工具权限码正交——连得上 ≠ 能看任务，
具体工具仍按 `flow.member.get` / `flow.task.list` 等各自判权。按账号授予（角色 → 权限）。

### 5.2 MCP 客户端配置（HTTP，推荐）

只需要把 AK/SK 填进请求头，不需要换取或管理任何 token：

```powershell
claude mcp add --transport http flow-workbench http://127.0.0.1:8082/flow/mcp `
  --header "x-fa-access-key: <你的 AK>" --header "x-fa-secret-key: <你的 SK>"
```

其他客户端等价写法：

```json
{
  "mcpServers": {
    "flow-workbench": {
      "type": "http",
      "url": "http://127.0.0.1:8082/flow/mcp",
      "headers": {
        "x-fa-access-key": "<你的 AK>",
        "x-fa-secret-key": "<你的 SK>"
      }
    }
  }
}
```

服务端处理：`/flow/mcp` 拿 AK/SK 调 IAM `AuthenticationService/CredentialAuth` 识别出是哪个账号
（校验凭证启用状态、过期时间、SK 常数时间比对），再用该账号身份判 `flow.mcp.connect`，
随后以同一身份调用 flow 网关取数据。换来的短期 token 只在进程内缓存 5 分钟、不落库、不返回给调用方。
换人接入只需要换他自己的 AK/SK，flow-service 不用改配置、不用重启。

### 5.3 stdio 模式（本地临时验证，可选）

```json
{
  "mcpServers": {
    "flow-workbench": {
      "command": "go",
      "args": ["run", ".", "mcp"],
      "cwd": "D:\\fastarch\\flow-service",
      "env": { "FLOW_TOKEN_FILE": "C:\\Users\\<你>\\.fastarch\\flow.token" }
    }
  }
}
```

stdio 模式只认调用者环境里的 token（`FLOW_TOKEN` / token 文件），服务端配置不再接受任何人的 token。
**只有一个 MCP 服务、15 个工具**（不是多个 MCP）：

| 工具 | 作用 | 对应接口 |
| --- | --- | --- |
| `whoami` | 我是谁 / 当前角色 / 可担任角色 / 可开工与被阻塞数 | `GET /flow/me` |
| `switch_role` | 切换当前工作角色 | `POST /flow/me/role` |
| `check_ready` | 传 task_id 查单条；不传列当前角色 ready 任务 | `GET /flow/tasks/{id}` |
| `my_questions` | 待我回答 / 我提出的问题（带预填选项） | `GET /flow/questions` |
| `answer_question` | 闭环问题（选项 / 结论 / 证据） | `POST /flow/questions/{id}/answer` |
| `reopen_question` | 答错打回：仅原提问人或 boss；任务自动重新 blocked | `POST /flow/questions/{id}/reopen` |
| `raise_question` | 抛阻塞问题给上游角色 | `POST /flow/questions` |
| `history` | 留痕查询 | `GET /flow/events` |

MCP 只是 HTTP 客户端：身份与判权仍由 JWT + IAM `CheckPermission` 在服务端裁决，不存在第二套权限逻辑。

接入层的失败码：`-32001` 缺 AK/SK（401）、`-32002` 凭证无效/停用/过期/SK 不匹配（401）、
`-32003` 无 `flow.mcp.connect` 权限（403）、`-32004` IAM 判权或凭证认证不可用（503，fail-closed）、
`-32005` 凭证认证未返回 token（500）、`-32006` 兜底 Bearer token 无效（401）。

### 5.3 建议的验证脚本（对着 agent 说即可）

1. `whoami` → 应显示 admin、当前角色 dev、8 个角色、可开工 1 / 被阻塞 1（任务 #2 ready、#1 blocked）。
2. `check_ready {task_id:"1"}` → `state=blocked`，blockers 里两条问题（问 pm、问 arch）。
3. `switch_role {role_code:"pm"}` → `my_questions` 看到“首屏展示范围”，选 `A` 闭环。
4. `switch_role {role_code:"arch"}` → 答掉第二条。
5. `switch_role {role_code:"dev"}` → `check_ready {task_id:"1"}` 变 `ready`，openQuestions=0。
6. 反向验证：`switch_role {"qa"}` → 任务 #3 的 evidence 问题；不填 `evidence` 直接 `answer_question` 会拿到 **业务码 3001001**，问题保持 open。

## 6. 归责与留痕口径

- 每次建任务/抛问题/闭环/切角色/作废都写 `t_event`，`history` 或页面「留痕」抽屉可查。
- `answered_by != asked_by` 才是正常闭环；同一人自答写 `self_answer=1`，单独动作 `question.self_answer`。
- evidence 类问题缺证据：返回 `3001001`，问题**不动**，任务继续 blocked。
- “闭环了但实际没人处理”：`answered_by` 是账号、`evidence` 是凭据、`t_event` 是时间线，三者对不上即可追责；`reopen_count` 记录被推翻次数。

## 7. 常用错误码

| 码 | 含义 |
| --- | --- |
| 3001001 | 证据必填（evidence 类问题） |
| 3001002 | 任务仍被未闭环问题阻塞，不能完成 |
| 3001003 | 任务/问题不属于当前角色，先切角色 |
| 3001004 | 问题已闭环，不能重复回答 |
| 3001005 | 没给结论（既没选选项也没写说明） |
| 3001009 | 任务已结束，不能再抛问题 |
| 3001012 | 不能切到未持有的角色 |
| 3001015 | 不能向自己当前角色提问 |
| 3001023 | 证据形态不合法（URL / commit SHA / 工单号 / 部署单 / *.md/*.pdf 之外） |
| 3001024 | 只有已闭环的问题才能重开 |
| 3001025 | 不是原提问人，且当前角色不是 boss，不能重开 |
| 3001026 | 任务已结束（done/discarded），不能重开其下问题 |
| 3001027 | 全部任务视图仅老板可查看 |
| 3001028 | 任务视图 scope 仅支持 mine / created / all |

## 8. 已知边界（MVP 取舍）

- 任务不按人分配，只按角色；同一角色多人时是"抢队列"，不是派工。
- 菜单/字典页是 contrib 通用能力，flow 控制台下同样可用（`/flow/menu_management`、`/flow/dict_management`）。
- evidence 只校验"形态"，不校验链接可达性——可追溯性靠人审 + 留痕；不填或填不了可核查的形态仍会被 3001001/3001023 挡住。
- 任务 done/discarded 后不能再重开其下问题；需要继续跟进请新建任务（避免终态被悄悄翻回）。
- 自动巡检（SLA 催办/升级）不在 v1.1，已拆为 v1.2 独立任务。

## 9. 附：本地小工具（`flow-service/tools/`）

| 工具 | 用法 | 用途 |
| --- | --- | --- |
| `dbctl` | `go run ./tools/dbctl -create flow_dev`、`-db flow_dev -query "SELECT ..."` | 建库、查种子/状态，不依赖 mysql 客户端 |
| `webserve` | `go run ./tools/webserve`（默认 127.0.0.1:8090，指到 `admin-web/dist`） | 无 node 环境时静态预览前端：`/flow/**` 反代 8082、IAM 前缀反代 8080、`/seed?token=&to=` 注入登录态 |

`webserve` 只是本地预览/截图辅助，正式联调仍用 `pnpm dev`。

## 10. v1.1：闭环可回退 + 证据强校验

### 10.1 新增接口

| 动作 | HTTP | 权限码 | 语义 |
| --- | --- | --- | --- |
| 重开问题 | `POST /flow/questions/{id}/reopen` | `flow.question.answer` | 仅 `answered` 可重开；原提问人或当前角色 boss 可发起 |

`reopen` 只改 `t_question` 一行：`status` 回 `open`、`reopen_count+1`、清空本轮答案；任务因 `EXISTS(open question)` 派生自动重新 `blocked`，不碰 `t_task.status`。子问题被打回时，若父问题是由“子问题闭环回填”自动闭环的，会沿 `parent_question_id` 一并重开，避免任务假 ready。

### 10.2 证据规则（3001023）

`evidence` 类问题闭环时必须命中以下任一形态（服务层校验，不依赖前端）：

- URL：`http(s)://` 且 host 非空
- commit：7–40 位 hex，可带 `sha:` / `commit:` 前缀；纯数字不算
- 工单：`FAST-123`、`PR-88`、`#123`、`!42`
- 部署单：`deploy:xxx`、`dep:xxx`、`DEP-xxx`
- 文档：`*.md` / `*.pdf`（允许 `docs/` 前缀）

不校验链接可达性（内网/离线链接会把正常闭环卡死）；只挡“done / 已修好了 / ok”这类无法回溯的空头承诺。

### 10.3 验证脚本（可复跑）

```powershell
# 起服务后，用个人 AK/SK 走 MCP HTTP（勿把 SK 写进仓库）
$h = @{ 'x-fa-access-key' = $env:FLOW_AK; 'x-fa-secret-key' = $env:FLOW_SK }

# 重开（用原提问人账号或 boss 角色）：把 <qid> 换成自己的已闭环问题
$body = '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"reopen_question","arguments":{"question_id":"<qid>","reason":"结论缺少证据，打回补"}}}'
Invoke-RestMethod -Uri 'http://127.0.0.1:8082/flow/mcp' -Method Post -ContentType 'application/json' -Headers $h -Body $body
```

验收要点：重开后 `check_ready` 对应任务从 `ready` 回到 `blocked`；`my_questions` 能看到该问题回到 open；`history` 有 `question.reopen`。

## 11. v1.1.1：角色归因与任务视图（老板反馈修复）

### 11.1 问题：一人多角色时“谁派的/谁答的”丢失

- 任务表只记 `created_by`（账号），页面只能按“当前角色队列”查，老板把任务派给 pm 后，老板/分析师看不到已派出的任务。
- 问题表只记 `answered_by`（账号），一个人切多个角色作答时，无法还原“用哪个角色答的”。

### 11.2 数据与契约

| 表 / 消息 | 新增 | 写入时机 |
| --- | --- | --- |
| `t_task` / `Task` | `created_role_id` / `created_role_name` | `CreateTask` 时取**创建动作发生时的当前角色** |
| `t_question` / `Question` | `answered_role_id` / `answered_role_name` | `AnswerQuestion` 时取**当前回答角色**；`reopen` 清 0 |
| `ListMyTasksRequest` | `scope` | `mine`（默认）/ `created`（我发起的）/ `all`（全部，仅 boss） |

迁移 `000002_role_attribution` 只加两列，旧数据保持 0，前端显示“—”；不改变 ready/blocked 派生。

### 11.3 视图语义

- **我的队列（mine）**：当前角色名下的任务，可完成/作废/去闭环。
- **我发起的（created）**：`created_by = 当前账号` 的任务，跨角色可见，只读；解决“老板建完就看不见”。老板登录时默认落在这个视图，不再先看到空的“我的队列”；老板空队列会给出切换提示。
- **全部（all）**：仅老板可见，按 `role_name` 分组，只读；其他角色调用返回 3001027。
- 任务表新增“链路”列：`创建角色 → 当前角色`，一眼看出这一跳是谁派的、现在在谁手里。
- 问题卡片与留痕抽屉显示“回答角色：老板（boss）”，同账号多角色不再靠猜。

### 11.4 MCP 与页面

- MCP 增 `list_tasks(scope/state/page_size)`，工具数 16；agent 也能看“我发起的”和全部。
- 任务工作台顶部 Segmented 切换视图；全部按角色分组展示；非 mine 视图隐藏完成/作废/去闭环，避免越权操作。
- 问题中心已闭环卡片 + 留痕事件都显示回答角色；旧事件没有 `answer_role_id` 时显示“—”。

### 11.5 验收

- `go build ./...` / `go vet ./...` / `pnpm build` 通过。
- boss 的“我发起的”能看到 #12、#18、#19、#20 等派发记录；analyst 队列能看到 #18。
- q7/q8/q10 显示“老板（boss）”，q9 显示“测试（qa）”。

## 12. v1.2：任务树（任务归人，父子派生）

### 12.1 对 v1.1.1 的修正

v1.1.1 的任务仍只挂 `role_id`，`CreateTask` 允许用 `role_code` 把任务改派给另一个角色，导致老板派完任务后自己队列为空。v1.2 改为任务树：

- 任务归**创建人 + 创建时角色**，`t_task.owner_id` + `role_id`。
- 下游角色不接收父任务，只能只读父任务，基于它新建**自己的子任务**，`parent_task_id` 指向父任务。
- 不再提供改派：`role_code` 与当前角色不一致时返回 3001030。

### 12.2 数据与契约

| 位置 | 变化 |
| --- | --- |
| `t_task` | 新增 `owner_id`、`parent_task_id`，均带索引，默认 0 |
| `Task` | 新增 `parent_task_id/parent_task_title/owner_id/owner_name/child_total/child_done/child_open` |
| `CreateTaskRequest` | 新增 `parent_task_id`；`role_code` 标记废弃 |
| `ListMyTasksRequest.scope` | 新增 `upstream`（上游角色任务，只读） |

### 12.3 队列语义

- **我的任务（mine）**：`owner_id=当前账号 AND role_id=当前角色`；老板看到自己建的父任务。
- **上游任务（upstream）**：`role.sort < 当前角色.sort` 的任务，只读；用于基于父任务建子任务。
- **我发起的（created）**：`created_by=当前账号`，跨角色。
- **全部（all）**：boss 只读，按角色分组。

### 12.4 完成门槛

`CloseTask` 在完成前统计 `parent_task_id=当前任务 AND status=open` 的直接子任务；大于 0 返回 3001029。已完成/已作废的下级不阻塞。每一级都受同一规则约束，所以整棵树逐级向上完成。

`CreateTask` 在基于父任务建子任务前，先检查父任务的 open 问题数；大于 0 返回 3001033，禁止在问题没闭环时往下派生。页面在父任务 blocked 时隐藏“基于此建我的任务”，改为“父任务有问题待闭环”。

### 12.5 页面

- 任务表“归属 / 上游”列显示：负责人 + 角色、父任务标题、下级 `完成/总数`。
- “我的任务”只显示自己的任务；有未完成下级时完成按钮换成“等 N 个下级完成”。
- “上游任务”与“全部”只读，提供“基于此建我的任务”，打开新建弹窗并带上父任务。
- 新建弹窗明确：任务归你和当前角色，不改派；父任务要等这条及所有下级完成。
- 顶部保留待处理 / 被阻塞 / 待我闭环；平均闭环、P50 闭环、重开率、闭环后停滞已下线（含 `/flow/metrics` 与 MCP `metrics`）。
- 任务状态按角色语义显示：规划类角色（boss/analyst/pm/arch）刚建、还没有下级时显示“待拆解”；已有下级显示“进行中”；开发/测试/运维无下级时显示“可开工”。筛选与顶部统计统一叫“待处理”。

## 13. v1.3：角色收敛与状态语义

- 删除项目经理 `pjm`：`000004_remove_pjm` 停用角色、清理成员担任关系、当前角色回退 boss；角色剩 7 个。
- 删除平均闭环 / P50 闭环 / 重开率 / 闭环后停滞，连同 `/flow/metrics` 与 MCP `metrics`。
- “全链路”改“全部”；任务表去掉“归属 / 上游”，只保留“处理角色”。
- 详情抽屉与列表统一状态语义，不再出现“建完就显示可开工”的规划类任务。

## 14. v1.4：Markdown 物料与开工门禁

### 14.1 设计口径

- **规划类** `analyst/pm/arch`：待拆解 → 进行中 → **已提交** → 已完成。提交物料后停在“已提交”，不自动验收自己的产物。
- **实施类** `dev/qa/ops`：可开工 → **进行中**（调 `start`）→ 已完成（调 `finish`）。开工前必须拿到上游规划物料，服务端生成工作包并记录上下文 hash，防止 AI 跑偏。
- **boss**：只负责目标与兜底确认，不参与实施。
- 物料先用 Markdown 文本存储，不接文件存储；同类型重复提交时，旧版本置 `superseded`。

### 14.2 数据与契约

| 位置 | 变化 |
| --- | --- |
| `t_task` | `required_artifact_types`、`started_at`、`submitted_at`、`context_hash`、`context_json` |
| `t_artifact` | `task_id/type/title/content_md/version/status/hash`，以及提交人、提交角色、确认人、确认时间 |
| `Task` | 新增 `required_artifact_types/started_at/submitted_at/context_hash/artifact_count` |
| `CreateTaskRequest` | 新增 `required_artifact_types`；不传则按当前角色默认 |
| 新 RPC | `start / submit / finish / accept`、`work-package / artifacts` |
| 新 MCP | `start_task / submit_task / finish_task / accept_task / work_package / list_artifacts` |

默认开工物料：dev=`requirement_spec,feature_design,tech_plan,acceptance_criteria`；qa=`requirement_spec,feature_design,acceptance_criteria,code_change`；ops=`acceptance_criteria,test_report,release_plan`。默认产出：dev=`code_change`、qa=`test_report`、ops=`deployment_record`。

### 14.3 门禁与状态

- `StartTask`：仅实施类、本人当前角色、任务 open、无 open 问题；缺料返回 `3001034`。成功后写 `started_at` 和工作包快照；祖先中已提交的规划任务只有在**没有未完成直接子任务**时才会被顺带确认，否则继续停在“已提交”。
- `SubmitTask`：仅规划类、本人当前角色、无 open 问题；写入 Markdown 物料，任务停在“已提交”（`status` 仍 open）。
- `AcceptTask`：任务必须已提交；当前角色不能是任务角色；老板或 sort 更大的下游可确认；直接 done，物料置 accepted。**有未完成直接子任务时返回 `3001029`**，与父任务完成门槛一致。
- `FinishTask`：仅实施类、必须已开工、无 open 问题、直接子任务全部完成；写入实施产物并直接 done。
- `CloseTask`：拒绝规划类与实施类 owner 自行关闭，只允许老板等兜底角色关闭目标；父任务仍需直接子任务全部完成。
- `TaskWorkbenchPage`：新增“进行中 / 已提交”筛选与状态；规划类显示“提交物料”，实施类显示“开始任务 / 完成任务”；详情抽屉展示开工物料、开工/提交时间、执行快照和 Markdown 物料列表。

### 14.4 错误码

| 码 | 含义 |
| --- | --- |
| 3001034 | 缺少开工物料 |
| 3001035 | 当前任务状态不允许该动作 |
| 3001036 | 未开工不能完成 |
| 3001037 | 物料正文为空 |
| 3001038 | 不能自己确认自己的提交 |

### 14.5 验证

- `go build ./...`、`go vet ./...`、`go test ./...`、`pnpm build` 均通过。
- 迁移 `000005` 已应用到 `flow_dev`，schema version=5。
- 事务内集成验证通过：无上游物料时 `StartTask` 返回 `3001034`；补齐 4 份物料后开工成功、工作包含 4 份物料和 context hash；父任务有未完成子任务时 `AcceptTask` 返回 `3001029`，子任务完成后确认父任务才 done；`FinishTask` 写入 `code_change` 后任务 done。
- HTTP 实时 MCP 工具因本机个人 token 过期未复测；服务端工具已注册，使用 AK/SK 重新接入后即可用。


