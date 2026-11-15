-- 000005 补种 iam.user.reset_password（#5 新增 ResetAccountPassword RPC 的权限码），
-- 并绑定 admin 角色。幂等：WHERE NOT EXISTS。
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.reset_password', '重置用户密码', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.reset_password');

INSERT INTO t_r_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM t_role r JOIN t_permission p ON p.code = 'iam.user.reset_password'
WHERE r.code = 'admin'
  AND NOT EXISTS (SELECT 1 FROM t_r_role_permission rp WHERE rp.role_id = r.id AND rp.permission_id = p.id);
