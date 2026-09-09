package model

import "time"

const (
	RoleRoot  = "0000"
	RoleAdmin = "1000"
	// RoleAdminLegacy 是历史版本的管理员角色编码，仅用于 Seeder 数据迁移
	// （specs/005-typed-permission-model F4）。
	RoleAdminLegacy = "admin"
)

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m role.go
type Role struct {
	Id             int64         `gorm:"column:id;primaryKey"`
	Name           string        `gorm:"column:name"`
	Code           string        `gorm:"column:code;size:255;uniqueIndex"`
	Description    string        `gorm:"column:description"`
	ApplicationId  int64         `gorm:"column:application_id;index"`
	Disabled       bool          `gorm:"column:disabled"`
	CreateTime     time.Time     `gorm:"column:create_time"`
	UpdateTime     time.Time     `gorm:"column:update_time"`
	CreatedBy      int64         `gorm:"column:created_by"`
	UpdatedBy      int64         `gorm:"column:updated_by"`
	Version        int64         `gorm:"column:version;default:1"`
	Application    Application   `gorm:"foreignKey:application_id;references:id"`
	PermissionList []*Permission `gorm:"many2many:t_r_role_permission;joinForeignKey:role_id;joinReferences:permission_id"`
}

type Roles []*Role

func (r *Role) TableName() string {
	return "t_role"
}

var RoleFields = []string{
	RoleColumnId, RoleColumnName, RoleColumnCode, RoleColumnDescription,
	RoleColumnApplicationId, RoleColumnDisabled, RoleColumnCreateTime,
	RoleColumnUpdateTime, RoleColumnCreatedBy, RoleColumnUpdatedBy,
	RoleColumnVersion,
}

func init() {
	migrate(Role{})
}
