package model

import "time"

const (
	AccountKindIdUser   = "0"
	AccountKindIdSystem = "1"
)

//go:generate contrib gen_gorm_mysql_repository -m account.go
type Account struct {
	Id         int64     `gorm:"column:id;primaryKey"`
	Code       string    `gorm:"column:code"`
	Name       string    `gorm:"column:name"`
	Email      string    `gorm:"column:email"`
	Status     string    `gorm:"column:status"`
	Salt       string    `gorm:"column:salt"`
	Password   string    `gorm:"column:password"`
	AK         string    `gorm:"column:ak"`
	SK         string    `gorm:"column:sk"`
	Kind       string    `gorm:"column:kind"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateTime time.Time `gorm:"column:update_time"`
	Version    int64     `gorm:"column:version"`
	RoleList   []*Role   `gorm:"many2many:t_r_account_role;joinForeignKey:account_id;joinReferences:role_id"`
}

func (a *Account) TableName() string {
	return "t_account"
}

type RAccountRole struct {
	Id        int64 `gorm:"column:id;primaryKey"`
	AccountId int64 `gorm:"column:account_id"`
	RoleId    int64 `gorm:"column:role_id"`
}

func (a *RAccountRole) TableName() string {
	return "t_r_account_role"
}

func init() {
	migrate(Account{}, RAccountRole{})
}
