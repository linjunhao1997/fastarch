-- 回滚：去掉开工物料与执行快照。
DROP TABLE IF EXISTS `t_artifact`;

ALTER TABLE t_task
  DROP COLUMN context_json,
  DROP COLUMN context_hash,
  DROP COLUMN submitted_at,
  DROP COLUMN started_at,
  DROP COLUMN required_artifact_types;
