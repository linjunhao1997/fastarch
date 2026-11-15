package model

// RUserApplication 用户-应用准入关联表（t_r_user_application，多应用裁决 #7-#9）：
// 决定用户可进入并管理哪些应用的控制台数据；admin 角色旁路全部启用应用。
// 删除策略: 基础架构型关联表（物理删除 + 引用检查）
type RUserApplication struct {
	Id            int64 `gorm:"column:id;primaryKey"`
	UserId        int64 `gorm:"column:user_id;index"`
	ApplicationId int64 `gorm:"column:application_id"`
}

func (r *RUserApplication) TableName() string {
	return "t_r_user_application"
}

var RUserApplicationFields = []string{
	RUserApplicationColumnId, RUserApplicationColumnUserId, RUserApplicationColumnApplicationId,
}
