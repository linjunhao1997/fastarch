package model

import "time"

// Menu 菜单模型（t_menu）：通用菜单树能力（ADR 0001，每服务嵌入自持）。
// permission_code 单列绑定权限码（可空）；展示结构归管理员维护，授权判定归宿主。
// 归属应用 application_id：多应用数据隔离（应用上下文由宿主 token 承载）。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type Menu struct {
	Id int64 `gorm:"column:id;primaryKey"`
	ParentId int64 `gorm:"column:parent_id;index;default:0"`
	// ApplicationId 归属应用：菜单树按应用独立
	ApplicationId  int64     `gorm:"column:application_id;index"`
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
}

func (m *Menu) TableName() string {
	return "t_menu"
}

var MenuFields = []string{
	MenuColumnId, MenuColumnParentId, MenuColumnApplicationId, MenuColumnName, MenuColumnPath, MenuColumnIcon,
	MenuColumnSort, MenuColumnPermissionCode, MenuColumnStatus, MenuColumnVersion,
	MenuColumnCreatedBy, MenuColumnUpdatedBy, MenuColumnCreateTime, MenuColumnUpdateTime,
}

// RRoleMenu 角色-菜单关联表（t_r_role_menu）：无独立审计字段，
// 绑定变更审计记录在角色操作日志中。角色概念属宿主，本表只存绑定关系；
// 宿主经绑定关系做页面码授权推导（勾菜单=授码）。
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
