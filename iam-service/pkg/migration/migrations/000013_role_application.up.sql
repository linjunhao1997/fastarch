-- 000013 角色所属应用：推翻 000011 的「角色全局」裁决（ADR 0006）。
-- 角色归属应用 = 数据边界：非 admin 仅可见/可操作与自身 home 一致的角色。
-- 0 = 未归属（fail-closed：非 admin 不可见，admin 旁路）。

ALTER TABLE `t_role`
  ADD COLUMN `application_id` bigint DEFAULT 0 AFTER `status`,
  ADD KEY `idx_t_role_application_id` (`application_id`);

-- 存量回填：按角色所挂权限的归属应用推导（取 id 最小者，稳定可复现）；
-- 无权限绑定的角色与 admin 角色归 IAM 控制台。
UPDATE t_role r
SET application_id = COALESCE(
  (
    SELECT p.application_id
    FROM t_r_role_permission rp
    JOIN t_permission p ON p.id = rp.permission_id
    WHERE rp.role_id = r.id AND p.application_id > 0
    ORDER BY p.id ASC LIMIT 1
  ),
  (SELECT id FROM t_application WHERE code = 'iam')
);

UPDATE t_role
SET application_id = (SELECT id FROM t_application WHERE code = 'iam')
WHERE code = 'admin';
