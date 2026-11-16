-- 000011 角色-菜单绑定退役（纯码门槛裁决）：菜单可见性改由前端按 permission_code
-- 与用户快照码过滤，t_r_role_menu 不再参与任何判定，删除该表。
DROP TABLE IF EXISTS `t_r_role_menu`;
