package model

import "time"

// User 用户模型（t_user，fastarch_dev 基线 Schema）：
// username 为唯一登录名，email 为可空唯一邮箱（双标识登录），password_hash 为
// bcrypt 摘要；status 取值 enabled/disabled。
// 审计字段 created_by/updated_by 为 varchar（记录操作者账号 id 字符串）。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type User struct {
	Id           int64  `gorm:"column:id;primaryKey"`
	Username     string `gorm:"column:username;size:64;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash;size:255"`
	Nickname     string `gorm:"column:nickname;size:64"`
	// email 可空唯一（迁移 000004）：nil 落库为 NULL，避免空串占用唯一索引位
	Email  *string `gorm:"column:email;size:255;uniqueIndex"`
	Status string  `gorm:"column:status;size:16;default:enabled"`
	// HomeApplicationId 所属应用（数据边界 ADR 0006）：单个、创建必填、隐式准入
	// 且无法取消；非 admin 仅可查看/操作该应用的数据；登录默认进入。
	// 0 = 未设置（存量用户 fail-closed，admin 可补设）。
	HomeApplicationId int64     `gorm:"column:home_application_id"`
	Version           int64     `gorm:"column:version"`
	CreatedBy         string    `gorm:"column:created_by;size:64"`
	UpdatedBy         string    `gorm:"column:updated_by;size:64"`
	CreateTime        time.Time `gorm:"column:create_time"`
	UpdateTime        time.Time `gorm:"column:update_time"`
	RoleList          []*Role   `gorm:"many2many:t_r_user_role;joinForeignKey:user_id;joinReferences:role_id"`
	// ApplicationList 用户可进入管理的应用集合（多应用准入，t_r_user_application）；
	// admin 角色旁路全部启用应用，不依赖本绑定。
	ApplicationList []*Application `gorm:"many2many:t_r_user_application;joinForeignKey:user_id;joinReferences:application_id"`
}

func (u *User) TableName() string {
	return "t_user"
}

// EmailString 返回用户邮箱（可空语义：NULL 视为空串）。
func (u *User) EmailString() string {
	if u.Email == nil {
		return ""
	}
	return *u.Email
}

var UserFields = []string{
	UserColumnId, UserColumnUsername, UserColumnPasswordHash, UserColumnNickname,
	UserColumnEmail, UserColumnStatus, UserColumnHomeApplicationId, UserColumnVersion,
	UserColumnCreatedBy, UserColumnUpdatedBy, UserColumnCreateTime, UserColumnUpdateTime,
}
