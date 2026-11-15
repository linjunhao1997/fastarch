package model

// RUserRole 用户-角色关联表（t_r_user_role）：无独立审计字段，
// 归属变更审计记录在用户操作日志中。
// 删除策略: 基础架构型关联表（物理删除 + 引用检查）
//
//go:generate contrib gen_gorm_mysql_repository -m r_user_role.go
type RUserRole struct {
	Id     int64 `gorm:"column:id;primaryKey"`
	UserId int64 `gorm:"column:user_id;index"`
	RoleId int64 `gorm:"column:role_id"`
}

func (r *RUserRole) TableName() string {
	return "t_r_user_role"
}

var RUserRoleFields = []string{
	RUserRoleColumnId, RUserRoleColumnUserId, RUserRoleColumnRoleId,
}
