-- 回滚 home 接入：先解绑用户/角色权限，再删角色、权限、接入关系、应用。
DELETE ur FROM t_r_user_role ur
JOIN t_role r ON r.id = ur.role_id
WHERE r.code = 'home_admin';

DELETE rp FROM t_r_role_permission rp
JOIN t_role r ON r.id = rp.role_id
WHERE r.code = 'home_admin';

DELETE FROM t_role WHERE code = 'home_admin';

DELETE ua FROM t_r_user_application ua
JOIN t_application a ON a.id = ua.application_id
WHERE a.code = 'home';

DELETE FROM t_permission WHERE code LIKE 'home.%';

DELETE FROM t_application WHERE code = 'home';
