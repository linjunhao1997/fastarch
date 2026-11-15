package model

import "time"

// User 用户模型（t_user，fastarch_dev 基线 Schema）：
// username 为唯一登录名，password_hash 为 bcrypt 摘要；status 取值 enabled/disabled。
// 审计字段 created_by/updated_by 为 varchar（记录操作者账号 id 字符串）。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m user.go
type User struct {
	Id           int64     `gorm:"column:id;primaryKey"`
	Username     string    `gorm:"column:username;size:64;uniqueIndex"`
	PasswordHash string    `gorm:"column:password_hash;size:255"`
	Nickname     string    `gorm:"column:nickname;size:64"`
	Status       string    `gorm:"column:status;size:16;default:enabled"`
	Version      int64     `gorm:"column:version"`
	CreatedBy    string    `gorm:"column:created_by;size:64"`
	UpdatedBy    string    `gorm:"column:updated_by;size:64"`
	CreateTime   time.Time `gorm:"column:create_time"`
	UpdateTime   time.Time `gorm:"column:update_time"`
	RoleList     []*Role   `gorm:"many2many:t_r_user_role;joinForeignKey:user_id;joinReferences:role_id"`
}

func (u *User) TableName() string {
	return "t_user"
}

var UserFields = []string{
	UserColumnId, UserColumnUsername, UserColumnPasswordHash, UserColumnNickname,
	UserColumnStatus, UserColumnVersion, UserColumnCreatedBy, UserColumnUpdatedBy,
	UserColumnCreateTime, UserColumnUpdateTime,
}
