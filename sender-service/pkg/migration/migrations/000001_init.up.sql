-- 000001 sender-service 基线 Schema：每服务自持的菜单/字典四表 + 消息模板 + 发送记录。
-- 数据落在 sender 自己的库（ADR 0001/0003）；application_id 固定为 IAM 注册的 sender app（id=2）。
-- 宪法 XIV：无物理外键，完整性由业务层保证；审计字段服务端注入；业务型模型软删除。

CREATE TABLE IF NOT EXISTS `t_menu` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `parent_id` bigint DEFAULT '0',
  `application_id` bigint DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `path` varchar(128) DEFAULT NULL,
  `icon` varchar(64) DEFAULT NULL,
  `sort` int DEFAULT '0',
  `permission_code` varchar(64) DEFAULT NULL,
  `status` varchar(16) DEFAULT 'enabled',
  `version` bigint DEFAULT NULL,
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_t_menu_parent_id` (`parent_id`),
  KEY `idx_t_menu_application_id` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_dict_type` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `code` varchar(64) DEFAULT NULL,
  `application_id` bigint DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `remark` varchar(255) DEFAULT NULL,
  `status` varchar(16) DEFAULT 'enabled',
  `version` bigint DEFAULT NULL,
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dict_app_code` (`application_id`, `code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_dict_item` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `type_id` bigint DEFAULT NULL,
  `label` varchar(64) DEFAULT NULL,
  `value` varchar(64) DEFAULT NULL,
  `sort` int DEFAULT '0',
  `status` varchar(16) DEFAULT 'enabled',
  `version` bigint DEFAULT NULL,
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_t_dict_item_type_id` (`type_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_message_template` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint DEFAULT NULL,
  `code` varchar(64) DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `content` text,
  `remark` varchar(255) DEFAULT NULL,
  `version` bigint DEFAULT NULL,
  `is_deleted` tinyint(1) DEFAULT '0',
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_template_app_code` (`application_id`, `code`),
  KEY `idx_template_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_message_record` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint DEFAULT NULL,
  `recipient` varchar(255) DEFAULT NULL,
  `template_code` varchar(64) DEFAULT NULL,
  `content` text,
  `channel` varchar(32) DEFAULT NULL,
  `send_status` varchar(16) DEFAULT 'pending',
  `version` bigint DEFAULT NULL,
  `is_deleted` tinyint(1) DEFAULT '0',
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_record_application_id` (`application_id`),
  KEY `idx_record_send_status` (`send_status`),
  KEY `idx_record_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 种子：菜单（展示门槛码）、字典（channel/send_status）、默认模板。幂等：WHERE NOT EXISTS。
INSERT INTO t_menu (id, parent_id, application_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000001, 0, 2, '发送记录', '/sender/records', '', 1, 'sender.record.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 2000000000000000001);
INSERT INTO t_menu (id, parent_id, application_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000002, 0, 2, '消息模板', '/sender/templates', '', 2, 'sender.template.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 2000000000000000002);

INSERT INTO t_dict_type (id, code, application_id, name, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000011, 'channel', 2, '发送渠道', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_dict_type WHERE id = 2000000000000000011);
INSERT INTO t_dict_item (id, type_id, label, value, sort, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000101, 2000000000000000011, '邮件', 'email', 1, 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_dict_item WHERE id = 2000000000000000101);
INSERT INTO t_dict_item (id, type_id, label, value, sort, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000102, 2000000000000000011, '站内信', 'inbox', 2, 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_dict_item WHERE id = 2000000000000000102);

INSERT INTO t_dict_type (id, code, application_id, name, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000012, 'send_status', 2, '发送状态', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_dict_type WHERE id = 2000000000000000012);
INSERT INTO t_dict_item (id, type_id, label, value, sort, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000201, 2000000000000000012, '待发送', 'pending', 1, 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_dict_item WHERE id = 2000000000000000201);
INSERT INTO t_dict_item (id, type_id, label, value, sort, status, version, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000202, 2000000000000000012, '已发送', 'sent', 2, 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_dict_item WHERE id = 2000000000000000202);

INSERT INTO t_message_template (id, application_id, code, name, content, remark, version, is_deleted, created_by, updated_by, create_time, update_time)
SELECT 2000000000000000031, 2, 'default', '默认模板', '{content}', '系统默认', 1, 0, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_message_template WHERE id = 2000000000000000031);
