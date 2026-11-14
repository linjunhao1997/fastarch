-- 000005 权限码模型重构（011 裁决落地）：
--   1) 删除 t_r_permission_resource：权限不再绑定资源，权限码即授权词汇
--   2) t_permission 删除 kind/value：废弃类型化权限与通配码
--   3) 清除 builtin.* 通配权限及其角色绑定
--   4) 播种权限码词汇表（与 pkg/model PERMISSION_CODES 镜像），并全量授予管理员角色
--   5) t_resource 重构为前端组件资源表：删除 rpc_full_method，新增 permission_codes
--      （JSON 数组），清空旧资源并播种组件树（与 pkg/model COMPONENT_SEEDS 镜像）
-- 破坏性重构：旧资源行与资源绑定关系不保留（存量自定义权限码行保留）。

-- ① 删除权限-资源绑定表
DROP TABLE IF EXISTS t_r_permission_resource;

-- ② 清除 builtin.* 通配权限及其角色绑定
DELETE FROM t_r_role_permission
WHERE permission_id IN (
  SELECT id FROM (
    SELECT id FROM t_permission WHERE code IN ('builtin.menu.all', 'builtin.http.all', 'builtin.grpc.all')
  ) AS builtin_rows
);
DELETE FROM t_permission WHERE code IN ('builtin.menu.all', 'builtin.http.all', 'builtin.grpc.all');

-- ③ t_permission 删除 kind/value 列
ALTER TABLE t_permission DROP COLUMN kind;
ALTER TABLE t_permission DROP COLUMN value;

-- ④ 播种权限码词汇表（服务.模块.操作）
INSERT INTO t_permission (name, code, description, application_id, disabled, create_time, update_time, created_by, updated_by, version)
SELECT c.code, c.code, 'API 权限码（词汇表归代码 METHOD_PERMISSIONS）', a.id, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
JOIN (
  SELECT 'iam.account.create' AS code
  UNION ALL SELECT 'iam.account.get'
  UNION ALL SELECT 'iam.account.list'
  UNION ALL SELECT 'iam.account.update'
  UNION ALL SELECT 'iam.account.delete'
  UNION ALL SELECT 'iam.account.assign_roles'
  UNION ALL SELECT 'iam.application.create'
  UNION ALL SELECT 'iam.application.get'
  UNION ALL SELECT 'iam.application.list'
  UNION ALL SELECT 'iam.application.update'
  UNION ALL SELECT 'iam.application.delete'
  UNION ALL SELECT 'iam.permission.create'
  UNION ALL SELECT 'iam.permission.get'
  UNION ALL SELECT 'iam.permission.list'
  UNION ALL SELECT 'iam.permission.update'
  UNION ALL SELECT 'iam.permission.delete'
  UNION ALL SELECT 'iam.role.create'
  UNION ALL SELECT 'iam.role.get'
  UNION ALL SELECT 'iam.role.list'
  UNION ALL SELECT 'iam.role.update'
  UNION ALL SELECT 'iam.role.delete'
  UNION ALL SELECT 'iam.subscription.create'
  UNION ALL SELECT 'iam.subscription.get'
  UNION ALL SELECT 'iam.subscription.update'
  UNION ALL SELECT 'iam.subscription.list'
  UNION ALL SELECT 'iam.access_credential.create'
  UNION ALL SELECT 'iam.access_credential.get'
  UNION ALL SELECT 'iam.access_credential.list'
  UNION ALL SELECT 'iam.access_credential.update'
  UNION ALL SELECT 'iam.access_credential.status'
  UNION ALL SELECT 'iam.access_credential.reset_secret'
  UNION ALL SELECT 'iam.access_credential.delete'
  UNION ALL SELECT 'iam.resource.create'
  UNION ALL SELECT 'iam.resource.get'
  UNION ALL SELECT 'iam.resource.list'
  UNION ALL SELECT 'iam.resource.update'
  UNION ALL SELECT 'iam.resource.delete'
) c
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_permission p WHERE p.code = c.code);

-- ⑤ 管理员角色全量授予格式合法的权限码
-- （管理员 JWT 轨道走 isAdmin 旁路，此处授权服务于 AK/SK 轨道的统一快照判定）
INSERT INTO t_r_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM t_role r
JOIN t_permission p
  ON p.code REGEXP '^[a-z][a-z0-9_]*(\\.[a-z][a-z0-9_]*){2,}$'
WHERE r.code = '1000'
  AND NOT EXISTS (SELECT 1 FROM t_r_role_permission rp
                  WHERE rp.role_id = r.id AND rp.permission_id = p.id);

-- ⑥ t_resource 重构为前端组件资源表
DELETE FROM t_resource;
ALTER TABLE t_resource DROP COLUMN rpc_full_method;
ALTER TABLE t_resource ADD COLUMN permission_codes longtext;

-- ⑦ 播种前端组件树（与 pkg/model COMPONENT_SEEDS 镜像；固定 id，幂等）
INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1001, a.id, NULL, 'menu', 'iam', '身份访问管理', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1001);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1002, a.id, 1001, 'menu', 'account_management', '账号管理', NULL, 0, 1, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1002);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1003, a.id, 1001, 'menu', 'application_management', '应用管理', NULL, 0, 2, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1003);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1004, a.id, 1001, 'menu', 'authentication_management', '认证管理', NULL, 0, 3, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1004);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1005, a.id, 1001, 'menu', 'authorization_management', '权限管理', NULL, 0, 4, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1005);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1006, a.id, 1002, 'page', 'account_list', '用户列表', '["iam.account.list"]', 0, 1, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1006);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1007, a.id, 1003, 'page', 'application_list', '应用列表', '["iam.application.list"]', 0, 1, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1007);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1008, a.id, 1004, 'page', 'authentication_policy_list', '认证策略', NULL, 0, 1, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1008);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1009, a.id, 1004, 'page', 'access_credential_list', '访问凭证', '["iam.access_credential.list"]', 0, 2, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1009);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1010, a.id, 1005, 'page', 'permission_list', '权限列表', '["iam.permission.list"]', 0, 1, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1010);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1011, a.id, 1005, 'page', 'role_management', '角色管理', '["iam.role.list"]', 0, 2, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1011);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1012, a.id, 1005, 'page', 'permission_subscription', '权限订阅', '["iam.subscription.list"]', 0, 3, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1012);

INSERT INTO t_resource (id, application_id, parent_id, kind, `key`, name, permission_codes, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT 1013, a.id, 1005, 'page', 'resource_management', '资源管理', '["iam.resource.list"]', 0, 4, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE id = 1013);
