package model

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type RRolePermission struct {
	Id           int64 `gorm:"column:id;primaryKey"`
	RoleId       int64 `gorm:"column:role_id;uniqueIndex:uk_r_role_permission_role_permission;index"`
	PermissionId int64 `gorm:"column:permission_id;uniqueIndex:uk_r_role_permission_role_permission;index"`
}

func (r *RRolePermission) TableName() string {
	return "t_r_role_permission"
}

var RRolePermissionFields = []string{
	RRolePermissionColumnId, RRolePermissionColumnRoleId, RRolePermissionColumnPermissionId,
}

func init() {
	migrate(RRolePermission{})
}
