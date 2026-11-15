package model

import "time"

// Menu 菜单模型（t_menu，fastarch_dev 基线 Schema）：菜单树取代旧组件资源模型，
// permission_code 绑定权限码（唯一授权词汇），前端按「菜单绑定码 ∩ 用户持有码」
// 或角色-菜单绑定决定展示。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m menu.go
type Menu struct {
	Id             int64     `gorm:"column:id;primaryKey"`
	ParentId       int64     `gorm:"column:parent_id;index;default:0"`
	Name           string    `gorm:"column:name;size:64"`
	Path           string    `gorm:"column:path;size:128"`
	Icon           string    `gorm:"column:icon;size:64"`
	Sort           int32     `gorm:"column:sort;default:0"`
	PermissionCode string    `gorm:"column:permission_code;size:64"`
	Status         string    `gorm:"column:status;size:16;default:enabled"`
	Version        int64     `gorm:"column:version"`
	CreatedBy      string    `gorm:"column:created_by;size:64"`
	UpdatedBy      string    `gorm:"column:updated_by;size:64"`
	CreateTime     time.Time `gorm:"column:create_time"`
	UpdateTime     time.Time `gorm:"column:update_time"`
	RoleList       []*Role   `gorm:"many2many:t_r_role_menu;joinForeignKey:menu_id;joinReferences:role_id"`
}

func (m *Menu) TableName() string {
	return "t_menu"
}

var MenuFields = []string{
	MenuColumnId, MenuColumnParentId, MenuColumnName, MenuColumnPath, MenuColumnIcon,
	MenuColumnSort, MenuColumnPermissionCode, MenuColumnStatus, MenuColumnVersion,
	MenuColumnCreatedBy, MenuColumnUpdatedBy, MenuColumnCreateTime, MenuColumnUpdateTime,
}
