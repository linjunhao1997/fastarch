-- 000016 rollback：恢复用户所属应用列。
-- home 已删除，无法还原历史归属值；按 000012 的回填规则从首个启用的接入应用重建，
-- 供回滚后的旧版本代码使用（旧版本 home 仅作默认落地/回落，接入关系本身仍在绑定表）。

ALTER TABLE `t_user`
  ADD COLUMN `home_application_id` bigint DEFAULT 0 AFTER `status`,
  ADD KEY `idx_t_user_home_application_id` (`home_application_id`);

UPDATE `t_user` u
SET `home_application_id` = (
  SELECT a.`id` FROM `t_r_user_application` ua
  JOIN `t_application` a ON a.`id` = ua.`application_id`
  WHERE ua.`user_id` = u.`id` AND a.`status` = 'enabled'
  ORDER BY ua.`id` ASC LIMIT 1
);
