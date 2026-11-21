-- 000002 回滚：移除角色归因列（不触碰业务数据本身）。
ALTER TABLE `t_question` DROP COLUMN `answered_role_id`;
ALTER TABLE `t_task` DROP COLUMN `created_role_id`;
