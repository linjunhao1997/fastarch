-- 000004 t_user 增加 email 列（唯一索引，可空）：登录支持 username 或 email 双标识。
-- 2026-11-15 裁决更新：由"仅 username"改为"username/email 双标识"。
ALTER TABLE `t_user`
  ADD COLUMN `email` varchar(255) DEFAULT NULL AFTER `nickname`,
  ADD UNIQUE KEY `idx_t_user_email` (`email`);
