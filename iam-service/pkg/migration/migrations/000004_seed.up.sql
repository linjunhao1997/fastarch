-- 000004 数据录入：内置应用、内置权限/资源、管理员角色及绑定、历史数据回填。
-- 全部按自然键（code / kind+key）幂等：全新库全量插入，存量库仅补缺失行。
-- 不写死主键 id：由 AUTO_INCREMENT 分配，绑定关系经 JOIN 自然键落库。

-- ① 内置应用
INSERT INTO t_application (code, name, description, disabled, create_time, update_time)
SELECT 'iam-service', 'IAM', NULL, 0, NOW(3), NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM t_application WHERE code = 'iam-service');

-- ② 历史管理员角色迁移：legacy 'admin' → canonical '1000'
-- ②-1 canonical 不存在时原地改名（code 有唯一索引，legacy 至多一行）
UPDATE t_role SET code = '1000', update_time = NOW(3)
WHERE code = 'admin'
  AND (SELECT cnt FROM (SELECT COUNT(*) AS cnt FROM t_role WHERE code = '1000') AS c) = 0;

-- ②-2 两者并存（历史中间版本产生）：绑定并入 canonical 后删除 legacy
DELETE ar FROM t_r_account_role ar
JOIN t_role l ON l.id = ar.role_id AND l.code = 'admin'
JOIN t_role c ON c.code = '1000' AND c.id <> l.id
JOIN t_r_account_role dup ON dup.account_id = ar.account_id AND dup.role_id = c.id;

UPDATE t_r_account_role ar
JOIN t_role l ON l.id = ar.role_id AND l.code = 'admin'
JOIN t_role c ON c.code = '1000' AND c.id <> l.id
SET ar.role_id = c.id;

DELETE rp FROM t_r_role_permission rp
JOIN t_role l ON l.id = rp.role_id AND l.code = 'admin'
JOIN t_role c ON c.code = '1000' AND c.id <> l.id
JOIN t_r_role_permission dup ON dup.role_id = c.id AND dup.permission_id = rp.permission_id;

UPDATE t_r_role_permission rp
JOIN t_role l ON l.id = rp.role_id AND l.code = 'admin'
JOIN t_role c ON c.code = '1000' AND c.id <> l.id
SET rp.role_id = c.id;

DELETE FROM t_role
WHERE code = 'admin'
  AND (SELECT cnt FROM (SELECT COUNT(*) AS cnt FROM t_role WHERE code = '1000') AS c) > 0;

-- ③ 内置管理员角色
INSERT INTO t_role (name, code, description, application_id, disabled, create_time, update_time, created_by, updated_by, version)
SELECT 'Administrator', '1000', NULL, a.id, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_role WHERE code = '1000');

-- ④ 内置 "*" 权限（每种 kind 一个）
INSERT INTO t_permission (name, kind, code, value, description, application_id, disabled, create_time, update_time, created_by, updated_by, version)
SELECT 'All Menu Access', 'menu', 'builtin.menu.all', '*', 'Full access to all menus', a.id, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'builtin.menu.all');

INSERT INTO t_permission (name, kind, code, value, description, application_id, disabled, create_time, update_time, created_by, updated_by, version)
SELECT 'All HTTP Access', 'http_api', 'builtin.http.all', '*', 'Full access to all HTTP endpoints', a.id, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'builtin.http.all');

INSERT INTO t_permission (name, kind, code, value, description, application_id, disabled, create_time, update_time, created_by, updated_by, version)
SELECT 'All gRPC Access', 'grpc', 'builtin.grpc.all', '*', 'Full access to all gRPC methods', a.id, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'builtin.grpc.all');

