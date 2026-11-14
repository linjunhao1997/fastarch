-- 000002 存量库清理：删除历史物理外键（宪法 XIV）与历史全局 key 索引
-- （应用级唯一 uk_resource_application_key 迁入前的遗留）。
-- DROP FOREIGN KEY / DROP INDEX 不支持 IF NOT EXISTS，故经存储过程条件执行；
-- 全新库无外键无遗留索引，本迁移零操作。

DROP PROCEDURE IF EXISTS drop_legacy_schema_objects;

CREATE PROCEDURE drop_legacy_schema_objects()
BEGIN
  DECLARE done INT DEFAULT 0;
  DECLARE v_table VARCHAR(64);
  DECLARE v_fk VARCHAR(64);
  DECLARE fk_cur CURSOR FOR
    SELECT TABLE_NAME, CONSTRAINT_NAME
    FROM information_schema.REFERENTIAL_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE();
  DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

  OPEN fk_cur;
  fk_loop: LOOP
    FETCH fk_cur INTO v_table, v_fk;
    IF done = 1 THEN
      LEAVE fk_loop;
    END IF;
    SET @fk_sql = CONCAT('ALTER TABLE `', v_table, '` DROP FOREIGN KEY `', v_fk, '`');
    PREPARE fk_stmt FROM @fk_sql;
    EXECUTE fk_stmt;
    DEALLOCATE PREPARE fk_stmt;
  END LOOP;
  CLOSE fk_cur;

  IF EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 't_resource'
      AND INDEX_NAME = 'idx_resource_key'
  ) THEN
    ALTER TABLE `t_resource` DROP INDEX `idx_resource_key`;
  END IF;

  -- 历史版本缺失的列按需补齐（000001 的 IF NOT EXISTS 不会给已存在的表加列，
  -- 而 000004 数据录入会写入这些列）
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_application' AND COLUMN_NAME = 'description'
  ) THEN
    ALTER TABLE `t_application` ADD COLUMN `description` longtext;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_role' AND COLUMN_NAME = 'version'
  ) THEN
    ALTER TABLE `t_role` ADD COLUMN `version` bigint DEFAULT 1;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_permission' AND COLUMN_NAME = 'kind'
  ) THEN
    ALTER TABLE `t_permission` ADD COLUMN `kind` longtext;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_permission' AND COLUMN_NAME = 'version'
  ) THEN
    ALTER TABLE `t_permission` ADD COLUMN `version` bigint DEFAULT 1;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_resource' AND COLUMN_NAME = 'version'
  ) THEN
    ALTER TABLE `t_resource` ADD COLUMN `version` bigint DEFAULT 1;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_resource' AND COLUMN_NAME = 'display_order'
  ) THEN
    ALTER TABLE `t_resource` ADD COLUMN `display_order` int DEFAULT 0;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_resource' AND COLUMN_NAME = 'created_by'
  ) THEN
    ALTER TABLE `t_resource` ADD COLUMN `created_by` bigint DEFAULT NULL;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_resource' AND COLUMN_NAME = 'updated_by'
  ) THEN
    ALTER TABLE `t_resource` ADD COLUMN `updated_by` bigint DEFAULT NULL;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_role' AND COLUMN_NAME = 'created_by'
  ) THEN
    ALTER TABLE `t_role` ADD COLUMN `created_by` bigint DEFAULT NULL;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_role' AND COLUMN_NAME = 'updated_by'
  ) THEN
    ALTER TABLE `t_role` ADD COLUMN `updated_by` bigint DEFAULT NULL;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_permission' AND COLUMN_NAME = 'created_by'
  ) THEN
    ALTER TABLE `t_permission` ADD COLUMN `created_by` bigint DEFAULT NULL;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_permission' AND COLUMN_NAME = 'updated_by'
  ) THEN
    ALTER TABLE `t_permission` ADD COLUMN `updated_by` bigint DEFAULT NULL;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_resource' AND COLUMN_NAME = 'rpc_full_method'
  ) THEN
    ALTER TABLE `t_resource` ADD COLUMN `rpc_full_method` longtext;
  END IF;

  -- 历史长文本列收窄为 varchar，使唯一索引可用（AutoMigrate 时代的索引在本迁移中
  -- 不再重建，业务唯一性由 000004 尾部补齐的 uk_resource_application_key 保证）
  ALTER TABLE `t_resource` MODIFY COLUMN `key` varchar(255) DEFAULT NULL;
  ALTER TABLE `t_role` MODIFY COLUMN `code` varchar(255) DEFAULT NULL;
  ALTER TABLE `t_permission` MODIFY COLUMN `code` varchar(255) DEFAULT NULL;
END;

CALL drop_legacy_schema_objects();

DROP PROCEDURE IF EXISTS drop_legacy_schema_objects;
