package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"

	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

// FederationService 联邦内部契约实现（ADR 0001/0003）：供宿主服务（sender 等）
// 机器间查询 IAM 角色数据（菜单可见性过滤、角色绑定校验）。应用凭证经 metadata
// x-fa-app-id/x-fa-app-secret 携带，服务内自鉴权（鉴权拦截器白名单放行 JWT）。
type FederationService struct {
	db *gorm.DB
}

var _ iam_v1.FederationServiceServer = (*FederationService)(nil)

func NewFederationService(db *gorm.DB) *FederationService {
	return &FederationService{db: db}
}

// 应用鉴权错误（fail-closed：凭证缺失/无效/应用停用一律拒绝）。
var (
	ErrFederationAppUnauthorized = NewErrorWithCode("1001061", codes.Unauthenticated, errors.New("应用凭证无效"))
	ErrFederationAppDisabled     = NewErrorWithCode("1001062", codes.PermissionDenied, errors.New("应用已停用"))
)

// authenticateApp 校验应用凭证：app_id 查应用 → 须 enabled → sha256(secret) hex
// 常量时间比对 secret_hash。任一环节失败 fail-closed，不透出底层细节。
func (s *FederationService) authenticateApp(ctx context.Context) (*model.Application, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrFederationAppUnauthorized
	}
	appId := firstMetadata(md, constant.MetadataAppId)
	secret := firstMetadata(md, constant.MetadataAppSecret)
	if appId == "" || secret == "" {
		return nil, ErrFederationAppUnauthorized
	}
	var app model.Application
	err := s.db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnAppId, appId).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFederationAppUnauthorized
		}
		slog.Error("federation lookup application failed", "app_id", appId, "error", err)
		return nil, err
	}
	if app.Status != constant.ApplicationStatusEnabled {
		return nil, ErrFederationAppDisabled
	}
	digest := sha256.Sum256([]byte(secret))
	hash := hex.EncodeToString(digest[:])
	if subtle.ConstantTimeCompare([]byte(hash), []byte(app.SecretHash)) != 1 {
		return nil, ErrFederationAppUnauthorized
	}
	return &app, nil
}

func firstMetadata(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// GetAccountRoleIds 查询账号绑定的角色 id 集合（宿主侧边栏按角色过滤菜单用）。
func (s *FederationService) GetAccountRoleIds(ctx context.Context, request *iam_v1.GetAccountRoleIdsRequest) (*iam_v1.GetAccountRoleIdsResponse, error) {
	if _, err := s.authenticateApp(ctx); err != nil {
		return nil, err
	}
	var roleIds []int64
	err := s.db.WithContext(ctx).Model(model.RUserRole{}).
		Where(model.RUserRoleColumnUserId, request.UserId).
		Pluck(model.RUserRoleColumnRoleId, &roleIds).Error
	if err != nil {
		slog.Warn("federation get account role ids failed", "user_id", request.UserId, "error", err)
		return nil, err
	}
	ids := make([]string, 0, len(roleIds))
	for _, id := range roleIds {
		ids = append(ids, cast.ToString(id))
	}
	return &iam_v1.GetAccountRoleIdsResponse{RoleIds: ids}, nil
}

// RoleExists 校验角色存在（宿主菜单绑定前校验用）。
func (s *FederationService) RoleExists(ctx context.Context, request *iam_v1.RoleExistsRequest) (*iam_v1.RoleExistsResponse, error) {
	if _, err := s.authenticateApp(ctx); err != nil {
		return nil, err
	}
	var count int64
	err := s.db.WithContext(ctx).Model(model.Role{}).
		Where(model.RoleColumnId, request.RoleId).
		Count(&count).Error
	if err != nil {
		slog.Warn("federation role exists failed", "role_id", request.RoleId, "error", err)
		return nil, err
	}
	return &iam_v1.RoleExistsResponse{Exists: count > 0}, nil
}
