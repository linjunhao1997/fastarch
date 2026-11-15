-- 000009 补种 iam.user.assign_applications（多应用裁决 #8：分配用户可进入应用），
-- 并绑定 admin 角色。幂等：WHERE NOT EXISTS。
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.assign_applications', '分配用户应用准入', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.assign_applications');

INSERT INTO t_r_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM t_role r JOIN t_permission p ON p.code = 'iam.user.assign_applications'
WHERE r.code = 'admin'
  AND NOT EXISTS (SELECT 1 FROM t_r_role_permission rp WHERE rp.role_id = r.id AND rp.permission_id = p.id);
