package model

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	"gorm.io/gorm"
)

// SeedAdminUsername 是种子管理员用户名（与迁移 SQL 000003 镜像）。
const SeedAdminUsername = "admin"

// adminSeedPasswordHash 是 admin/admin123 的 bcrypt 摘要（与迁移 SQL 000003 镜像）。
const adminSeedPasswordHash = "$2a$10$Fo73nnjx6c606EGLmXq0uuiZJRnbXOUqiWI9.Mn.vT4coClo79lcq"

// Seed 内置权限体系数据初始化（测试夹具）。
// 运行时的数据录入由 pkg/migration 迁移 SQL（000003_seed）承担；
// 本函数在 sqlite 内存库复现同一份种子：admin 角色、admin 用户（admin/admin123）、
// 权限码词汇表、admin 全量授权（权限 + 菜单树）。
func Seed(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		adminRole, err := seedAdminRole(tx)
		if err != nil {
			return err
		}
		if err := seedAdminUser(tx, adminRole.Id); err != nil {
			return err
		}
		if err := seedVocabularyPermissions(tx, adminRole.Id); err != nil {
			return err
		}
		return seedMenus(tx, adminRole.Id)
	})
}

func seedAdminRole(tx *gorm.DB) (*Role, error) {
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
		Id:         krand.NewID(),
		Name:       "系统管理员",
		Code:       RoleAdmin,
		Status:     StatusEnabled,
		Version:    1,
		CreatedBy:  "system",
		UpdatedBy:  "system",
		CreateTime: now,
		UpdateTime: now,
	}
	if err := tx.Create(&role).Error; err != nil {
		return nil, err
	}
	slog.Info("seed admin role", "role_id", role.Id, "code", RoleAdmin)
	return &role, nil
}

func seedAdminUser(tx *gorm.DB, adminRoleId int64) error {
	var user User
	err := tx.Where("username = ?", SeedAdminUsername).First(&user).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	now := time.Now()
	user = User{
		Id:           krand.NewID(),
		Username:     SeedAdminUsername,
		PasswordHash: adminSeedPasswordHash,
		Nickname:     "系统管理员",
		Status:       StatusEnabled,
		Version:      1,
		CreatedBy:    "system",
		UpdatedBy:    "system",
		CreateTime:   now,
		UpdateTime:   now,
	}
	if err := tx.Create(&user).Error; err != nil {
		return err
	}
	binding := RUserRole{UserId: user.Id, RoleId: adminRoleId}
	if err := tx.Create(&binding).Error; err != nil {
		return err
	}
	slog.Info("seed admin user", "user_id", user.Id, "username", SeedAdminUsername)
	return nil
}

// seedVocabularyPermissions 按权限码词汇表建权限行，并全量授予 admin 角色。
// 权限码是唯一授权词汇（011 裁决），无 kind/value，无资源绑定。
func seedVocabularyPermissions(tx *gorm.DB, adminRoleId int64) error {
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
			Id:         krand.NewID(),
			Name:       code,
			Code:       code,
			Type:       "api",
			Status:     StatusEnabled,
			Version:    1,
			CreatedBy:  "system",
			UpdatedBy:  "system",
			CreateTime: now,
			UpdateTime: now,
		})
	}
	if len(created) > 0 {
		if err := tx.CreateInBatches(created, 100).Error; err != nil {
			return err
		}
	}

	var permissions []Permission
	if err := tx.Where("code IN ?", PERMISSION_CODES).Find(&permissions).Error; err != nil {
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
	newGrants := make([]RRolePermission, 0)
	for _, permission := range permissions {
		if _, ok := grantedSet[permission.Id]; ok {
			continue
		}
		newGrants = append(newGrants, RRolePermission{RoleId: adminRoleId, PermissionId: permission.Id})
	}
	if len(newGrants) > 0 {
		if err := tx.Create(&newGrants).Error; err != nil {
			return err
		}
	}
	if len(created) > 0 || len(newGrants) > 0 {
		slog.Info("seed vocabulary permissions",
			"created", len(created), "granted_to_admin", len(newGrants))
	}
	return nil
}

