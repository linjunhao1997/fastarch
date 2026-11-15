-- 000006 对齐菜单 path/icon 与前端路由表（iam-web router.tsx）：
-- t_menu 是导航唯一事实源，path 必须命中前端路由；icon 为 AntD 图标名。
-- 同时隐藏"权限订阅"（#5 裁决：Subscription 无表未实现，菜单节点删除，
-- 待功能落地由新迁移补回）。幂等：UPDATE/DELETE 天然可重复执行。

UPDATE t_menu SET path = '', icon = 'SafetyOutlined', update_time = NOW(3)
WHERE id = 1001;
UPDATE t_menu SET path = '/account_management/account_list', icon = 'UserSwitchOutlined', update_time = NOW(3)
WHERE id = 1002;
UPDATE t_menu SET path = '/application_management/application_list', icon = 'AppstoreOutlined', update_time = NOW(3)
WHERE id = 1003;
UPDATE t_menu SET path = '', icon = 'SecurityScanOutlined', update_time = NOW(3)
WHERE id = 1004;
UPDATE t_menu SET path = '', icon = 'KeyOutlined', update_time = NOW(3)
WHERE id = 1005;
-- 菜单管理/字典管理：前端页面在重构 #6 第二批落地，path 先行占位（未命中路由时前端自动隐藏）
UPDATE t_menu SET path = '/authorization_management/menu_management', icon = 'MenuOutlined', update_time = NOW(3)
WHERE id = 1006;
UPDATE t_menu SET path = '/authorization_management/dict_management', icon = 'BookOutlined', update_time = NOW(3)
WHERE id = 1007;
UPDATE t_menu SET path = '/authentication_management/access_credential_list', icon = 'LockOutlined', update_time = NOW(3)
WHERE id = 1008;
UPDATE t_menu SET path = '/authorization_management/permission_list', icon = 'UnorderedListOutlined', update_time = NOW(3)
WHERE id = 1009;
UPDATE t_menu SET path = '/authorization_management/role_management', icon = 'TeamOutlined', update_time = NOW(3)
WHERE id = 1010;

-- 权限订阅未实现：删除菜单节点与角色绑定
DELETE FROM t_r_role_menu WHERE menu_id = 1011;
DELETE FROM t_menu WHERE id = 1011;
