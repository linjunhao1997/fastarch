-- 000010 回滚：移除 sender.* 权限码（含角色绑定引用）与 sender 应用。
-- sender 的菜单/字典数据在其自持库，不在 IAM，此处不涉及。
DELETE rp FROM t_r_role_permission rp
JOIN t_permission p ON p.id = rp.permission_id
WHERE p.code LIKE 'sender.%';
DELETE FROM t_permission WHERE code LIKE 'sender.%';
DELETE FROM t_application WHERE code = 'sender';
