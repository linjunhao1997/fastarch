-- 回滚：恢复全局唯一（若已存在跨应用重复 code 将失败，需先人工清理）。
ALTER TABLE `t_dict_type` DROP KEY `uk_t_dict_type_app_code`;
ALTER TABLE `t_dict_type` ADD UNIQUE KEY `idx_t_dict_type_code` (`code`);
