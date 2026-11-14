-- 000001 基线 Schema：全部业务表（MySQL 8，utf8mb4）。
-- 由 gorm 模型标签推导；CREATE TABLE IF NOT EXISTS 使本迁移同时兼容
-- 全新库（全量建表）与存量库（AutoMigrate 时代的表已存在则跳过）。
-- 宪法 XIV：不建立物理外键，完整性约束由业务层保证。
-- id 一律 AUTO_INCREMENT：服务层用雪花 ID 显式插入，关联表插入省略 id。

CREATE TABLE IF NOT EXISTS `t_account` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `code` longtext,
  `name` longtext,
  `email` varchar(255) DEFAULT NULL,
  `status` longtext,
  `salt` longtext,
  `password` longtext,
  `ak` longtext,
  `sk` longtext,
  `kind` longtext,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  `created_by` bigint DEFAULT NULL,
  `updated_by` bigint DEFAULT NULL,
  `version` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_t_account_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_application` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `code` longtext,
  `name` longtext,
  `description` longtext,
  `disabled` tinyint(1) DEFAULT NULL,
  `kind` longtext,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_permission` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `name` longtext,
  `kind` longtext,
  `code` varchar(255) DEFAULT NULL,
  `value` longtext,
  `description` longtext,
  `application_id` bigint DEFAULT NULL,
  `disabled` tinyint(1) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  `created_by` bigint DEFAULT NULL,
  `updated_by` bigint DEFAULT NULL,
  `version` bigint DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_t_permission_code` (`code`),
  KEY `idx_t_permission_application_id` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_role` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `name` longtext,
  `code` varchar(255) DEFAULT NULL,
  `description` longtext,
  `application_id` bigint DEFAULT NULL,
  `disabled` tinyint(1) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  `created_by` bigint DEFAULT NULL,
  `updated_by` bigint DEFAULT NULL,
  `version` bigint DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_t_role_code` (`code`),
  KEY `idx_t_role_application_id` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_r_account_role` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `account_id` bigint DEFAULT NULL,
  `role_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_r_account_role_account_role` (`account_id`, `role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_r_account_application` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `account_id` bigint DEFAULT NULL,
  `application_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_r_account_application_account_application` (`account_id`, `application_id`),
  KEY `idx_t_r_account_application_account_id` (`account_id`),
  KEY `idx_t_r_account_application_application_id` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_r_role_permission` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `role_id` bigint DEFAULT NULL,
  `permission_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_r_role_permission_role_permission` (`role_id`, `permission_id`),
  KEY `idx_t_r_role_permission_role_id` (`role_id`),
  KEY `idx_t_r_role_permission_permission_id` (`permission_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_r_permission_resource` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `permission_id` bigint DEFAULT NULL,
  `resource_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_permission_resource` (`permission_id`, `resource_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_resource` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint NOT NULL DEFAULT 0,
  `parent_id` bigint DEFAULT NULL,
  `kind` varchar(32) DEFAULT NULL,
  `key` varchar(255) DEFAULT NULL,
  `name` longtext,
  `rpc_full_method` longtext,
  `disabled` tinyint(1) DEFAULT NULL,
  `display_order` int DEFAULT 0,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  `created_by` bigint DEFAULT NULL,
  `updated_by` bigint DEFAULT NULL,
  `version` bigint DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_resource_application_key` (`application_id`, `key`),
  KEY `idx_t_resource_application_id` (`application_id`),
  KEY `idx_t_resource_parent_id` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_access_credential` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `access_key` varchar(64) DEFAULT NULL,
  `secret_key_cipher` varchar(256) DEFAULT NULL,
  `account_id` bigint DEFAULT NULL,
  `remark` varchar(255) DEFAULT NULL,
  `status` varchar(16) DEFAULT NULL,
  `expire_time` datetime(3) DEFAULT NULL,
  `last_used_time` datetime(3) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  `created_by` bigint DEFAULT NULL,
  `updated_by` bigint DEFAULT NULL,
  `version` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_t_access_credential_access_key` (`access_key`),
  KEY `idx_t_access_credential_account_id` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
