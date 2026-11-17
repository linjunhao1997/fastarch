-- 000012 用户所属应用（模型 A）：默认进入的应用，必须属于准入集合。
-- 0 = 未设置（admin 旁路可不绑定准入应用）。幂等：迁移链一次性前进。

ALTER TABLE `t_user`
  ADD COLUMN `home_application_id` bigint DEFAULT 0 AFTER `status`,
  ADD KEY `idx_t_user_home_application_id` (`home_application_id`);

-- 回填：优先取首个已启用的准入应用（按绑定 id 顺序）。
UPDATE t_user u
SET home_application_id = (
  SELECT a.id FROM t_r_user_application ua
  JOIN t_application a ON a.id = ua.application_id
  WHERE ua.user_id = u.id AND a.status = 'enabled'
  ORDER BY ua.id ASC LIMIT 1
)
WHERE home_application_id = 0
  AND EXISTS (
    SELECT 1 FROM t_r_user_application ua
    JOIN t_application a ON a.id = ua.application_id
    WHERE ua.user_id = u.id AND a.status = 'enabled'
  );

-- admin 角色旁路准入绑定，默认归属 IAM 控制台。
UPDATE t_user u
SET home_application_id = (SELECT id FROM t_application WHERE code = 'iam')
WHERE home_application_id = 0
  AND EXISTS (
    SELECT 1 FROM t_r_user_role ur
    JOIN t_role r ON r.id = ur.role_id
    WHERE ur.user_id = u.id AND r.code = 'admin'
  );
