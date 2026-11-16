package model

import (
	"time"

	contribmodel "github.com/linjunhao1997/contrib/model"
)

const RoleAdmin = "admin"

// Role 角色模型（t_role，fastarch_dev 基线 Schema）：code 唯一业务码，
// status 取值 enabled/disabled；与权限、菜单经关联表绑定。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type Role struct {
	Id             int64                `gorm:"column:id;primaryKey"`
	Code           string               `gorm:"column:code;size:64;uniqueIndex"`
	Name           string               `gorm:"column:name;size:64"`
	Description    string               `gorm:"column:description;size:255"`
	Status         string               `gorm:"column:status;size:16;default:enabled"`
	Version        int64                `gorm:"column:version"`
	CreatedBy      string               `gorm:"column:created_by;size:64"`
	UpdatedBy      string               `gorm:"column:updated_by;size:64"`
	CreateTime     time.Time            `gorm:"column:create_time"`
	UpdateTime     time.Time            `gorm:"column:update_time"`
	PermissionList []*Permission        `gorm:"many2many:t_r_role_permission;joinForeignKey:role_id;joinReferences:permission_id"`
	MenuList       []*contribmodel.Menu `gorm:"many2many:t_r_role_menu;joinForeignKey:role_id;joinReferences:menu_id"`
}

type Roles []*Role

func (r *Role) TableName() string {
	return "t_role"
}

var RoleFields = []string{
	RoleColumnId, RoleColumnCode, RoleColumnName, RoleColumnDescription,
	RoleColumnStatus, RoleColumnVersion, RoleColumnCreatedBy, RoleColumnUpdatedBy,
	RoleColumnCreateTime, RoleColumnUpdateTime,
}
