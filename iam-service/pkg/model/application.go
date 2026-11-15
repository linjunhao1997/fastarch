package model

import "time"

// Application 应用模型（t_application，fastarch_dev 基线 Schema）：
// app_id 唯一应用标识，secret_hash 为应用密钥摘要（AK/SK 双轨的应用级凭证）。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type Application struct {
	Id    int64  `gorm:"column:id;primaryKey"`
	AppId string `gorm:"column:app_id;size:64;uniqueIndex"`
	// Code 为权限码服务段命名空间（iam/order/crm…，多应用裁决 #7-#9）：
	// 该应用名下菜单绑定的权限码、归属权限码的服务段必须等于本值。
	Code       string    `gorm:"column:code;size:32;uniqueIndex"`
	Name       string    `gorm:"column:name;size:64"`
	SecretHash string    `gorm:"column:secret_hash;size:255"`
	Status     string    `gorm:"column:status;size:16;default:enabled"`
	Version    int64     `gorm:"column:version"`
	CreatedBy  string    `gorm:"column:created_by;size:64"`
	UpdatedBy  string    `gorm:"column:updated_by;size:64"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateTime time.Time `gorm:"column:update_time"`
}

func (a *Application) TableName() string {
	return "t_application"
}

var ApplicationFields = []string{
	ApplicationColumnId, ApplicationColumnAppId, ApplicationColumnCode, ApplicationColumnName,
	ApplicationColumnSecretHash, ApplicationColumnStatus, ApplicationColumnVersion,
	ApplicationColumnCreatedBy, ApplicationColumnUpdatedBy,
	ApplicationColumnCreateTime, ApplicationColumnUpdateTime,
}
