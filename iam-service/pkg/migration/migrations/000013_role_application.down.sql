-- 000013 回滚：移除角色所属应用列（索引随列删除）。

ALTER TABLE `t_role` DROP COLUMN `application_id`;
