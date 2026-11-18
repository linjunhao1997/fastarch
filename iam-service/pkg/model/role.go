package model

import (
	"time"
)

const RoleAdmin = "admin"

// Role 角色模型（t_role，fastarch_dev 基线 Schema）：code 应用内唯一业务码
// （复合唯一 application_id+code，000014 起），status 取值 enabled/disabled；
// 与权限经关联表绑定。菜单为纯展示配置（纯码门槛裁决），角色-菜单绑定已退役。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type Role struct {
	Id          int64  `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;size:64;uniqueIndex:uk_t_role_app_code,priority:2"`
	Name        string `gorm:"column:name;size:64"`
	Description string `gorm:"column:description;size:255"`
	Status      string `gorm:"column:status;size:16;default:enabled"`
	// ApplicationId 是角色归属应用（数据边界，ADR 0006）：非 admin 仅可见 home 一致的角色。
	ApplicationId int64 `gorm:"column:application_id;index;uniqueIndex:uk_t_role_app_code,priority:1"`
	// IsSuperAdmin 平台超管标志（000014 起）：登录 IsAdmin、禁删、非 admin 禁绑
	// 均按本标志判定，与 code 解耦——各应用可自建 'admin' 应用级角色而不获得平台超管。
	// 服务端控制：仅种子/迁移置位，CreateRole/UpdateRole 不接受该字段。
	IsSuperAdmin   bool          `gorm:"column:is_super_admin;default:0"`
	Version        int64         `gorm:"column:version"`
	CreatedBy      string        `gorm:"column:created_by;size:64"`
	UpdatedBy      string        `gorm:"column:updated_by;size:64"`
	CreateTime     time.Time     `gorm:"column:create_time"`
	UpdateTime     time.Time     `gorm:"column:update_time"`
	PermissionList []*Permission `gorm:"many2many:t_r_role_permission;joinForeignKey:role_id;joinReferences:permission_id"`
}

type Roles []*Role

func (r *Role) TableName() string {
	return "t_role"
}

var RoleFields = []string{
	RoleColumnId, RoleColumnCode, RoleColumnName, RoleColumnDescription,
	RoleColumnStatus, RoleColumnApplicationId, RoleColumnIsSuperAdmin, RoleColumnVersion, RoleColumnCreatedBy, RoleColumnUpdatedBy,
	RoleColumnCreateTime, RoleColumnUpdateTime,
}
