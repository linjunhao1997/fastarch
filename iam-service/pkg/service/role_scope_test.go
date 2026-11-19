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

// TestRoleDataScopeIsolation 验证角色归属应用成为数据边界：
// 非 admin 只能列出/查看/修改/删除数据范围应用一致的角色，创建时强制归属数据范围应用；
// 数据范围未注入 fail-closed；admin 旁路。
func TestRoleDataScopeIsolation(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
	const otherAppId = int64(901)
	if err := db.Create(&model.Application{
		Id: otherAppId, AppId: "901", Code: "sender", Name: "sender", Status: model.StatusEnabled,
		Version: 1, CreateTime: time.Now(), UpdateTime: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create other application: %v", err)
	}

	// 数据范围由请求头 dataAppId 注入 ctx（删除 home 后不再从用户行读取）。
	for _, spec := range []struct{ id, scope int64 }{{1, testAppId}, {2, otherAppId}, {3, 0}} {
		createTestUser(t, db, spec.id, "operator-"+cast.ToString(spec.id), 1)
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
	ctxIam := auth.WithDataAppId(auth.WithAccountId(context.Background(), 1), testAppId)
	ctxSender := auth.WithDataAppId(auth.WithAccountId(context.Background(), 2), otherAppId)
	ctxNoScope := auth.WithAccountId(context.Background(), 3)
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
	if ids := roleIds(ctxNoScope); len(ids) != 0 {
		t.Fatalf("no-scope operator roles = %v, want empty", ids)
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
		t.Fatalf("created role applicationId = %d, want %d (operator data scope)", created.ApplicationId, testAppId)
	}
	if _, err := svc.CreateRole(ctxNoScope, &iam_v1.CreateRoleRequest{Name: "x", RoleCode: "x-role"}); !errors.Is(err, ErrDataScopeRequired) {
		t.Fatalf("CreateRole(no data scope) error = %v, want %v", err, ErrDataScopeRequired)
	}
}
