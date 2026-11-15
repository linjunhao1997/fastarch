-- 000008 字典类型编码唯一性改为应用内唯一（多应用裁决 #8）：
-- 全局唯一 idx_t_dict_type_code → 复合唯一 (application_id, code)，
-- 不同应用可各自定义同名业务字典（如 gender）。
ALTER TABLE `t_dict_type` DROP KEY `idx_t_dict_type_code`;
ALTER TABLE `t_dict_type` ADD UNIQUE KEY `uk_t_dict_type_app_code` (`application_id`, `code`);
