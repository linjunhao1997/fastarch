-- 回滚 000007：列随 DROP COLUMN 自动移除其单列索引；order:* 样例不可恢复（属清理）。
DROP TABLE IF EXISTS `t_r_user_application`;
ALTER TABLE `t_permission` DROP COLUMN `application_id`;
ALTER TABLE `t_dict_type` DROP COLUMN `application_id`;
ALTER TABLE `t_menu` DROP COLUMN `application_id`;
DELETE FROM t_application WHERE code = 'iam';
ALTER TABLE `t_application` DROP COLUMN `code`;
