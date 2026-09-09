package model

// PermissionResource 是权限与资源的绑定关系（specs/005-typed-permission-model FR-003）。
// (PermissionId, ResourceId) 唯一，服务层校验资源 kind 与权限 kind 一致。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m permission_resource.go
type PermissionResource struct {
	Id           int64 `gorm:"column:id;primaryKey"`
	PermissionId int64 `gorm:"column:permission_id;uniqueIndex:uk_permission_resource"`
	ResourceId   int64 `gorm:"column:resource_id;uniqueIndex:uk_permission_resource"`
}

func (p *PermissionResource) TableName() string {
	return "t_r_permission_resource"
}

func init() {
	migrate(PermissionResource{})
}

var PermissionResourceFields = []string{
	PermissionResourceColumnId, PermissionResourceColumnPermissionId, PermissionResourceColumnResourceId,
}
