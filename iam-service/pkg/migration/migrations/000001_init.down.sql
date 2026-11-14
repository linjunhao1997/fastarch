-- 回滚基线 Schema：按依赖无关顺序删除全部业务表（无物理外键，可任意序）。
DROP TABLE IF EXISTS `t_access_credential`;
DROP TABLE IF EXISTS `t_resource`;
DROP TABLE IF EXISTS `t_r_permission_resource`;
DROP TABLE IF EXISTS `t_r_role_permission`;
DROP TABLE IF EXISTS `t_r_account_application`;
DROP TABLE IF EXISTS `t_r_account_role`;
DROP TABLE IF EXISTS `t_role`;
DROP TABLE IF EXISTS `t_permission`;
DROP TABLE IF EXISTS `t_application`;
DROP TABLE IF EXISTS `t_account`;
