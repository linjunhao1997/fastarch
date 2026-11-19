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

// TestPermissionDataScopeIsolation 验证权限管理纳入数据范围边界：
// 非 admin 即使登录在 IAM 控制台，权限列表也只能看到数据范围应用的码；
// 跨应用增删改拒绝；创建码强制归属数据范围应用；数据范围未注入 fail-closed。
// admin/系统调用继续按宿主应用上下文管理。
func TestPermissionDataScopeIsolation(t *testing.T) {
	db := setupAuthorizationTestDB(t)
	setupTestApplication(t, db)
	const otherAppId = int64(901)
	if err := db.Create(&model.Application{
		Id: otherAppId, AppId: "901", Code: "sender", Name: "sender", Status: model.StatusEnabled,
		Version: 1, CreateTime: time.Now(), UpdateTime: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create other application: %v", err)
	}

	for _, spec := range []struct{ id, scope int64 }{{1, testAppId}, {2, otherAppId}, {3, 0}} {
		createTestUser(t, db, spec.id, "permission-operator-"+cast.ToString(spec.id), 1)
	}

	iamPermission := createTestPermission(t, db, 10, "iam.alpha.code", 1)
	senderPermission := model.Permission{
		Id: 11, Code: "sender.beta.code", Name: "permission", ApplicationId: otherAppId,
		Version: 1, Status: model.StatusEnabled, Type: "api",
		CreateTime: time.Now(), UpdateTime: time.Now(),
	}
	if err := db.Create(&senderPermission).Error; err != nil {
		t.Fatalf("create sender permission: %v", err)
	}

	svc := NewAuthorizationService(db, nil)
	// 三个操作者都登录在 IAM 控制台：宿主 appId 是 iam，数据范围由 dataAppId 注入。
	ctxIam := auth.WithDataAppId(appCtx(auth.WithAccountId(context.Background(), 1)), testAppId)
	ctxSender := auth.WithDataAppId(appCtx(auth.WithAccountId(context.Background(), 2)), otherAppId)
	ctxNoScope := appCtx(auth.WithAccountId(context.Background(), 3))
	page := &iam_v1.ListPermissionsRequest{CurrentPage: 1, PageSize: 10}

	listCodes := func(ctx context.Context) []string {
		t.Helper()
		resp, err := svc.ListPermissions(ctx, page)
		if err != nil {
			t.Fatalf("ListPermissions() error = %v", err)
		}
		codes := make([]string, 0, len(resp.Permissions))
		for _, p := range resp.Permissions {
			codes = append(codes, p.PermissionCode)
		}
		return codes
	}

	if codes := listCodes(ctxSender); len(codes) != 1 || codes[0] != senderPermission.Code {
		t.Fatalf("sender-scope operator codes = %v, want only %q", codes, senderPermission.Code)
	}
	if codes := listCodes(ctxIam); len(codes) != 1 || codes[0] != iamPermission.Code {
		t.Fatalf("iam-scope operator codes = %v, want only %q", codes, iamPermission.Code)
	}
	if codes := listCodes(ctxNoScope); len(codes) != 0 {
		t.Fatalf("no-scope operator codes = %v, want empty", codes)
	}

	if _, err := svc.GetPermission(ctxSender, &common_v1.GetRequest{Id: iamPermission.Id}); !errors.Is(err, ErrPermissionOutOfScope) {
		t.Fatalf("GetPermission(cross-app) error = %v, want %v", err, ErrPermissionOutOfScope)
	}
	if _, err := svc.UpdatePermission(ctxSender, &iam_v1.UpdatePermissionRequest{Id: iamPermission.Id, Name: "hacked", Version: 1}); !errors.Is(err, ErrPermissionOutOfScope) {
		t.Fatalf("UpdatePermission(cross-app) error = %v, want %v", err, ErrPermissionOutOfScope)
	}
	if _, err := svc.DeletePermission(ctxSender, &iam_v1.DeletePermissionRequest{Id: iamPermission.Id}); !errors.Is(err, ErrPermissionOutOfScope) {
		t.Fatalf("DeletePermission(cross-app) error = %v, want %v", err, ErrPermissionOutOfScope)
	}

	created, err := svc.CreatePermission(ctxSender, &iam_v1.CreatePermissionRequest{
		Name: "new", Code: "sender.new.code",
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	var createdRow model.Permission
	if err := db.First(&createdRow, cast.ToInt64(created.Id)).Error; err != nil {
		t.Fatalf("load created permission: %v", err)
	}
	if createdRow.ApplicationId != otherAppId {
		t.Fatalf("created permission applicationId = %d, want %d (operator data scope)", createdRow.ApplicationId, otherAppId)
	}
	if _, err := svc.CreatePermission(ctxSender, &iam_v1.CreatePermissionRequest{Name: "x", Code: "iam.new.code"}); !errors.Is(err, ErrPermissionCodeSegmentMismatch) {
		t.Fatalf("CreatePermission(other segment) error = %v, want %v", err, ErrPermissionCodeSegmentMismatch)
	}
	if _, err := svc.CreatePermission(ctxNoScope, &iam_v1.CreatePermissionRequest{Name: "x", Code: "sender.no_scope.code"}); !errors.Is(err, ErrDataScopeRequired) {
		t.Fatalf("CreatePermission(no data scope) error = %v, want %v", err, ErrDataScopeRequired)
	}
}
