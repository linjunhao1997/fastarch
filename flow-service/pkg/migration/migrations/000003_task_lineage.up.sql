-- v1.2 任务树：任务归人、归创建角色，父任务只读引用，不再按角色改派。
ALTER TABLE t_task
  ADD COLUMN owner_id BIGINT NOT NULL DEFAULT 0 AFTER role_id,
  ADD COLUMN parent_task_id BIGINT NOT NULL DEFAULT 0 AFTER role_id;

CREATE INDEX idx_t_task_owner ON t_task (owner_id);
CREATE INDEX idx_t_task_parent ON t_task (parent_task_id);

-- 历史数据按“任务当前角色”找持有该角色的成员回填归属人；单账号环境即本人。
UPDATE t_task
SET owner_id = COALESCE(
  (SELECT m.account_id FROM t_member m
   JOIN t_r_member_role mr ON mr.member_id = m.id
   WHERE mr.role_id = t_task.role_id AND m.is_deleted = 0
   ORDER BY m.id LIMIT 1),
  0
);
