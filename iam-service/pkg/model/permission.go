package model

//go:generate contrib gen_gorm_mysql_repository -m permission.go
type Permission struct {
	Id            int64       `gorm:"column:id;primaryKey"`
	Name          string      `gorm:"column:name"`
	kind          string      `gorm:"column:kind"`
	Code          string      `gorm:"column:code"`
	Value         string      `gorm:"column:value"`
	Description   string      `gorm:"column:description"`
	ApplicationId int64       `gorm:"column:application_id"`
	Disabled      bool        `gorm:"column:disabled"`
	Application   Application `gorm:"foreignKey:application_id;references:id"`
	RoleList      []*Role     `gorm:"many2many:t_r_role_permission;joinForeignKey:permission_id;joinReferences:role_id"`
}

func (a *Permission) TableName() string {
	return "t_permission"
}

func init() {
	migrate(Permission{})
}
