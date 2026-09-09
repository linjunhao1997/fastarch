package model

import "time"

// Resource 是被保护的具体资源登记（specs/005-typed-permission-model FR-002）。
// Key 全局唯一，格式随 Kind：
//
//	menu     → 路由键，如 "authorization_management.permission_list"
//	http_api → "<METHOD> <path>"，如 "GET /authorization/permissions"
//	grpc     → full method，如 "/iam.v1.AccountService/ListAccounts"
//
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m resource.go
type Resource struct {
	Id            int64     `gorm:"column:id;primaryKey"`
	ApplicationId int64     `gorm:"column:application_id;not null;default:0;index;uniqueIndex:uk_resource_application_key"`
	ParentId      *int64    `gorm:"column:parent_id;index"`
	Kind          string    `gorm:"column:kind;index"`
	Key           string    `gorm:"column:key;size:255;uniqueIndex:uk_resource_application_key"`
	Name          string    `gorm:"column:name"`
	RpcFullMethod string    `gorm:"column:rpc_full_method"`
	Disabled      bool      `gorm:"column:disabled"`
	DisplayOrder  int32     `gorm:"column:display_order;default:0"`
	CreateTime    time.Time `gorm:"column:create_time"`
	UpdateTime    time.Time `gorm:"column:update_time"`
	CreatedBy     int64     `gorm:"column:created_by"`
	UpdatedBy     int64     `gorm:"column:updated_by"`
	Version       int64     `gorm:"column:version;default:1"`
}

func (r *Resource) TableName() string {
	return "t_resource"
}

func init() {
	migrate(Resource{})
}

var ResourceFields = []string{
	ResourceColumnId, ResourceColumnApplicationId, ResourceColumnParentId, ResourceColumnKind,
	ResourceColumnKey, ResourceColumnName,
	ResourceColumnRpcFullMethod, ResourceColumnDisabled, ResourceColumnCreateTime,
	ResourceColumnUpdateTime, ResourceColumnCreatedBy, ResourceColumnUpdatedBy,
	ResourceColumnVersion, ResourceColumnDisplayOrder,
}
