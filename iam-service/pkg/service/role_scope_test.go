package service

import (
	"context"
	"errors"
	"testing"
	"time"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"
)

// TestRoleHomeApplicationIsolation 验证角色归属应用成为数据边界（推翻 ADR 0006 的「角色全局」）：
// 非 admin 只能列出/查看/修改/删除 home 一致的角色，创建时强制归属 home；
// home 未设置 fail-closed；admin 旁路。
func TestRoleHomeApplicationIsolation(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
	const otherAppId = int64(901)
	if err := db.Create(&model.Application{
		Id: otherAppId, AppId: "901", Code: "sender", Name: "sender", Status: model.StatusEnabled,
		Version: 1, CreateTime: time.Now(), UpdateTime: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create other application: %v", err)
	}

	for _, spec := range []struct{ id, home int64 }{{1, testAppId}, {2, otherAppId}, {3, 0}} {
		operator := createTestUser(t, db, spec.id, "operator-"+cast.ToString(spec.id), 1)
		operator.HomeApplicationId = spec.home
		if err := db.Save(&operator).Error; err != nil {
			t.Fatalf("set operator home: %v", err)
		}
	}

	iamRole := createTestRole(t, db, 11, "iam-role", 1)
	iamRole.ApplicationId = testAppId
	if err := db.Save(&iamRole).Error; err != nil {
		t.Fatalf("set iam role app: %v", err)
	}
	otherRole := createTestRole(t, db, 12, "sender-role", 1)
	otherRole.ApplicationId = otherAppId
	if err := db.Save(&otherRole).Error; err != nil {
		t.Fatalf("set sender role app: %v", err)
	}

	svc := NewAuthorizationService(db, nil)
	ctxIam := auth.WithAccountId(context.Background(), 1)
	ctxSender := auth.WithAccountId(context.Background(), 2)
	ctxNoHome := auth.WithAccountId(context.Background(), 3)
	page := &iam_v1.ListRolesRequest{CurrentPage: 1, PageSize: 10}

	roleIds := func(ctx context.Context) map[int64]bool {
		t.Helper()
		resp, err := svc.ListRoles(ctx, page)
		if err != nil {
			t.Fatalf("ListRoles() error = %v", err)
		}
		ids := make(map[int64]bool, len(resp.Roles))
		for _, r := range resp.Roles {
			ids[cast.ToInt64(r.Id)] = true
		}
		return ids
	}

	if ids := roleIds(ctxIam); !ids[iamRole.Id] || ids[otherRole.Id] {
		t.Fatalf("iam operator roles = %v, want only iam role", ids)
	}
	if ids := roleIds(ctxSender); !ids[otherRole.Id] || ids[iamRole.Id] {
		t.Fatalf("sender operator roles = %v, want only sender role", ids)
	}
	if ids := roleIds(ctxNoHome); len(ids) != 0 {
		t.Fatalf("no-home operator roles = %v, want empty", ids)
	}
	if ids := roleIds(auth.WithAdmin(auth.WithAccountId(context.Background(), 99))); !ids[iamRole.Id] || !ids[otherRole.Id] {
		t.Fatalf("admin roles = %v, want both", ids)
	}

	if _, err := svc.GetRole(ctxIam, &common_v1.GetRequest{Id: otherRole.Id}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("GetRole(cross-app) error = %v, want %v", err, ErrRoleOutOfScope)
	}
	if _, err := svc.UpdateRole(ctxIam, &iam_v1.UpdateRoleRequest{Id: otherRole.Id, Name: "hacked", Version: 1}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("UpdateRole(cross-app) error = %v, want %v", err, ErrRoleOutOfScope)
	}
	if _, err := svc.DeleteRole(ctxIam, &iam_v1.DeleteRoleRequest{Id: otherRole.Id}); !errors.Is(err, ErrRoleOutOfScope) {
		t.Fatalf("DeleteRole(cross-app) error = %v, want %v", err, ErrRoleOutOfScope)
	}

	created, err := svc.CreateRole(ctxIam, &iam_v1.CreateRoleRequest{
		Name: "new", RoleCode: "new-role", ApplicationId: otherAppId,
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if created.ApplicationId != testAppId {
		t.Fatalf("created role applicationId = %d, want %d (operator home)", created.ApplicationId, testAppId)
	}
	if _, err := svc.CreateRole(ctxNoHome, &iam_v1.CreateRoleRequest{Name: "x", RoleCode: "x-role"}); !errors.Is(err, ErrHomeApplicationRequired) {
		t.Fatalf("CreateRole(no home) error = %v, want %v", err, ErrHomeApplicationRequired)
	}
}
