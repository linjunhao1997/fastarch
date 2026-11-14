-- 回滚种子：仅清理种子写入的业务行（表结构由 000001/000002 的 down 回滚）。
DELETE rm FROM t_r_role_menu rm JOIN t_role r ON r.id = rm.role_id WHERE r.code = 'admin';
DELETE rp FROM t_r_role_permission rp JOIN t_role r ON r.id = rp.role_id WHERE r.code = 'admin';
DELETE ur FROM t_r_user_role ur JOIN t_user u ON u.id = ur.user_id WHERE u.username = 'admin';
DELETE FROM t_menu WHERE id BETWEEN 1001 AND 1011;
DELETE FROM t_permission WHERE code LIKE 'iam.%';
DELETE FROM t_user WHERE username = 'admin' AND created_by = 'system';
DELETE FROM t_role WHERE code = 'admin' AND created_by = 'system';
