-- 000016 删除用户所属应用列（ADR 0008 收尾）：
-- home 的语义（隐式准入/数据边界回落/账号归属）已由接入集合 t_r_user_application
-- 与请求头数据范围 dataAppId 完全承接，物理列不再需要。
--
-- 删列前先把仍未被显式绑定覆盖的 home 补写成接入绑定，避免丢失用户的接入关系。
-- 000015 已在此前把存量凭证的 application_id 按 home 回填完毕，删列不影响凭证归属。

INSERT IGNORE INTO `t_r_user_application` (`user_id`, `application_id`)
SELECT u.`id`, u.`home_application_id`
FROM `t_user` u
WHERE u.`home_application_id` > 0
  AND EXISTS (SELECT 1 FROM `t_application` a WHERE a.`id` = u.`home_application_id` AND a.`status` = 'enabled');

ALTER TABLE `t_user`
  DROP KEY `idx_t_user_home_application_id`,
  DROP COLUMN `home_application_id`;
