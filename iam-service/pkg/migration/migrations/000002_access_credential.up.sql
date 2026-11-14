-- 000002 AK/SK 访问凭证表（specs/009，保留功能；归属列 account_id → user_id 对齐 t_user）。
CREATE TABLE IF NOT EXISTS `t_access_credential` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `access_key` varchar(64) DEFAULT NULL,
  `secret_key_cipher` varchar(256) DEFAULT NULL,
  `user_id` bigint DEFAULT NULL,
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
  KEY `idx_t_access_credential_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
