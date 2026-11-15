package model

import "time"

// Permission 权限模型（t_permission，fastarch_dev 基线 Schema）：
// code 为点式权限码（服务.模块.操作，唯一授权词汇），type 取值 api 等词汇类别。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type Permission struct {
	Id   int64  `gorm:"column:id;primaryKey"`
	Code string `gorm:"column:code;size:64;uniqueIndex"`
	// ApplicationId 码归属应用（由码服务段派生，全局授予不变、仅管理数据隔离）
	ApplicationId int64     `gorm:"column:application_id;index"`
	Name          string    `gorm:"column:name;size:64"`
	Type          string    `gorm:"column:type;size:16"`
	Description   string    `gorm:"column:description;size:255"`
	Status        string    `gorm:"column:status;size:16;default:enabled"`
	Version       int64     `gorm:"column:version"`
	CreatedBy     string    `gorm:"column:created_by;size:64"`
	UpdatedBy     string    `gorm:"column:updated_by;size:64"`
	CreateTime    time.Time `gorm:"column:create_time"`
	UpdateTime    time.Time `gorm:"column:update_time"`
	RoleList      []*Role   `gorm:"many2many:t_r_role_permission;joinForeignKey:permission_id;joinReferences:role_id"`
}

func (a *Permission) TableName() string {
	return "t_permission"
}

var PermissionFields = []string{
	PermissionColumnId, PermissionColumnCode, PermissionColumnApplicationId, PermissionColumnName, PermissionColumnType,
	PermissionColumnDescription, PermissionColumnStatus, PermissionColumnVersion,
	PermissionColumnCreatedBy, PermissionColumnUpdatedBy, PermissionColumnCreateTime,
	PermissionColumnUpdateTime,
}
