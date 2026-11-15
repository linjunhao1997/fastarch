DELETE rp FROM t_r_role_permission rp
JOIN t_permission p ON p.id = rp.permission_id AND p.code = 'iam.user.reset_password';
DELETE FROM t_permission WHERE code = 'iam.user.reset_password';
