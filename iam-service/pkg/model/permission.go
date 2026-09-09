package model

import "time"

// 权限类型（kind）：创建后不可变（specs/005-typed-permission-model FR-001）。
const (
	PermissionKindMenu    = "menu"
	PermissionKindHTTPAPI = "http_api"
	PermissionKindGRPC    = "grpc"
)

// IsValidPermissionKind 校验 kind 取值合法性。
func IsValidPermissionKind(kind string) bool {
	switch kind {
	case PermissionKindMenu, PermissionKindHTTPAPI, PermissionKindGRPC:
		return true
	}
	return false
}

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m permission.go
type Permission struct {
	Id            int64       `gorm:"column:id;primaryKey"`
	Name          string      `gorm:"column:name"`
	Kind          string      `gorm:"column:kind"`
	Code          string      `gorm:"column:code;size:255;uniqueIndex"`
	Value         string      `gorm:"column:value"`
	Description   string      `gorm:"column:description"`
	ApplicationId int64       `gorm:"column:application_id;index"`
	Disabled      bool        `gorm:"column:disabled"`
	CreateTime    time.Time   `gorm:"column:create_time"`
	UpdateTime    time.Time   `gorm:"column:update_time"`
	CreatedBy     int64       `gorm:"column:created_by"`
	UpdatedBy     int64       `gorm:"column:updated_by"`
	Version       int64       `gorm:"column:version;default:1"`
	Application   Application `gorm:"foreignKey:application_id;references:id"`
	RoleList      []*Role     `gorm:"many2many:t_r_role_permission;joinForeignKey:permission_id;joinReferences:role_id"`
	ResourceList  []*Resource `gorm:"many2many:t_r_permission_resource;joinForeignKey:permission_id;joinReferences:resource_id"`
}

func (a *Permission) TableName() string {
	return "t_permission"
}

func init() {
	migrate(Permission{})
}

var PermissionFields = []string{
	PermissionColumnId, PermissionColumnName, PermissionColumnKind, PermissionColumnCode,
	PermissionColumnValue, PermissionColumnDescription, PermissionColumnApplicationId,
	PermissionColumnDisabled, PermissionColumnCreateTime, PermissionColumnUpdateTime,
	PermissionColumnCreatedBy, PermissionColumnUpdatedBy, PermissionColumnVersion,
}