-- ⑤ 凭证域资源登记（specs/009：7 条 HTTP 路由 + 7 个 gRPC full method + 1 个菜单键）
INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'http_api', 'POST /authentication/access-credentials', '创建访问凭证', '/iam.v1.AccessCredentialService/CreateAccessCredential', 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'http_api' AND `key` = 'POST /authentication/access-credentials');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'http_api', 'GET /authentication/access-credentials', '凭证列表', '/iam.v1.AccessCredentialService/ListAccessCredentials', 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'http_api' AND `key` = 'GET /authentication/access-credentials');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'http_api', 'GET /authentication/access-credentials/{id}', '凭证详情', '/iam.v1.AccessCredentialService/GetAccessCredential', 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'http_api' AND `key` = 'GET /authentication/access-credentials/{id}');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'http_api', 'PATCH /authentication/access-credentials/{id}', '编辑凭证', '/iam.v1.AccessCredentialService/UpdateAccessCredential', 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'http_api' AND `key` = 'PATCH /authentication/access-credentials/{id}');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'http_api', 'POST /authentication/access-credentials/{id}/status', '启用/禁用凭证', '/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus', 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'http_api' AND `key` = 'POST /authentication/access-credentials/{id}/status');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'http_api', 'POST /authentication/access-credentials/{id}/reset-secret', '重置凭证密钥', '/iam.v1.AccessCredentialService/ResetAccessCredentialSecret', 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'http_api' AND `key` = 'POST /authentication/access-credentials/{id}/reset-secret');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'http_api', 'DELETE /authentication/access-credentials/{id}', '删除凭证', '/iam.v1.AccessCredentialService/DeleteAccessCredential', 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'http_api' AND `key` = 'DELETE /authentication/access-credentials/{id}');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'grpc', '/iam.v1.AccessCredentialService/CreateAccessCredential', '创建访问凭证', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'grpc' AND `key` = '/iam.v1.AccessCredentialService/CreateAccessCredential');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'grpc', '/iam.v1.AccessCredentialService/ListAccessCredentials', '凭证列表', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'grpc' AND `key` = '/iam.v1.AccessCredentialService/ListAccessCredentials');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'grpc', '/iam.v1.AccessCredentialService/GetAccessCredential', '凭证详情', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'grpc' AND `key` = '/iam.v1.AccessCredentialService/GetAccessCredential');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'grpc', '/iam.v1.AccessCredentialService/UpdateAccessCredential', '编辑凭证', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'grpc' AND `key` = '/iam.v1.AccessCredentialService/UpdateAccessCredential');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'grpc', '/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus', '启用/禁用凭证', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'grpc' AND `key` = '/iam.v1.AccessCredentialService/ChangeAccessCredentialStatus');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'grpc', '/iam.v1.AccessCredentialService/ResetAccessCredentialSecret', '重置凭证密钥', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'grpc' AND `key` = '/iam.v1.AccessCredentialService/ResetAccessCredentialSecret');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'grpc', '/iam.v1.AccessCredentialService/DeleteAccessCredential', '删除凭证', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'grpc' AND `key` = '/iam.v1.AccessCredentialService/DeleteAccessCredential');

INSERT INTO t_resource (application_id, kind, `key`, name, rpc_full_method, disabled, display_order, create_time, update_time, created_by, updated_by, version)
SELECT a.id, 'menu', 'access_credential_list', '访问凭证', NULL, 0, 0, NOW(3), NOW(3), a.id, a.id, 1
FROM t_application a
WHERE a.code = 'iam-service'
  AND NOT EXISTS (SELECT 1 FROM t_resource WHERE application_id = a.id AND kind = 'menu' AND `key` = 'access_credential_list');

-- ⑥ 历史资源归属回填：application_id = 0 的资源归属内置应用
UPDATE t_resource r
JOIN t_application a ON a.code = 'iam-service'
SET r.application_id = a.id, r.updated_by = a.id, r.update_time = NOW(3)
WHERE r.application_id = 0;

-- ⑦ 权限-资源绑定：builtin 权限绑定同 kind 全部资源
INSERT INTO t_r_permission_resource (permission_id, resource_id)
SELECT p.id, r.id
FROM t_permission p
JOIN t_resource r ON r.kind = p.kind
WHERE p.code IN ('builtin.menu.all', 'builtin.http.all', 'builtin.grpc.all')
  AND NOT EXISTS (SELECT 1 FROM t_r_permission_resource pr
                  WHERE pr.permission_id = p.id AND pr.resource_id = r.id);

-- ⑧ 管理员角色授予全部内置权限
INSERT INTO t_r_role_permission (role_id, permission_id)
SELECT ro.id, p.id
FROM t_role ro
JOIN t_permission p ON p.code IN ('builtin.menu.all', 'builtin.http.all', 'builtin.grpc.all')
WHERE ro.code = '1000'
  AND NOT EXISTS (SELECT 1 FROM t_r_role_permission rp
                  WHERE rp.role_id = ro.id AND rp.permission_id = p.id);

-- ⑨ 账号-应用归属回填：由启用角色推导（specs/008 初始化逻辑）
INSERT INTO t_r_account_application (account_id, application_id)
SELECT DISTINCT ar.account_id, r.application_id
FROM t_r_account_role ar
JOIN t_role r ON r.id = ar.role_id AND r.disabled = 0
WHERE NOT EXISTS (SELECT 1 FROM t_r_account_application aa
                  WHERE aa.account_id = ar.account_id AND aa.application_id = r.application_id);

-- ⑩ 补齐应用级资源唯一索引（归属回填完成后执行，远古库缺该索引；
--    历史脏数据若存在 (application_id, key) 重复将在此 fail-fast 暴露）
DROP PROCEDURE IF EXISTS ensure_resource_uk;

CREATE PROCEDURE ensure_resource_uk()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 't_resource'
      AND INDEX_NAME = 'uk_resource_application_key'
  ) THEN
    ALTER TABLE `t_resource` ADD UNIQUE KEY `uk_resource_application_key` (`application_id`, `key`);
  END IF;
END;

CALL ensure_resource_uk();

DROP PROCEDURE IF EXISTS ensure_resource_uk;
