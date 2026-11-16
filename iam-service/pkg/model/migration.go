package model

import (
	contribmodel "github.com/linjunhao1997/contrib/model"
	"gorm.io/gorm"
)

// Models 注册全部业务模型，仅供测试夹具（sqlite 内存库）搭建 Schema 使用。
// 运行时的表创建、字段变更与数据录入由 pkg/migration 的 golang-migrate
// 迁移 SQL 承担，禁止在服务启动路径调用 AutoMigrate。

var Models []interface{}

func migrate(model ...interface{}) {
	Models = append(Models, model...)
}

// init 统一注册全部业务模型（sqlite 测试夹具搭建 Schema 用）。
// 关联表必须先于携带 many2many 的主表注册：GORM 为 many2many 自动建表时
// 只有外键列没有 id 列，若先建了缺 id 的关联表，后续按结构体迁移无法补回
// 主键列（sqlite 限制），插入关联行时将报 no such column: id。
func init() {
	migrate(
		// 关联表先行
		RUserRole{},
		RRolePermission{},
		RUserApplication{},
		// 主表（菜单/字典模型来自 contrib，ADR 0001）
		User{},
		Role{},
		Permission{},
		contribmodel.Menu{},
		contribmodel.DictType{},
		contribmodel.DictItem{},
		Application{},
		AccessCredential{},
	)
}

// AutoMigrateModels 逐个迁移全部业务模型（仅供 sqlite 测试夹具搭建 Schema）。
// 不能一次 AutoMigrate(Models...)：GORM 批量迁移会按 many2many 关联统一重建
// 关联表（只有外键列、没有 id 列），导致按关联表结构体 CRUD 报 no such column: id。
func AutoMigrateModels(db *gorm.DB) error {
	for _, model := range Models {
		if err := db.AutoMigrate(model); err != nil {
			return err
		}
	}
	return nil
}

// 通用启用/禁用状态取值（011 模型统一 status 字段：enabled/disabled）。
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
)
