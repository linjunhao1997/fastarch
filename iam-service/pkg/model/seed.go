package model

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	"gorm.io/gorm"

	"iam-service/pkg/constant"
)

// Seed 内置权限体系数据初始化（测试夹具）。
// 运行时的数据录入由 pkg/migration 迁移 SQL（000005_seed_permission_codes）承担；
// 本函数在 sqlite 内存库复现同一份种子：内置应用、管理员角色、
// 权限码词汇表、管理员全量授权、前端组件资源树。
func Seed(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		applicationId, err := seedApplication(tx)
		if err != nil {
			return err
		}
		adminRole, err := seedAdminRole(tx, applicationId)
		if err != nil {
			return err
		}
		if err := seedVocabularyPermissions(tx, applicationId, adminRole.Id); err != nil {
			return err
		}
		if err := seedComponents(tx, applicationId); err != nil {
			return err
		}
		return seedAccountApplications(tx)
	})
}

func seedApplication(tx *gorm.DB) (int64, error) {
	var existing int64
	if err := tx.Model(&Application{}).Count(&existing).Error; err != nil {
		return 0, err
	}
	if existing > 0 {
		var app Application
		if err := tx.First(&app).Error; err != nil {
			return 0, err
		}
		return app.Id, nil
	}
	pkId := krand.NewID()
	now := time.Now()
	app := Application{
		Id:         pkId,
		Code:       constant.ApplicationCode,
		Name:       "IAM",
		Disabled:   0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := tx.Create(&app).Error; err != nil {
		return 0, err
	}
	slog.Info("seed application", "id", app.Id, "code", app.Code)
	return app.Id, nil
}

func seedAdminRole(tx *gorm.DB, applicationId int64) (*Role, error) {
	var existing Role
	err := tx.Where("code = ?", RoleAdmin).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	now := time.Now()
	role := Role{
		Id:            krand.NewID(),
		Name:          "Administrator",
		Code:          RoleAdmin,
		ApplicationId: applicationId,
		Disabled:      false,
		CreateTime:    now,
		UpdateTime:    now,
		Version:       1,
	}
	if err := tx.Create(&role).Error; err != nil {
		return nil, err
	}
	slog.Info("seed admin role", "role_id", role.Id, "code", RoleAdmin)
	return &role, nil
}

// seedVocabularyPermissions 按权限码词汇表建权限行，并全量授予管理员角色。
// 权限码是唯一授权词汇（011 裁决），无 kind/value，无资源绑定。
func seedVocabularyPermissions(tx *gorm.DB, applicationId, adminRoleId int64) error {
	now := time.Now()
	var existing []Permission
	if err := tx.Where("code IN ?", PERMISSION_CODES).Find(&existing).Error; err != nil {
		return err
	}
	existingByCode := make(map[string]Permission, len(existing))
	for _, row := range existing {
		existingByCode[row.Code] = row
	}

	created := make([]Permission, 0, len(PERMISSION_CODES))
	for _, code := range PERMISSION_CODES {
		if _, ok := existingByCode[code]; ok {
			continue
		}
		created = append(created, Permission{
			Id:            krand.NewID(),
			Name:          code,
			Code:          code,
			ApplicationId: applicationId,
			Disabled:      false,
			CreateTime:    now,
			UpdateTime:    now,
			Version:       1,
		})
	}
	if len(created) > 0 {
		if err := tx.CreateInBatches(created, 100).Error; err != nil {
			return err
		}
	}

	codes := make([]string, 0, len(PERMISSION_CODES))
	codes = append(codes, PERMISSION_CODES...)
	var permissions []Permission
	if err := tx.Where("code IN ?", codes).Find(&permissions).Error; err != nil {
		return err
	}
	grantedSet := make(map[int64]struct{})
	var granted []int64
	if err := tx.Raw(
		"SELECT permission_id FROM t_r_role_permission WHERE role_id = ?", adminRoleId,
	).Scan(&granted).Error; err != nil {
		return err
	}
	for _, id := range granted {
		grantedSet[id] = struct{}{}
	}
	newGrants := make([]map[string]interface{}, 0)
	for _, permission := range permissions {
		if _, ok := grantedSet[permission.Id]; ok {
			continue
		}
		newGrants = append(newGrants, map[string]interface{}{
			"role_id":       adminRoleId,
			"permission_id": permission.Id,
		})
	}
	if len(newGrants) > 0 {
		if err := tx.Table("t_r_role_permission").Create(&newGrants).Error; err != nil {
			return err
		}
	}
	if len(created) > 0 || len(newGrants) > 0 {
		slog.Info("seed vocabulary permissions",
			"created", len(created), "granted_to_admin", len(newGrants))
	}
	return nil
}

// seedComponents 按组件种子清单建 t_resource 行（固定 id，与迁移 SQL 镜像）。
func seedComponents(tx *gorm.DB, applicationId int64) error {
	now := time.Now()
	for _, item := range COMPONENT_SEEDS {
		resource := Resource{
			Id:              item.Id,
			ApplicationId:   applicationId,
			Kind:            item.Kind,
			Key:             item.Key,
			Name:            item.Name,
			PermissionCodes: item.PermissionCodes,
			DisplayOrder:    item.Order,
			CreateTime:      now,
			UpdateTime:      now,
			Version:         1,
		}
		if item.ParentId > 0 {
			parentId := item.ParentId
			resource.ParentId = &parentId
		}
		var count int64
		if err := tx.Model(&Resource{}).Where(ResourceColumnId, item.Id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := tx.Select(ResourceFields).Create(&resource).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedAccountApplications(tx *gorm.DB) error {
	var pairs []RAccountApplication
	err := tx.Table("t_r_account_role AS ar").
		Select("DISTINCT ar.account_id AS account_id, r.application_id AS application_id").
		Joins("JOIN t_role AS r ON r.id = ar.role_id AND r.disabled = ?", false).
		Scan(&pairs).Error
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return nil
	}

	var existing []RAccountApplication
	if err := tx.Select(RAccountApplicationFields).Find(&existing).Error; err != nil {
		return err
	}
	existingSet := make(map[string]struct{}, len(existing))
	for _, row := range existing {
		existingSet[permResourceKey(row.AccountId, row.ApplicationId)] = struct{}{}
	}

	missing := make([]RAccountApplication, 0, len(pairs))
	for _, pair := range pairs {
		if pair.ApplicationId == 0 || pair.AccountId == 0 {
			continue
		}
		if _, ok := existingSet[permResourceKey(pair.AccountId, pair.ApplicationId)]; ok {
			continue
		}
		existingSet[permResourceKey(pair.AccountId, pair.ApplicationId)] = struct{}{}
		missing = append(missing, RAccountApplication{
			AccountId:     pair.AccountId,
			ApplicationId: pair.ApplicationId,
		})
	}
	if len(missing) == 0 {
		return nil
	}
	if err := tx.Create(&missing).Error; err != nil {
		return err
	}
	slog.Info("seed account applications from roles",
		"initialized", len(missing))
	return nil
}

func permResourceKey(accountId, applicationId int64) string {
	return strconv.FormatInt(accountId, 10) + ":" + strconv.FormatInt(applicationId, 10)
}
