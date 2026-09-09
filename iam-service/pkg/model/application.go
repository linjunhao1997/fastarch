package model

import "time"

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m application.go
type Application struct {
	Id          int64     `gorm:"column:id;primaryKey"`
	Code        string    `gorm:"column:code"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	Disabled    int       `gorm:"column:disabled"`
	Kind        string    `gorm:"column:kind"`
	CreateTime  time.Time `gorm:"column:create_time"`
	UpdateTime  time.Time `gorm:"column:update_time"`
}

func (a *Application) TableName() string {
	return "t_application"
}

func init() {
	migrate(Application{})
}

var ApplicationFields = []string{
	ApplicationColumnId, ApplicationColumnCode, ApplicationColumnName,
	ApplicationColumnDescription, ApplicationColumnDisabled, ApplicationColumnKind,
	ApplicationColumnCreateTime, ApplicationColumnUpdateTime,
}
