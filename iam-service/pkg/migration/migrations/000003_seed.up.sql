-- 000003 种子数据：admin 角色 + admin 用户（bcrypt，初始密码 admin123，首次登录后应修改）
-- + 点式权限码词汇（服务.模块.操作）+ 菜单树种子 + admin 全量绑定。
-- 幂等：INSERT ... WHERE NOT EXISTS，重复执行不产生重复行。

-- ── 角色（按 code 幂等，不写死 id：存量库可能有任意自增数据）──────────────
INSERT INTO t_role (code, name, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'admin', '系统管理员', '内置超级管理员角色', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_role WHERE code = 'admin');

-- ── 用户（按 username 幂等；种子保证 admin 具有已知初始密码 admin123）──────
INSERT INTO t_user (username, password_hash, nickname, status, version, created_by, updated_by, create_time, update_time)
SELECT 'admin', '$2a$10$Fo73nnjx6c606EGLmXq0uuiZJRnbXOUqiWI9.Mn.vT4coClo79lcq', '系统管理员', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_user WHERE username = 'admin');

UPDATE t_user SET password_hash = '$2a$10$Fo73nnjx6c606EGLmXq0uuiZJRnbXOUqiWI9.Mn.vT4coClo79lcq', update_time = NOW(3)
WHERE username = 'admin' AND password_hash <> '$2a$10$Fo73nnjx6c606EGLmXq0uuiZJRnbXOUqiWI9.Mn.vT4coClo79lcq';

INSERT INTO t_r_user_role (user_id, role_id)
SELECT u.id, r.id FROM t_user u JOIN t_role r ON r.code = 'admin'
WHERE u.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM t_r_user_role x WHERE x.user_id = u.id AND x.role_id = r.id);

-- ── 权限码词汇（与 pkg/model/permission_codes.go 镜像）───────────────────
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.create', '用户创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.get', '用户查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.list', '用户列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.update', '用户更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.delete', '用户删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.delete');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.user.assign_roles', '用户分配角色', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.user.assign_roles');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.application.create', '应用创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.application.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.application.get', '应用查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.application.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.application.list', '应用列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.application.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.application.update', '应用更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.application.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.application.delete', '应用删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.application.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.permission.create', '权限创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.permission.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.permission.get', '权限查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.permission.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.permission.list', '权限列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.permission.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.permission.update', '权限更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.permission.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.permission.delete', '权限删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.permission.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.role.create', '角色创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.role.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.role.get', '角色查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.role.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.role.list', '角色列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.role.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.role.update', '角色更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.role.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.role.delete', '角色删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.role.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.subscription.create', '权限订阅创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.subscription.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.subscription.get', '权限订阅查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.subscription.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.subscription.update', '权限订阅更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.subscription.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.subscription.list', '权限订阅列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.subscription.list');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.access_credential.create', '访问凭证创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.access_credential.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.access_credential.get', '访问凭证查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.access_credential.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.access_credential.list', '访问凭证列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.access_credential.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.access_credential.update', '访问凭证更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.access_credential.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.access_credential.status', '访问凭证状态变更', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.access_credential.status');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.access_credential.reset_secret', '访问凭证重置密钥', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.access_credential.reset_secret');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.access_credential.delete', '访问凭证删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.access_credential.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.menu.create', '菜单创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.menu.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.menu.get', '菜单查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.menu.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.menu.list', '菜单列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.menu.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.menu.update', '菜单更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.menu.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.menu.delete', '菜单删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.menu.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.dict.create', '字典创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.dict.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.dict.get', '字典查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.dict.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.dict.list', '字典列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.dict.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.dict.update', '字典更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.dict.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time)
SELECT 'iam.dict.delete', '字典删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'iam.dict.delete');

-- ── 菜单树种子（t_menu：结构归代码种子，管理员可后续调整）────────────────
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1001, 0, '身份访问管理', '/iam', 'SafetyOutlined', 0, NULL, 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1001);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1002, 1001, '用户管理', '/iam/users', 'UserOutlined', 1, 'iam.user.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1002);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1003, 1001, '应用管理', '/iam/applications', 'AppstoreOutlined', 2, 'iam.application.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1003);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1004, 1001, '认证管理', '/iam/authentication', 'LockOutlined', 3, NULL, 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1004);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1005, 1001, '权限管理', '/iam/authorization', 'ApiOutlined', 4, NULL, 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1005);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1006, 1001, '菜单管理', '/iam/menus', 'MenuOutlined', 5, 'iam.menu.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1006);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1007, 1001, '字典管理', '/iam/dicts', 'BookOutlined', 6, 'iam.dict.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1007);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1008, 1004, '访问凭证', '/iam/credentials', 'KeyOutlined', 1, 'iam.access_credential.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1008);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1009, 1005, '权限列表', '/iam/authorization/permissions', 'UnorderedListOutlined', 1, 'iam.permission.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1009);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1010, 1005, '角色管理', '/iam/authorization/roles', 'TeamOutlined', 2, 'iam.role.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1010);
INSERT INTO t_menu (id, parent_id, name, path, icon, sort, permission_code, status, version, created_by, updated_by, create_time, update_time)
SELECT 1011, 1005, '权限订阅', '/iam/authorization/subscriptions', 'SubscriptionOutlined', 3, 'iam.subscription.list', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_menu WHERE id = 1011);

-- ── admin 角色全量绑定（权限 + 菜单；role_id 按 code 动态查找）────────────
INSERT INTO t_r_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM t_role r CROSS JOIN t_permission p
WHERE r.code = 'admin'
  AND NOT EXISTS (SELECT 1 FROM t_r_role_permission rp WHERE rp.role_id = r.id AND rp.permission_id = p.id);

INSERT INTO t_r_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM t_role r CROSS JOIN t_menu m
WHERE r.code = 'admin'
  AND NOT EXISTS (SELECT 1 FROM t_r_role_menu rm WHERE rm.role_id = r.id AND rm.menu_id = m.id);
