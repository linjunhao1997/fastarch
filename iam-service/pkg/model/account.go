package model

import "time"

const (
	AccountKindIdUser   = "0"
	AccountKindIdSystem = "1"
)

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
//go:generate contrib gen_gorm_mysql_repository -m account.go
type Account struct {
	Id         int64     `gorm:"column:id;primaryKey"`
	Code       string    `gorm:"column:code"`
	Name       string    `gorm:"column:name"`
	Email      string    `gorm:"column:email;size:255;uniqueIndex"`
	Status     string    `gorm:"column:status"`
	Salt       string    `gorm:"column:salt"`
	Password   string    `gorm:"column:password"`
	AK         string    `gorm:"column:ak"`
	SK         string    `gorm:"column:sk"`
	Kind       string    `gorm:"column:kind"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateTime time.Time `gorm:"column:update_time"`
	CreatedBy  int64     `gorm:"column:created_by"`
	UpdatedBy  int64     `gorm:"column:updated_by"`
	Version    int64     `gorm:"column:version"`
	RoleList   []*Role   `gorm:"many2many:t_r_account_role;joinForeignKey:account_id;joinReferences:role_id"`
	// ApplicationList 账号归属的应用列表（specs/008，显式维护，不随角色推导）
	ApplicationList []*Application `gorm:"many2many:t_r_account_application;joinForeignKey:account_id;joinReferences:application_id"`
}

func (a *Account) TableName() string {
	return "t_account"
}

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type RAccountRole struct {
	Id        int64 `gorm:"column:id;primaryKey"`
	AccountId int64 `gorm:"column:account_id;uniqueIndex:uk_r_account_role_account_role"`
	RoleId    int64 `gorm:"column:role_id;uniqueIndex:uk_r_account_role_account_role"`
}

func (a *RAccountRole) TableName() string {
	return "t_r_account_role"
}

// RAccountApplication 账号-应用多对多归属（specs/008-account-app-context）。
// 显式维护：由管理员在账号创建/编辑时直接分配，不从角色推导（research.md D1）。
// 删除策略: 基础架构型关联表（与 RAccountRole 一致）：无独立审计字段，
// 归属变更审计记录在账号操作日志中。
type RAccountApplication struct {
	Id            int64 `gorm:"column:id;primaryKey"`
	AccountId     int64 `gorm:"column:account_id;uniqueIndex:uk_r_account_application_account_application;index"`
	ApplicationId int64 `gorm:"column:application_id;uniqueIndex:uk_r_account_application_account_application;index"`
}

func (a *RAccountApplication) TableName() string {
	return "t_r_account_application"
}

func init() {
	migrate(Account{}, RAccountRole{}, RAccountApplication{})
}

var AccountFields = []string{
	AccountColumnId, AccountColumnCode, AccountColumnName, AccountColumnEmail,
	AccountColumnStatus, AccountColumnSalt, AccountColumnPassword, AccountColumnAK,
	AccountColumnSK, AccountColumnKind, AccountColumnCreateTime, AccountColumnUpdateTime,
	AccountColumnCreatedBy, AccountColumnUpdatedBy, AccountColumnVersion,
}

var RAccountRoleFields = []string{
	RAccountRoleColumnId, RAccountRoleColumnAccountId, RAccountRoleColumnRoleId,
}

var RAccountApplicationFields = []string{
	RAccountApplicationColumnId, RAccountApplicationColumnAccountId, RAccountApplicationColumnApplicationId,
}
