-- 000003 历史重复绑定去重：uk_r_role_permission_role_permission 建立前
-- 产生的 (role_id, permission_id) 重复行，保留最小 id。在唯一索引约束下
-- 本语句天然幂等（无重复行时零删除）。

DELETE FROM t_r_role_permission
WHERE id NOT IN (
  SELECT min_id FROM (
    SELECT MIN(id) AS min_id FROM t_r_role_permission GROUP BY role_id, permission_id
  ) AS keep_rows
);
