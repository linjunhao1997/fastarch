package model

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"strconv"
	"time"
)

var Models []interface{}

func migrate(model ...interface{}) {
	Models = append(Models, model...)
}

// DedupRolePermissions 在建立联合唯一键前按最小 id 去重（幂等）。
func DedupRolePermissions(db *gorm.DB) error {
	type duplicateGroup struct {
		MinId        int64
		RoleId       int64
		PermissionId int64
	}
	var groups []duplicateGroup
	if err := db.Model(&RRolePermission{}).
		Select("MIN(id) AS min_id, role_id, permission_id").
		Group("role_id, permission_id").
		Having("COUNT(*) > 1").
		Find(&groups).Error; err != nil {
		return err
	}
	var rows []RRolePermission
	if err := db.Select(RRolePermissionFields).Find(&rows).Error; err != nil {
		return err
	}
	minId := make(map[string]int64)
	deleteIds := make([]int64, 0)
	for _, row := range rows {
		key := strconv.FormatInt(row.RoleId, 10) + ":" + strconv.FormatInt(row.PermissionId, 10)
		existing, seen := minId[key]
		if !seen {
			minId[key] = row.Id
			continue
		}
		if row.Id < existing {
			deleteIds = append(deleteIds, existing)
			minId[key] = row.Id
		} else {
			deleteIds = append(deleteIds, row.Id)
		}
	}
	if len(deleteIds) == 0 {
		return nil
	}
	return db.Where("id IN ?", deleteIds).Delete(&RRolePermission{}).Error
}

// DropLegacyForeignKeys 仅在 MySQL 方言下幂等删除存量物理外键。
func DropLegacyForeignKeys(db *gorm.DB) error {
	if _, ok := db.Dialector.(mysql.Dialector); !ok {
		return nil
	}

	type constraint struct {
		TableName      string `gorm:"column:TABLE_NAME"`
		ConstraintName string `gorm:"column:CONSTRAINT_NAME"`
	}
	var constraints []constraint
	if err := db.Raw(`
SELECT TABLE_NAME, CONSTRAINT_NAME
FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA = DATABASE()`).Scan(&constraints).Error; err != nil {
		return fmt.Errorf("query foreign keys: %w", err)
	}
	for _, item := range constraints {
		if err := dropForeignKey(db, item.TableName, item.ConstraintName); err != nil {
			return err
		}
	}
	return nil
}

func dropForeignKey(db *gorm.DB, tableName, constraintName string) error {
	statement := fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`", tableName, constraintName)
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("drop foreign key %s on %s: %w", constraintName, tableName, err)
	}
	return nil
}

// MigrateResourceTree backfills resource ownership and removes the obsolete
// global key index after application-scoped uniqueness has been migrated.
// It also adds the display_order column (default 0).
func MigrateResourceTree(db *gorm.DB) error {
	var application Application
	if err := db.Where(ApplicationColumnCode, "iam-service").
		Select(ApplicationFields).First(&application).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("query builtin application: %w", err)
	}
	if err := db.Model(&Resource{}).
		Where(ResourceColumnApplicationId, 0).
		Updates(map[string]interface{}{
			ResourceColumnApplicationId: application.Id,
			ResourceColumnUpdatedBy:     application.Id,
			ResourceColumnUpdateTime:    time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("backfill resource application ownership: %w", err)
	}
	if db.Migrator().HasIndex(&Resource{}, "idx_resource_key") {
		if err := db.Migrator().DropIndex(&Resource{}, "idx_resource_key"); err != nil {
			return fmt.Errorf("drop legacy resource key index: %w", err)
		}
	}
	if !db.Migrator().HasColumn(&Resource{}, ResourceColumnDisplayOrder) {
		if err := db.Migrator().AddColumn(&Resource{}, ResourceColumnDisplayOrder); err != nil {
			return fmt.Errorf("add display_order column: %w", err)
		}
	}
	return nil
}
