-- 000007 多应用平台化（#7-#9 裁决）：应用维度进入菜单/字典/权限。
-- 原则：数据隔离（按应用切数据）+ 全局唯一码（服务段=应用 code）+ 授权保持全局授予。
-- 幂等：ALTER 用条件化存储过程包装过重，此处依赖"列/索引/表不存在才加"的
-- 执行前置（迁移链一次性前进，dirty 后人工处理，与 000001-000006 同风格）。

-- 1) 应用注册表：code = 权限码服务段命名空间（如 iam/order/crm），唯一
ALTER TABLE `t_application`
  ADD COLUMN `code` varchar(32) DEFAULT NULL AFTER `app_id`,
  ADD UNIQUE KEY `idx_t_application_code` (`code`);

INSERT INTO t_application (id, app_id, code, name, secret_hash, status, version, created_by, updated_by, create_time, update_time)
SELECT 1, '1', 'iam', 'IAM 控制台', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_application WHERE code = 'iam');

-- 2) 三张资源表加应用归属（可空：NULL = 待归类，本迁移末尾统一回填）
ALTER TABLE `t_menu`
  ADD COLUMN `application_id` bigint DEFAULT NULL AFTER `parent_id`,
  ADD KEY `idx_t_menu_application_id` (`application_id`);

ALTER TABLE `t_dict_type`
  ADD COLUMN `application_id` bigint DEFAULT NULL AFTER `code`,
  ADD KEY `idx_t_dict_type_application_id` (`application_id`);

ALTER TABLE `t_permission`
  ADD COLUMN `application_id` bigint DEFAULT NULL AFTER `code`,
  ADD KEY `idx_t_permission_application_id` (`application_id`);

-- 3) 清理 order:* 样例权限（其服务段应用未注册，违反服务段注册语义；
--    真实 order 应用接入时经契约 API 重新登记）
DELETE rp FROM t_r_role_permission rp
JOIN t_permission p ON p.id = rp.permission_id
WHERE p.code LIKE 'order:%';
DELETE FROM t_permission WHERE code LIKE 'order:%';

-- 4) 回填：全部既有菜单与 iam.* 权限码归入 iam 控制台应用
UPDATE t_menu SET application_id = (SELECT id FROM t_application WHERE code = 'iam')
WHERE application_id IS NULL;

UPDATE t_permission SET application_id = (SELECT id FROM t_application WHERE code = 'iam')
WHERE code LIKE 'iam.%' AND application_id IS NULL;

-- 5) 用户↔应用准入（数据隔离下的"可进入哪些应用的管理台"；admin 角色旁路全量）
CREATE TABLE IF NOT EXISTS `t_r_user_application` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint DEFAULT NULL,
  `application_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_t_r_user_application_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
