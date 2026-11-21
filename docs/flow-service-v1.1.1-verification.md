# flow-service v1.1.1 验收记录（QA）

> 验收对象：`50d51ab418e81cfe4de705d182b9337e0742723c`（角色归因与视图）+ `94cabc261d55099b21a4609bd7d64db95b034ee2`（老板默认视图修正）
> 验收时间：2026-11-21 18:40 CST ｜ 验收角色：测试（qa）
> 结论：**PASS**。三项原始投诉均已闭环：老板/需求分析师能看到任务、回答能看出角色、任务能看出“谁派给谁”。

## 1. 验收范围

| 项 | 验收点 | 结果 |
| --- | --- | --- |
| 迁移 | `schema_migrations` 版本 2；`t_question.answered_role_id`、`t_task.created_role_id` 存在，默认 0 | PASS |
| 契约 | `Question.answered_role_id/name`、`Task.created_role_id/name`、`ListMyTasksRequest.scope` 已由 buf 生成 | PASS |
| 视图 | mine / created / all 三种 scope 生效，all 仅 boss/pjm | PASS |
| 链路 | 任务返回 `createdRoleName → roleName`，历史任务已回填 | PASS |
| 回答角色 | 已答问题返回 `answeredRoleId/answeredRoleName` | PASS |
| 老板体验 | boss 进入任务页默认「我发起的」，空队列有解释提示 | PASS |
| 质量 | `go build/vet/test`、`pnpm build` 全绿 | PASS |

## 2. 构建与测试

```text
go build ./...   exit 0
go vet ./...     exit 0
go test ./...    exit 0  （pkg/service ok，0.494s）
pnpm build       exit 0  （tsc -b + vite build，25.35s；修正后复跑 18.71s）
```

## 3. 迁移与数据回填

```text
schema_migrations: version=2, dirty=0
t_question.answered_role_id  column_default=0
t_task.created_role_id       column_default=0

t_task      total=10, created_role_id>0 = 10
t_question  total=5,  answered_role_id>0 = 4   （第 5 条是本次待答 q11，open 状态为 0 属正常）
```

历史任务链路（库内事实）：

```text
#12 老板 → 产品经理      #18 老板 → 需求分析师
#13 产品经理 → 架构师    #19 需求分析师 → 产品经理
#14 架构师 → 开发        #20 产品经理 → 开发
#15 测试 → 测试          #21 开发 → 测试
#16 运维 → 运维          #22 测试 → 开发
```

## 4. 视图与权限矩阵（实测）

| 角色 | scope | 结果 |
| --- | --- | --- |
| boss | created | `total=11`，含 #22/#21/#20/#19/#18/#17/#16/#15/#14/#13/#12 |
| boss | all | `total=11`，覆盖测试/产品经理/架构师/开发/需求分析师/运维 6 个角色 |
| qa | created | `total=10`（同一账号跨角色发起，按账号归集） |
| qa | all | `业务码 3001027：全链路视图仅老板/项目经理可看` |
| qa | mine + state=ready | `total=1`，仅 #21 |
| analyst | mine | 仅 #18（角色归因任务） |

说明：`created` 按 `created_by` 账号归集，而不是按“当前角色”归集，所以单人多角色环境会看到自己所有角色发起的任务；多人环境下即“我发起的”。

## 5. 回答角色

`my_questions(scope=raised, status=answered)` 实测：

```text
q7  task12  产品经理提问  answeredRoleName=老板(1)
q8  task13  架构师提问    answeredRoleName=老板(1)
q9  task14  开发提问      answeredRoleName=测试(6)
q10 task18  需求分析师提问 answeredRoleName=老板(1)
```

问题卡片显示「回答角色：老板（boss）」；留痕 detail 同步写入 `answer_role_name` / `answer_role_code`，重开时清零，避免把旧回答角色带到新回答上。

## 6. QA 发现与修正

**问题**：仅加 scope 页签不够。boss 登录仍默认落在「我的队列」，而老板没有执行任务，队列恒为 0，用户仍会认为“看不到任务”。

**修正**（`94cabc2`，仅前端）：

1. boss 登录默认 `scope=created`，直接看到自己派下去的活；其他角色仍默认 `mine`。
2. boss/pjm 在 mine 为空时显示说明：老板通常不接执行任务，派下去的活看「我发起的」，全局看「全链路」。
3. 切换角色后重置 scope override，按新角色重新决定默认视图。

## 7. 残余风险

- 前端验收到“代码 + 构建”级，尚未做浏览器人工点选回归；默认视图与空队列提示需在真实浏览器复核一次。
- `created` 按账号归集：单人演示环境会把所有角色的发起混在一起；多人协作时语义正确。
- 旧数据 `answered_role_id=0` 前端显示“—”，不做猜测回填；本次仅回填了可确认的历史任务/问题。
- evidence 仍只校验形态，不校验链接可达性。
- v1.2 sweep 任务 #17 仍挂在 pm 队列，未纳入本次验收。

## 8. 复跑命令

```powershell
cd D:\fastarch\flow-service
go build ./...; go vet ./...; go test ./... -count=1

cd D:\fastarch\frontend-service\admin-web
pnpm build
```

MCP 复跑（当前角色）：

```text
list_tasks(scope=mine)     当前角色队列
list_tasks(scope=created)  我发起的（跨角色）
list_tasks(scope=all)      全链路，仅 boss/pjm
my_questions(scope=raised, status=answered)  看自己提过的问题及回答角色
```
