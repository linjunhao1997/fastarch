-- v1.4 开工物料：任务增加执行快照，物料先以 Markdown 文本存储。
ALTER TABLE t_task
  ADD COLUMN required_artifact_types VARCHAR(512) NOT NULL DEFAULT '' AFTER brief,
  ADD COLUMN started_at DATETIME(3) NULL AFTER status,
  ADD COLUMN submitted_at DATETIME(3) NULL AFTER started_at,
  ADD COLUMN context_hash VARCHAR(64) NOT NULL DEFAULT '' AFTER submitted_at,
  ADD COLUMN context_json LONGTEXT NULL AFTER context_hash;

CREATE TABLE IF NOT EXISTS `t_artifact` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `application_id` BIGINT NOT NULL,
  `task_id` BIGINT NOT NULL,
  `type` VARCHAR(64) NOT NULL,
  `title` VARCHAR(255) NOT NULL DEFAULT '',
  `content_md` LONGTEXT NOT NULL,
  `version` INT NOT NULL DEFAULT 1,
  `status` VARCHAR(16) NOT NULL DEFAULT 'submitted',
  `hash` VARCHAR(64) NOT NULL DEFAULT '',
  `created_by` BIGINT NOT NULL DEFAULT 0,
  `created_role_id` BIGINT NOT NULL DEFAULT 0,
  `accepted_by` BIGINT NOT NULL DEFAULT 0,
  `accepted_at` DATETIME(3) NULL,
  `is_deleted` TINYINT(1) NOT NULL DEFAULT 0,
  `create_time` DATETIME(3) NULL,
  `update_time` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_artifact_task` (`task_id`),
  KEY `idx_artifact_type` (`task_id`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
