// Package migration 以 golang-migrate/migrate 管理表创建、字段变更与数据录入。
// 迁移 SQL 内嵌进二进制，服务启动时按版本号顺序执行至最新版本，
// 失败 fail-fast，禁止以未迁移的 Schema 运行。
package migration

import (
	"embed"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql" // 注册 mysql 方言
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrations 暴露内嵌迁移 FS，供外部按版本号定向迁移（如集成测试先迁到中间版本
// 构造存量库再升级）。
var Migrations = migrationFS

// Run 对 DSN 指向的数据库应用全部未执行的迁移（已最新则无操作）。
// multiStatements 允许单迁移文件多条语句（含存储过程条件清理脚本），
// x-multi-statement 使 mysql 驱动以整文件方式执行迁移内容。
func Run(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["multiStatements"] = "true"
	cfg.Params["x-multi-statement"] = "true"

	source, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, "mysql://"+cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
