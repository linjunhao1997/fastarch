-- 回滚：恢复 pjm 角色及成员的担任关系。
UPDATE t_work_role
SET status = 'enabled', update_time = NOW(3)
WHERE code = 'pjm';

INSERT INTO t_r_member_role (member_id, role_id, create_time)
SELECT m.id, r.id, NOW(3)
FROM t_member m
JOIN t_work_role r ON r.code = 'pjm'
WHERE m.is_deleted = 0
  AND NOT EXISTS (
    SELECT 1 FROM t_r_member_role mr
    WHERE mr.member_id = m.id AND mr.role_id = r.id
  );
