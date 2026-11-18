-- 注意：若已存在跨应用同名角色码，恢复全局唯一索引会失败（需先人工去重）。
ALTER TABLE `t_role` DROP KEY `uk_t_role_app_code`;
ALTER TABLE `t_role` ADD UNIQUE KEY `idx_t_role_code` (`code`);
ALTER TABLE `t_role` DROP COLUMN `is_super_admin`;