// seedMenus 按菜单树种子建 t_menu 行（固定 id，与迁移 SQL 000003 镜像），并全量绑定 admin 角色。
func seedMenus(tx *gorm.DB, adminRoleId int64) error {
	now := time.Now()
	for _, item := range MENU_SEEDS {
		menu := Menu{
			Id:             item.Id,
			ParentId:       item.ParentId,
			Name:           item.Name,
			Path:           item.Path,
			Icon:           item.Icon,
			Sort:           item.Sort,
			PermissionCode: item.PermissionCode,
			Status:         StatusEnabled,
			Version:        1,
			CreatedBy:      "system",
			UpdatedBy:      "system",
			CreateTime:     now,
			UpdateTime:     now,
		}
		var count int64
		if err := tx.Model(&Menu{}).Where(MenuColumnId, item.Id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := tx.Select(MenuFields).Create(&menu).Error; err != nil {
			return err
		}
	}
	var menus []Menu
	if err := tx.Select(MenuFields).Find(&menus).Error; err != nil {
		return err
	}
	grantedSet := make(map[int64]struct{})
	var granted []int64
	if err := tx.Raw(
		"SELECT menu_id FROM t_r_role_menu WHERE role_id = ?", adminRoleId,
	).Scan(&granted).Error; err != nil {
		return err
	}
	for _, id := range granted {
		grantedSet[id] = struct{}{}
	}
	newGrants := make([]RRoleMenu, 0)
	for _, menu := range menus {
		if _, ok := grantedSet[menu.Id]; ok {
			continue
		}
		newGrants = append(newGrants, RRoleMenu{RoleId: adminRoleId, MenuId: menu.Id})
	}
	if len(newGrants) > 0 {
		if err := tx.Create(&newGrants).Error; err != nil {
			return err
		}
	}
	return nil
}

// MenuSeed 是菜单树初始清单条目（结构归代码：与迁移 SQL 000003 镜像）。
type MenuSeed struct {
	Id             int64
	ParentId       int64 // 0 表示根节点
	Name           string
	Path           string
	Icon           string
	Sort           int32
	PermissionCode string
}

// MENU_SEEDS 是菜单树初始清单（与迁移 SQL 000003 种子镜像）。
var MENU_SEEDS = []MenuSeed{
	{Id: 1001, ParentId: 0, Name: "身份访问管理", Path: "/iam", Icon: "SafetyOutlined", Sort: 0},
	{Id: 1002, ParentId: 1001, Name: "用户管理", Path: "/iam/users", Icon: "UserOutlined", Sort: 1, PermissionCode: "iam.user.list"},
	{Id: 1003, ParentId: 1001, Name: "应用管理", Path: "/iam/applications", Icon: "AppstoreOutlined", Sort: 2, PermissionCode: "iam.application.list"},
	{Id: 1004, ParentId: 1001, Name: "认证管理", Path: "/iam/authentication", Icon: "LockOutlined", Sort: 3},
	{Id: 1005, ParentId: 1001, Name: "权限管理", Path: "/iam/authorization", Icon: "ApiOutlined", Sort: 4},
	{Id: 1006, ParentId: 1001, Name: "菜单管理", Path: "/iam/menus", Icon: "MenuOutlined", Sort: 5, PermissionCode: "iam.menu.list"},
	{Id: 1007, ParentId: 1001, Name: "字典管理", Path: "/iam/dicts", Icon: "BookOutlined", Sort: 6, PermissionCode: "iam.dict.list"},
	{Id: 1008, ParentId: 1004, Name: "访问凭证", Path: "/iam/credentials", Icon: "KeyOutlined", Sort: 1, PermissionCode: "iam.access_credential.list"},
	{Id: 1009, ParentId: 1005, Name: "权限列表", Path: "/iam/authorization/permissions", Icon: "UnorderedListOutlined", Sort: 1, PermissionCode: "iam.permission.list"},
	{Id: 1010, ParentId: 1005, Name: "角色管理", Path: "/iam/authorization/roles", Icon: "TeamOutlined", Sort: 2, PermissionCode: "iam.role.list"},
	{Id: 1011, ParentId: 1005, Name: "权限订阅", Path: "/iam/authorization/subscriptions", Icon: "SubscriptionOutlined", Sort: 3, PermissionCode: "iam.subscription.list"},
}
