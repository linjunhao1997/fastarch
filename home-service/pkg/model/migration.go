package model

import "gorm.io/gorm"

// AutoMigrateModels 仅用于测试/本地 sqlite 夹具；生产表由内嵌迁移管理。
func AutoMigrateModels(db *gorm.DB) error {
	return db.AutoMigrate(&Homepage{})
}
