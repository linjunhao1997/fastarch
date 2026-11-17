package model

import (
	contribmodel "github.com/linjunhao1997/contrib/model"
	"gorm.io/gorm"
)

// Models 注册全部业务模型，仅供测试夹具（sqlite 内存库）搭建 Schema 使用。
// 运行时的表创建/变更/种子由 pkg/migration 的 golang-migrate 迁移 SQL 承担，
// 禁止在服务启动路径调用 AutoMigrate。
var Models []interface{}

func migrate(model ...interface{}) {
	Models = append(Models, model...)
}

func init() {
	migrate(
		// 菜单/字典模型来自 contrib（ADR 0001，每服务自持表）
		contribmodel.Menu{},
		contribmodel.DictType{},
		contribmodel.DictItem{},
		MessageRecord{},
		MessageTemplate{},
	)
}

// AutoMigrateModels 逐个迁移全部业务模型（仅供 sqlite 测试夹具）。
func AutoMigrateModels(db *gorm.DB) error {
	for _, m := range Models {
		if err := db.AutoMigrate(m); err != nil {
			return err
		}
	}
	return nil
}

const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
)
