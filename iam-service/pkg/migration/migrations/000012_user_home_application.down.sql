-- 000012 回滚：移除用户所属应用列（索引随列删除）。

ALTER TABLE `t_user` DROP COLUMN `home_application_id`;
