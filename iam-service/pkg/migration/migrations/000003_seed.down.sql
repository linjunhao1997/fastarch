-- 回滚种子：仅清理种子写入的业务行（表结构由 000001/000002 的 down 回滚）。
DELETE FROM t_r_role_menu WHERE role_id = 1;
DELETE FROM t_r_role_permission WHERE role_id = 1;
DELETE FROM t_r_user_role WHERE user_id = 1;
DELETE FROM t_menu WHERE id BETWEEN 1001 AND 1011;
DELETE FROM t_permission WHERE code LIKE 'iam.%';
DELETE FROM t_user WHERE id = 1;
DELETE FROM t_role WHERE id = 1;
