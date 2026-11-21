-- 去掉项目经理（pjm）：保留历史行的可追溯性，但不再作为可担任角色。
UPDATE t_work_role
SET status = 'disabled', update_time = NOW(3)
WHERE code = 'pjm';

-- 清掉成员与该角色的担任关系，避免仍能切到 pjm。
DELETE FROM t_r_member_role
WHERE role_id IN (SELECT id FROM t_work_role WHERE code = 'pjm');

-- 当前角色是 pjm 的成员回落到老板，避免登录后没有当前角色。
UPDATE t_member
SET current_role_id = COALESCE(
  (SELECT id FROM t_work_role WHERE code = 'boss' AND status = 'enabled' ORDER BY id LIMIT 1),
  current_role_id
),
update_time = NOW(3)
WHERE current_role_id IN (SELECT id FROM t_work_role WHERE code = 'pjm');
