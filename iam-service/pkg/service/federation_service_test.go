package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

// FederationService 回归：应用凭证鉴权 fail-closed（缺失/错 secret/未知应用/停用），
// 角色查询正确。机器间契约，凭证经 metadata 携带。

func federationCtx(appId, secret string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.MetadataAppId, appId,
		constant.MetadataAppSecret, secret,
	))
}

func setupFederationEnv(t *testing.T) (*FederationService, *gorm.DB) {
	t.Helper()
	db := setupAuthorizationTestDB(t)
	now := time.Now()
	digest := sha256.Sum256([]byte("test-secret"))
	app := model.Application{
		Id:         2,
		AppId:      "sender-test",
		Code:       "sender",
		Name:       "消息中心",
		SecretHash: hex.EncodeToString(digest[:]),
		Status:     model.StatusEnabled,
		Version:    1,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	return NewFederationService(db), db
}

func TestFederationGetAccountRoleIds(t *testing.T) {
	svc, db := setupFederationEnv(t)
	createTestRole(t, db, 7, "role-seven", 1)
	createTestUser(t, db, 5, "scoped-user", 1)
	if err := db.Create(&model.RUserRole{UserId: 5, RoleId: 7}).Error; err != nil {
		t.Fatalf("bind user role: %v", err)
	}

	resp, err := svc.GetAccountRoleIds(federationCtx("sender-test", "test-secret"), &iam_v1.GetAccountRoleIdsRequest{UserId: 5})
	if err != nil {
		t.Fatalf("GetAccountRoleIds() error = %v", err)
	}
	if len(resp.RoleIds) != 1 || resp.RoleIds[0] != "7" {
		t.Fatalf("RoleIds = %v, want [7]", resp.RoleIds)
	}
}

func TestFederationRoleExists(t *testing.T) {
	svc, db := setupFederationEnv(t)
	createTestRole(t, db, 7, "role-seven", 1)
	ctx := federationCtx("sender-test", "test-secret")

	resp, err := svc.RoleExists(ctx, &iam_v1.RoleExistsRequest{RoleId: 7})
	if err != nil || !resp.Exists {
		t.Fatalf("RoleExists(7) = %v, %v; want true, nil", resp, err)
	}
	resp, err = svc.RoleExists(ctx, &iam_v1.RoleExistsRequest{RoleId: 999})
	if err != nil || resp.Exists {
		t.Fatalf("RoleExists(999) = %v, %v; want false, nil", resp, err)
	}
}

func TestFederationRejectsInvalidCredentials(t *testing.T) {
	svc, db := setupFederationEnv(t)
	createTestRole(t, db, 7, "role-seven", 1)

	cases := []struct {
		name string
		ctx  context.Context
	}{
		{"missing metadata", context.Background()},
		{"wrong secret", federationCtx("sender-test", "wrong-secret")},
		{"unknown app", federationCtx("nope", "test-secret")},
		{"empty secret", federationCtx("sender-test", "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.RoleExists(tc.ctx, &iam_v1.RoleExistsRequest{RoleId: 7})
			if !errors.Is(err, ErrFederationAppUnauthorized) {
				t.Fatalf("RoleExists() error = %v, want ErrFederationAppUnauthorized", err)
			}
		})
	}
}

func TestFederationRejectsDisabledApp(t *testing.T) {
	svc, db := setupFederationEnv(t)
	if err := db.Model(model.Application{}).Where(model.ApplicationColumnId, 2).
		Update(model.ApplicationColumnStatus, model.StatusDisabled).Error; err != nil {
		t.Fatalf("disable app: %v", err)
	}
	_, err := svc.RoleExists(federationCtx("sender-test", "test-secret"), &iam_v1.RoleExistsRequest{RoleId: 7})
	if !errors.Is(err, ErrFederationAppDisabled) {
		t.Fatalf("RoleExists() error = %v, want ErrFederationAppDisabled", err)
	}
}
