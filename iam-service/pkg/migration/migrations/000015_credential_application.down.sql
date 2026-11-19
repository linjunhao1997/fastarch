-- 000015 rollback：移除凭证归属应用列。
ALTER TABLE `t_access_credential` DROP COLUMN `application_id`;
