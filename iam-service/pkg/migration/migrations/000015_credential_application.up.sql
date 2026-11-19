-- 000015 访问凭证归属应用：凭证的归属应用决定其机机调用产生的数据归属，
-- 与绑定用户的权限来源（user_id）正交。回填为绑定用户的 home 作为基线，
-- 之后创建凭证时由创建者显式指定 ∈ 其接入集合的应用。
ALTER TABLE `t_access_credential`
  ADD COLUMN `application_id` bigint DEFAULT NULL AFTER `user_id`,
  ADD KEY `idx_t_access_credential_application_id` (`application_id`);

-- 存量回填：归属取绑定用户的 home（与新模型"默认落地应用"一致）。
UPDATE `t_access_credential` c
  INNER JOIN `t_user` u ON c.`user_id` = u.`id`
SET c.`application_id` = u.`home_application_id`
WHERE c.`application_id` IS NULL;
