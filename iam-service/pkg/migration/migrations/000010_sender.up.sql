-- 000010 sender-service 第二宿主接入（ADR 0001/0003）：sender 应用注册 + sender.* 权限码种子。
-- 菜单/字典数据归 sender 自持库（不在 IAM）；IAM 只登记应用与权限码（授权词汇归 IAM）。
-- 幂等：INSERT ... WHERE NOT EXISTS。

-- 1) sender 应用（固定 id=2 供 sender 库跨库弱引用 application_id；
--    secret_hash 为 dev secret 明文 'sender-dev-secret-please-change-in-prod' 的 sha256 hex，
--    生产经应用管理页重置密钥，dev 明文经环境变量注入 sender，不入仓）。
INSERT INTO t_application (id, app_id, code, name, secret_hash, status, version, created_by, updated_by, create_time, update_time)
SELECT 2, '2', 'sender', '消息中心', '93a6fc618f40703f210cb67b1fe0b9d2f57a554854177d4f3a360fd2d65fb337', 'enabled', 1, 'system', 'system', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM t_application WHERE code = 'sender');

-- 2) sender.* 权限码（服务段 sender = 应用 code；application_id 归 sender 应用）
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.record.send', '消息发送', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.record.send');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.record.get', '发送记录查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.record.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.record.list', '发送记录列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.record.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.record.delete', '发送记录删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.record.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.template.create', '模板创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.template.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.template.get', '模板查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.template.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.template.list', '模板列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.template.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.template.update', '模板更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.template.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.template.delete', '模板删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.template.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.menu.create', '菜单创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.menu.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.menu.get', '菜单查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.menu.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.menu.list', '菜单列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.menu.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.menu.update', '菜单更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.menu.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.menu.delete', '菜单删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.menu.delete');

INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.dict.create', '字典创建', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.dict.create');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.dict.get', '字典查询', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.dict.get');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.dict.list', '字典列表', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.dict.list');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.dict.update', '字典更新', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.dict.update');
INSERT INTO t_permission (code, name, type, description, status, version, created_by, updated_by, create_time, update_time, application_id)
SELECT 'sender.dict.delete', '字典删除', 'api', '', 'enabled', 1, 'system', 'system', NOW(3), NOW(3), (SELECT id FROM t_application WHERE code = 'sender')
WHERE NOT EXISTS (SELECT 1 FROM t_permission WHERE code = 'sender.dict.delete');
