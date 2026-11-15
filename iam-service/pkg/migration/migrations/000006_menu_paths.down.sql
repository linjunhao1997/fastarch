-- 回滚到 000003 原始种子形态（/iam/* 占位路径与图标、恢复权限订阅节点）。
UPDATE t_menu SET path = '/iam', icon = 'SafetyOutlined', update_time = NOW(3) WHERE id = 1001;
UPDATE t_menu SET path = '/iam/users', icon = 'UserOutlined', update_time = NOW(3) WHERE id = 1002;
UPDATE t_menu SET path = '/iam/applications', icon = 'AppstoreOutlined', update_time = NOW(3) WHERE id = 1003;
UPDATE t_menu SET path = '/iam/authentication', icon = 'LockOutlined', update_time = NOW(3) WHERE id = 1004;
UPDATE t_menu SET path = '/iam/authorization', icon = 'ApiOutlined', update_time = NOW(3) WHERE id = 1005;
UPDATE t_menu SET path = '/iam/menus', icon = 'MenuOutlined', update_time = NOW(3) WHERE id = 1006;
UPDATE t_menu SET path = '/iam/dicts', icon = 'BookOutlined', update_time = NOW(3) WHERE id = 1007;
UPDATE t_menu SET path = '/iam/credentials', icon = 'KeyOutlined', update_time = NOW(3) WHERE id = 1008;
UPDATE t_menu SET path = '/iam/authorization/permissions', icon = 'UnorderedListOutlined', update_time = NOW(3) WHERE id = 1009;
UPDATE t_menu SET path = '/iam/authorization/roles', icon = 'TeamOutlined', update_time = NOW(3) WHERE id = 1010;

INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1011, 1005, '权限订阅', '/iam/authorization/subscriptions', 'SubscriptionOutlined', 3, 'iam.subscription.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1011);

INSERT INTO t_r_role_menu (role_id, menu_id)
SELECT r.id, 1011 FROM t_role r
WHERE r.code = 'admin'
  AND NOT EXISTS (SELECT 1 FROM t_r_role_menu rm WHERE rm.role_id = r.id AND rm.menu_id = 1011);
