DROP INDEX idx_t_task_parent ON t_task;
DROP INDEX idx_t_task_owner ON t_task;

ALTER TABLE t_task
  DROP COLUMN parent_task_id,
  DROP COLUMN owner_id;
