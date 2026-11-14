package model

import (
	"time"
)

// 组件类型（kind）：t_resource 是前端组件资源注册表（011 权限码模型裁决），
// 结构归代码、展示归管理员；组件经 permission_codes 绑定权限码，
// 前端按「组件绑定码 ∩ 用户持有码」决定展示或置灰。
const (
	ComponentKindMenu   = "menu"
	ComponentKindPage   = "page"
	ComponentKindButton = "button"
)

// IsValidComponentKind 校验组件类型取值合法性。
func IsValidComponentKind(kind string) bool {
	switch kind {
	case ComponentKindMenu, ComponentKindPage, ComponentKindButton:
		return true
	}
	return false
}

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m resource.go
type Resource struct {
	Id              int64     `gorm:"column:id;primaryKey"`
	ApplicationId   int64     `gorm:"column:application_id;not null;default:0;index;uniqueIndex:uk_resource_application_key"`
	ParentId        *int64    `gorm:"column:parent_id;index"`
	Kind            string    `gorm:"column:kind;size:32"`
	Key             string    `gorm:"column:key;size:255;uniqueIndex:uk_resource_application_key"`
	Name            string    `gorm:"column:name"`
	PermissionCodes []string  `gorm:"column:permission_codes;serializer:json"`
	Disabled        bool      `gorm:"column:disabled"`
	DisplayOrder    int32     `gorm:"column:display_order;default:0"`
	CreateTime      time.Time `gorm:"column:create_time"`
	UpdateTime      time.Time `gorm:"column:update_time"`
	CreatedBy       int64     `gorm:"column:created_by"`
	UpdatedBy       int64     `gorm:"column:updated_by"`
	Version         int64     `gorm:"column:version;default:1"`
}

func (r *Resource) TableName() string {
	return "t_resource"
}

func init() {
	migrate(Resource{})
}

var ResourceFields = []string{
	ResourceColumnId, ResourceColumnApplicationId, ResourceColumnParentId, ResourceColumnKind,
	ResourceColumnKey, ResourceColumnName, ResourceColumnPermissionCodes,
	ResourceColumnDisabled, ResourceColumnCreateTime,
	ResourceColumnUpdateTime, ResourceColumnCreatedBy, ResourceColumnUpdatedBy,
	ResourceColumnVersion, ResourceColumnDisplayOrder,
}
