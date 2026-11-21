-- 000021 home-service 第四宿主接入（ADR 0003/0005/0008）：home 应用注册 + home.* 权限码 + dev 角色/接入种子。
-- 菜单/字典数据归 home 自持库（不在 IAM）；IAM 只登记应用、权限码与角色。
-- 幂等：INSERT ... WHERE NOT EXISTS。

-- 1) home 应用（固定 id=4 供 home 库跨库弱引用 application_id；
--    secret_hash 为 dev secret 明文 'home-dev-secret-please-change-in-prod' 的 sha256 hex，
--    生产经应用管理页重置密钥，dev 明文不入仓）。
INSERT INTO t_application (id, app_id, code, name, secret_hash, status, version, created_by, updated_by, create_time, update_time)
SELECT 4, '4', 'home', '企业官网', '2e58339e3b37e803a19b71c6296e20969fad2d4c230ac97e005d2ce667aefbd5', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_application WHERE code = 'home');

-- 2) 首页配置域权限码
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.page.get', '首页配置读取', 'api', '读取首页草稿/已发布/版本', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.page.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.page.update', '首页草稿保存', 'api', '保存首页草稿', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.page.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.page.publish', '首页发布', 'api', '发布当前草稿', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.page.publish');

-- 3) 菜单域权限码（home 自持菜单表，权限码仍登记在 IAM）
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.menu.create', '菜单创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.menu.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.menu.get', '菜单查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.menu.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.menu.list', '菜单列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.menu.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.menu.update', '菜单更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.menu.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'home.menu.delete', '菜单删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'home')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'home.menu.delete');

-- 4) 角色 home_admin：绑定全部 home.* 权限码
INSERT INTO t_role (id, code, name, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 4000000000000000002, 'home_admin', '企业官网管理员', '企业官网控制台管理员（dev 种子）', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), 4
WHERE NOT EXISTS (SELECT 1 FROM t_role WHERE code = 'home_admin');

INSERT INTO t_r_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM t_role r
JOIN t_permission p ON p.code IN ('home.page.get','home.page.update','home.page.publish','home.menu.create','home.menu.get','home.menu.list','home.menu.update','home.menu.delete')
WHERE r.code = 'home_admin'
  AND NOT EXISTS (
    SELECT 1 FROM t_r_role_permission x WHERE x.role_id = r.id AND x.permission_id = p.id
  );

-- 5) dev 种子：当前用户接入 home 应用并授予 home_admin（生产环境改由管理员分配）
INSERT INTO t_r_user_application (user_id, application_id)
SELECT u.id, a.id
FROM t_user u
JOIN t_application a ON a.code = 'home'
WHERE u.username = 'linjunhao'
  AND NOT EXISTS (
    SELECT 1 FROM t_r_user_application x WHERE x.user_id = u.id AND x.application_id = a.id
  );

INSERT INTO t_r_user_role (user_id, role_id)
SELECT u.id, r.id
FROM t_user u
JOIN t_role r ON r.code = 'home_admin'
WHERE u.username = 'linjunhao'
  AND NOT EXISTS (
    SELECT 1 FROM t_r_user_role x WHERE x.user_id = u.id AND x.role_id = r.id
  );
