// Package migration 以 golang-migrate/migrate 管理 home 自持库的表创建与种子。
// 迁移 SQL 内嵌进二进制，服务启动时按版本顺序执行，失败 fail-fast。
package migration

import (
	"embed"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Run 执行 home_dev 库的迁移。
// 单条 SQL 文件内含多条语句（含种子），必须开 multiStatements。
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
