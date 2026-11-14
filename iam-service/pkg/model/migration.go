package model

// Models 注册全部业务模型，仅供测试夹具（sqlite 内存库）搭建 Schema 使用。
// 运行时的表创建、字段变更与数据录入由 pkg/migration 的 golang-migrate
// 迁移 SQL 承担，禁止在服务启动路径调用 AutoMigrate。

var Models []interface{}

func migrate(model ...interface{}) {
	Models = append(Models, model...)
}
