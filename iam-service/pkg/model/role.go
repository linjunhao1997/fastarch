package model

const (
	RoleRoot  = "0000"
	RoleAdmin = "1000"
)

//go:generate contrib gen_gorm_mysql_repository -m role.go
type Role struct {
	Id             int64         `gorm:"column:id;primaryKey"`
	Name           string        `gorm:"column:name"`
	Code           string        `gorm:"column:code;size:255;uniqueIndex"`
	Description    string        `gorm:"column:description"`
	ApplicationId  int64         `gorm:"column:application_id"`
	Disabled       bool          `gorm:"column:disabled"`
	Version        int64         `gorm:"column:version;default:1"`
	Application    Application   `gorm:"foreignKey:application_id;references:id"`
	PermissionList []*Permission `gorm:"many2many:t_r_role_permission;joinForeignKey:role_id;joinReferences:permission_id"`
}

type Roles []*Role

func (r *Role) TableName() string {
	return "t_role"
}

/*type RRolePermission struct {
	Id           int64 `gorm:"column:id;primaryKey"`
	RoleId       int64 `gorm:"column:role_id"`
	PermissionId int64 `gorm:"column:permission_id"`
}

func (r *RRolePermission) TableName() string {
	return "t_r_role_permission"
}*/

func init() {
	migrate(Role{})
}
