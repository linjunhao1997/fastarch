-- 000011 回滚：重建 t_r_role_menu（DDL 与 000001 镜像；历史绑定数据不可恢复）。
CREATE TABLE IF NOT EXISTS `t_r_role_menu` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `role_id` bigint DEFAULT NULL,
  `menu_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_t_r_role_menu_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
