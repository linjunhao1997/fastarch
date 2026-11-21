package model

import "time"

// WorkRole 是工作角色（环节职责），与 IAM 的授权角色（t_role，挂权限码）是两套概念。
// 归属应用固定为 flow（application_id=3）。
type WorkRole struct {
	Id            int64     `gorm:"column:id;primaryKey"`
	ApplicationId int64     `gorm:"column:application_id;index"`
	Code          string    `gorm:"column:code;size:64;uniqueIndex:uk_work_role_code"`
	Name          string    `gorm:"column:name;size:64"`
	Stage         string    `gorm:"column:stage;size:64"`
	Sort          int       `gorm:"column:sort"`
	Status        string    `gorm:"column:status;size:16;default:enabled"`
	Version       int64     `gorm:"column:version"`
	CreatedBy     string    `gorm:"column:created_by;size:64"`
	UpdatedBy     string    `gorm:"column:updated_by;size:64"`
	CreateTime    time.Time `gorm:"column:create_time"`
	UpdateTime    time.Time `gorm:"column:update_time"`
}

func (WorkRole) TableName() string { return "t_work_role" }

const (
	WorkRoleColumnId            = "id"
	WorkRoleColumnApplicationId = "application_id"
	WorkRoleColumnCode          = "code"
	WorkRoleColumnName          = "name"
	WorkRoleColumnStage         = "stage"
	WorkRoleColumnSort          = "sort"
	WorkRoleColumnStatus        = "status"
)

// WorkRoleFields 显式查询列清单（禁止 SELECT *）。
var WorkRoleFields = []string{
	WorkRoleColumnId, WorkRoleColumnApplicationId, WorkRoleColumnCode,
	WorkRoleColumnName, WorkRoleColumnStage, WorkRoleColumnSort,
	WorkRoleColumnStatus, "version", "created_by", "updated_by", "create_time", "update_time",
}
