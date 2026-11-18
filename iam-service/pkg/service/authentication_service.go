package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	"github.com/go-dev-frame/sponge/pkg/krand"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// defaultApplicationCode 是人机登录未携带 AK/SK 时的回退值：IAM 自有控制台。
const defaultApplicationCode = "iam"

// resolveLoginApplicationCode 解析登录目标应用 code（ADR 0007 补充裁决）：
//   - 入站携带 AK/SK：验签通过后以机机账号所属应用为准，调用方自报的
//     application_code 一律忽略（应用身份不能由调用方声明）；
//   - 未携带 AK/SK：IAM 自有控制台登录，锁定 iam 应用。
//
// 机机账号只证明“谁在调”，不代替用户凭据；用户准入仍由 resolveLoginApplication 校验。
func (a AuthenticationService) resolveLoginApplicationCode(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get(auth.MetadataAccessKey)) == 0 {
		return defaultApplicationCode, nil
	}
	if a.signatures == nil {
		return "", status.Error(codes.Unauthenticated, "service authentication unavailable")
	}
	serviceAccountId, err := a.signatures.Authenticate(ctx, md)
	if err != nil {
		slog.Warn("service authentication failed on password auth", "error", err)
		return "", status.Error(codes.Unauthenticated, "service authentication failed")
	}
	var serviceAccount model.User
	if err := a.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, serviceAccountId).
		First(&serviceAccount).Error; err != nil {
		slog.Warn("service account lookup failed", "account_id", serviceAccountId, "error", err)
		return "", status.Error(codes.Unauthenticated, "service account not found")
	}
	if serviceAccount.HomeApplicationId <= 0 {
		return "", status.Error(codes.FailedPrecondition, "service account has no home application")
	}
	var application model.Application
	if err := a.db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnId, serviceAccount.HomeApplicationId).
		First(&application).Error; err != nil {
		slog.Warn("service application lookup failed", "application_id", serviceAccount.HomeApplicationId, "error", err)
		return "", status.Error(codes.FailedPrecondition, "service application not found")
	}
	return application.Code, nil
}

type AuthenticationService struct {
	db         *gorm.DB
	rdb        *redis.Client
	signatures *auth.SignatureStore
}

var _ iam_v1.AuthenticationServiceServer = (*AuthenticationService)(nil)

// NewAuthenticationService 构造认证服务（无 AK/SK 验签器，仅用于人机登录测试）。
func NewAuthenticationService(db *gorm.DB, rdb *redis.Client) iam_v1.AuthenticationServiceServer {
	return &AuthenticationService{db: db, rdb: rdb}
}

// NewAuthenticationServiceWithSignature 构造支持机机代登录的认证服务。
func NewAuthenticationServiceWithSignature(db *gorm.DB, rdb *redis.Client, signatures *auth.SignatureStore) iam_v1.AuthenticationServiceServer {
	return &AuthenticationService{db: db, rdb: rdb, signatures: signatures}
}

// PasswordAuth 密码登录（011 模型：t_user + bcrypt 摘要）。
// identifier 为双标识（username 或 email）：单次 OR 条件查询命中其一即取用户，
// 命中后 bcrypt 校验；用户不存在与密码错误统一语义，不泄露账号存在性。
func (a AuthenticationService) PasswordAuth(ctx context.Context, request *iam_v1.PasswordAuthRequest) (*iam_v1.PasswordAuthResponse, error) {
	var user model.User
	err := a.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnUsername+" = ? OR "+model.UserColumnEmail+" = ?", request.Identifier, request.Identifier).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 用户不存在与密码错误统一语义，不泄露账号存在性
			return nil, ErrInvalidAccountPassword
		}
		slog.Warn("query user for login failed", "identifier", request.Identifier, "error", err)
		return nil, err
	}

	// bcrypt 校验：password_hash 直接与明文比对，无盐字段
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)); err != nil {
		return nil, ErrInvalidAccountPassword
	}
	// 禁用账号拒绝登录（服务端安全裁决，fail-closed）
	if user.Status == constant.AccountStatusDisabled {
		return nil, ErrAccountDisabled
	}

	// 权限并集计算与读穿回源共用同一实现（research.md D6-①）
	if err := loadUserPermissionGraph(ctx, a.db, &user); err != nil {
		return nil, err
	}
	isAdmin := false
	for _, role := range user.RoleList {
		// 超管判定按标志（000014 起）：角色码应用内唯一，各应用可自建 'admin'
		// 应用级角色，按码判定会把应用级管理员误升为平台超管。
		if role.IsSuperAdmin {
			isAdmin = true
			break
		}
	}
	keys := collectUserPermissionCodes(user)
	jsonData, err := json.Marshal(keys)
	if err != nil {
		return nil, err
	}
	const jwtDuration = 30 * time.Minute
	expirationTime := time.Now().Add(jwtDuration)

	if err = a.rdb.Set(ctx, constant.GetAccountPermissionKey(user.Id), jsonData, constant.AccountPermissionSnapshotTTL).Err(); err != nil {
		return nil, err
	}
	// 应用锁定（ADR 0007 补充裁决）：机机代登录时目标应用由 AK/SK 验签后的服务账号
	// 所属应用决定；人机登录缺省 iam。调用方自报的 application_code 不参与决策。
	// 随后仍按 code 查启用应用并校验用户准入（admin 旁路；非 admin 须所属或准入绑定）。
	appCode, err := a.resolveLoginApplicationCode(ctx)
	if err != nil {
		return nil, err
	}
	appId, err := resolveLoginApplication(ctx, a.db, user.Id, user.HomeApplicationId, isAdmin, appCode)
	if err != nil {
		return nil, err
	}
	jti := cast.ToString(krand.NewID())
	jwtStr, err := auth.GenerateJWT(auth.ClaimsParam{
		UserId:         cast.ToString(user.Id),
		Username:       user.Username,
		ExpirationTime: expirationTime,
		IsAdmin:        isAdmin,
		Jti:            jti,
		ApplicationId:  appId,
	})

	accessTokenCookie := http.Cookie{
		Name:     auth.Authorization,
		Value:    jwtStr,
		Expires:  expirationTime,
		Path:     "/",
		HttpOnly: true,
	}
	err = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", accessTokenCookie.String()))
	if err != nil {
		return nil, err
	}
	return &iam_v1.PasswordAuthResponse{
		AccessToken: jwtStr,
	}, nil
}

// GetMyProfile 返回当前登录用户的个人信息（username/nickname/email/status/审计）。
// 仅凭登录态（JWT accountId）获取，不接收用户标识参数；响应不含密码摘要等敏感字段。
func (a AuthenticationService) GetMyProfile(ctx context.Context, empty *emptypb.Empty) (*iam_v1.GetMyProfileResponse, error) {
	accountId := auth.GetAccountId(ctx)
	if accountId == 0 {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	var user model.User
	err := a.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, accountId).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// token 对应的账号已不存在：按未认证处理，不泄露账号存在性
			return nil, status.Error(codes.Unauthenticated, "authentication required")
		}
		slog.Warn("get my profile failed", "account_id", accountId, "error", err)
		return nil, err
	}
	return &iam_v1.GetMyProfileResponse{
		Id:                cast.ToString(user.Id),
		Username:          user.Username,
		Nickname:          user.Nickname,
		Email:             user.EmailString(),
		Status:            user.Status,
		CreatedBy:         user.CreatedBy,
		CreateTime:        timestamppb.New(user.CreateTime),
		HomeApplicationId: cast.ToString(user.HomeApplicationId),
		IsAdmin:           auth.IsAdminFromContext(ctx),
	}, nil
}
