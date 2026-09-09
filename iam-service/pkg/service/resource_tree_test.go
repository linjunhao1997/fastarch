package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"iam-service/pkg/model"
)

func setupResourceTreeTestDB(t *testing.T) (*gorm.DB, *AuthorizationService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{}, &model.Role{}, &model.Permission{},
		&model.RRolePermission{}, &model.Resource{}, &model.PermissionResource{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db, &AuthorizationService{db: db}
}

func createResourceTestApplication(t *testing.T, db *gorm.DB, id int64, code string) model.Application {
	t.Helper()
	application := model.Application{Id: id, Code: code, Name: code + " Application", Disabled: 0, Description: "test application"}
	if err := db.Create(&application).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	return application
}

func TestResourceOwnershipHierarchyAndSafety(t *testing.T) {
	db, service := setupResourceTreeTestDB(t)
	ctx := context.Background()
	createResourceTestApplication(t, db, 1, "iam")

	if _, err := service.CreateResource(ctx, &iam_v1.CreateResourceRequest{
		Kind: model.PermissionKindMenu, Key: "root", Name: "Root",
	}); err != ErrResourceApplicationRequired {
		t.Fatalf("CreateResource() error = %v, want %v", err, ErrResourceApplicationRequired)
	}
	root, err := service.CreateResource(ctx, &iam_v1.CreateResourceRequest{
		ApplicationId: 1, Kind: model.PermissionKindMenu, Key: "account", Name: "账号管理",
	})
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	rootId := castResourceID(root.Id)
	child, err := service.CreateResource(ctx, &iam_v1.CreateResourceRequest{
		ApplicationId: 1, ParentId: rootId, Kind: model.PermissionKindMenu, Key: "account.list", Name: "用户列表",
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	childId := castResourceID(child.Id)
	grandchild, err := service.CreateResource(ctx, &iam_v1.CreateResourceRequest{
		ApplicationId: 1, ParentId: childId, Kind: model.PermissionKindHTTPAPI, Key: "GET /account/list", Name: "用户列表接口",
	})
	if err != nil {
		t.Fatalf("create grandchild: %v", err)
	}
	grandchildId := castResourceID(grandchild.Id)

	if _, err := service.CreateResource(ctx, &iam_v1.CreateResourceRequest{
		ApplicationId: 1, Kind: model.PermissionKindMenu, Key: "account", Name: "重复资源",
	}); err != ErrResourceKeyDuplicated {
		t.Fatalf("duplicate key error = %v, want %v", err, ErrResourceKeyDuplicated)
	}
	if _, err := service.UpdateResource(ctx, &iam_v1.UpdateResourceRequest{
		Id: rootId, ParentId: grandchildId, Name: root.Name, Version: root.Version,
	}); err != ErrResourceHierarchyCircular {
		t.Fatalf("circular hierarchy error = %v, want %v", err, ErrResourceHierarchyCircular)
	}
	if _, err := service.DeleteResource(ctx, &iam_v1.DeleteResourceRequest{Id: rootId}); err != ErrResourceHasChildren {
		t.Fatalf("delete root error = %v, want %v", err, ErrResourceHasChildren)
	}

	tree, err := service.ListResourceTree(ctx, &iam_v1.ListResourceTreeRequest{ApplicationId: 1})
	if err != nil {
		t.Fatalf("ListResourceTree() error = %v", err)
	}
	if len(tree.Roots) != 1 || tree.Roots[0].Resource.Id != root.Id || len(tree.Roots[0].Children) != 1 {
		t.Fatalf("tree roots/children = %+v, want one root with one child", tree.Roots)
	}
	if tree.Roots[0].Resource.Application == nil || tree.Roots[0].Resource.Application.Name != "iam Application" {
		t.Fatalf("resource application summary = %+v", tree.Roots[0].Resource.Application)
	}

	permission, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{
		ApplicationId: 1, Name: "用户列表", Code: "account.list", Kind: model.PermissionKindMenu,
		ResourceIdList: []int64{grandchildId, childId},
	})
	if err != nil {
		t.Fatalf("create permission with mixed hierarchy: %v", err)
	}
	if len(permission.ResourceList) != 3 {
		t.Fatalf("selected resource count = %d, want root, child, and grandchild", len(permission.ResourceList))
	}
	if _, err := service.DeleteResource(ctx, &iam_v1.DeleteResourceRequest{Id: grandchildId}); err != ErrResourceReferenced {
		t.Fatalf("delete bound resource error = %v, want %v", err, ErrResourceReferenced)
	}
}

func TestRoleCreationAndListSummaries(t *testing.T) {
	db, service := setupResourceTreeTestDB(t)
	ctx := context.Background()
	createResourceTestApplication(t, db, 1, "iam")
	resource := model.Resource{Id: 100, ApplicationId: 1, Kind: model.PermissionKindMenu, Key: "account.list", Name: "用户列表"}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatalf("create resource: %v", err)
	}

	role, err := service.CreateRole(ctx, &iam_v1.CreateRoleRequest{ApplicationId: 1, Name: "管理员", RoleCode: "admin"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if len(role.PermissionList) != 0 {
		t.Fatalf("new role permissions = %d, want 0", len(role.PermissionList))
	}
	permission, err := service.CreatePermission(ctx, &iam_v1.CreatePermissionRequest{
		ApplicationId: 1, Name: "用户列表", Code: "account.list", Kind: model.PermissionKindMenu,
		ResourceIdList: []int64{resource.Id},
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	if _, err := service.UpdateRole(ctx, &iam_v1.UpdateRoleRequest{
		Id: castResourceID(role.Id), Version: role.Version,
		AddedPermissionCodeList: []string{permission.PermissionCode},
	}); err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}

	roles, err := service.ListRoles(ctx, &iam_v1.ListRolesRequest{})
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles.Roles) != 1 || roles.Roles[0].Application == nil {
		t.Fatalf("role response = %+v, want one role with application summary", roles.Roles)
	}
	if roles.Roles[0].Application.Name != "iam Application" {
		t.Fatalf("application name = %q", roles.Roles[0].Application.Name)
	}
	if len(roles.Roles[0].PermissionList) != 1 || roles.Roles[0].PermissionList[0].Name != "用户列表" {
		t.Fatalf("permission summaries = %+v", roles.Roles[0].PermissionList)
	}
}

func castResourceID(value string) int64 {
	return cast.ToInt64(value)
}
