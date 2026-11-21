package model

import "time"

// Member 是员工档案：account_id 弱引用 IAM 账号，姓名/职务是只读快照。
// current_role_id 是一人多角色场景下"当前正在使用的角色"。
type Member struct {
	Id            int64      `gorm:"column:id;primaryKey"`
	AccountId     int64      `gorm:"column:account_id;uniqueIndex:uk_member_account"`
	Name          string     `gorm:"column:name;size:64"`
	Title         string     `gorm:"column:title;size:64"`
	CurrentRoleId int64      `gorm:"column:current_role_id"`
	Version       int64      `gorm:"column:version"`
	IsDeleted     bool       `gorm:"column:is_deleted;default:false"`
	CreatedBy     string     `gorm:"column:created_by;size:64"`
	UpdatedBy     string     `gorm:"column:updated_by;size:64"`
	CreateTime    *time.Time `gorm:"column:create_time"`
	UpdateTime    *time.Time `gorm:"column:update_time"`
}

func (Member) TableName() string { return "t_member" }

const (
	MemberColumnId            = "id"
	MemberColumnAccountId     = "account_id"
	MemberColumnName          = "name"
	MemberColumnTitle         = "title"
	MemberColumnCurrentRoleId = "current_role_id"
	MemberColumnVersion       = "version"
	MemberColumnIsDeleted     = "is_deleted"
	MemberColumnCreateTime    = "create_time"
	MemberColumnUpdateTime    = "update_time"
	MemberColumnCreatedBy     = "created_by"
	MemberColumnUpdatedBy     = "updated_by"
)

var MemberFields = []string{
	MemberColumnId, MemberColumnAccountId, MemberColumnName, MemberColumnTitle,
	MemberColumnCurrentRoleId, MemberColumnVersion, MemberColumnIsDeleted,
	MemberColumnCreatedBy, MemberColumnUpdatedBy,
	MemberColumnCreateTime, MemberColumnUpdateTime,
}

// MemberRole 是"一个人能担任哪些工作角色"（t_r_member_role）。
type MemberRole struct {
	MemberId   int64      `gorm:"column:member_id;primaryKey"`
	RoleId     int64      `gorm:"column:role_id;primaryKey"`
	CreateTime *time.Time `gorm:"column:create_time"`
}

func (MemberRole) TableName() string { return "t_r_member_role" }

// RoleCodeBoss 是"能给别人定角色"的老板角色 code；唯一在业务层做身份判定的角色。
const RoleCodeBoss = "boss"
