package model

// RRoleMenu 角色-菜单关联表（t_r_role_menu）：无独立审计字段，
// 绑定变更审计记录在角色操作日志中。
// 删除策略: 基础架构型关联表（物理删除 + 引用检查）
type RRoleMenu struct {
	Id     int64 `gorm:"column:id;primaryKey"`
	RoleId int64 `gorm:"column:role_id;index"`
	MenuId int64 `gorm:"column:menu_id"`
}

func (r *RRoleMenu) TableName() string {
	return "t_r_role_menu"
}

var RRoleMenuFields = []string{
	RRoleMenuColumnId, RRoleMenuColumnRoleId, RRoleMenuColumnMenuId,
}
