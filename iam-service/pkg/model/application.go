package model

import "time"

//go:generate contrib gen_gorm_mysql_repository -m application.go
type Application struct {
	Id          int64     `gorm:"column:id;primaryKey"`
	Code        string    `gorm:"column:code"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	Status      string    `gorm:"column:status"`
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
