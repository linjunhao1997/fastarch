-- 000002 v1.1.1 角色归因：一人多角色时，能还原“用哪个角色答的/派发的”。
-- 只加列，不改旧数据语义；旧数据保持 0，前端显示“—”。

ALTER TABLE `t_question`
  ADD COLUMN `answered_role_id` bigint NOT NULL DEFAULT 0 COMMENT '闭环该问题的工作角色 id（0=旧数据/未知）' AFTER `answered_by`;

ALTER TABLE `t_task`
  ADD COLUMN `created_role_id` bigint NOT NULL DEFAULT 0 COMMENT '创建该任务时的工作角色 id（0=旧数据/未知）' AFTER `created_by`;
