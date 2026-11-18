-- 000014 角色码应用内唯一 + 超管标志（ADR 0006 补充裁决）：
-- 1) 全局唯一 idx_t_role_code → 复合唯一 (application_id, code)：
--    不同应用可各自定义同名角色（如 ops/admin），消除跨租户码互斥。
--    存量受旧全局唯一索引保证无跨应用重复码，复合索引迁移安全。
-- 2) 超管判定从 code='admin' 改为 is_super_admin 标志：各应用可自建 'admin'
--    应用级管理员角色而不获得平台超管；仅内置 admin 角色置 1（服务端控制，
--    CreateRole/UpdateRole 不接受该字段）。
ALTER TABLE `t_role` DROP KEY `idx_t_role_code`;
ALTER TABLE `t_role` ADD UNIQUE KEY `uk_t_role_app_code` (`application_id`, `code`);
ALTER TABLE `t_role` ADD COLUMN `is_super_admin` tinyint(1) NOT NULL DEFAULT 0 AFTER `application_id`;

UPDATE t_role SET is_super_admin = 1
WHERE code = 'admin' AND application_id = (SELECT id FROM t_application WHERE code = 'iam');
