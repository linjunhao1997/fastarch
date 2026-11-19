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

type AuthenticationService struct {
	db  *gorm.DB
	rdb *redis.Client
}

var _ iam_v1.AuthenticationServiceServer = (*AuthenticationService)(nil)

// NewAuthenticationService 构造统一登录认证服务。
func NewAuthenticationService(db *gorm.DB, rdb *redis.Client) iam_v1.AuthenticationServiceServer {
	return &AuthenticationService{db: db, rdb: rdb}
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
	// 统一登录：令牌只承载身份与接入集合，不携带应用上下文。
	// 菜单/字典归属宿主固定 appId（拦截器注入），数据归属由请求头 appCode 选择，
	// 控制台由前端登录后选择；调用方自报的 application_code 一律忽略。
	// 接入集合快照写入 JWT：运行时数据归属校验以此为准（授权来自签名，
	// header appCode 仅是选择信号）。接入集合 = t_r_user_application 显式绑定。
	admittedAppIds, err := loadAdmittedAppIds(ctx, a.db, user.Id)
	if err != nil {
		slog.Warn("load admitted apps for jwt failed", "account_id", user.Id, "error", err)
		return nil, err
	}
	// ADR 0008：非 admin 只需属于某应用即可登录——隐含必须至少接入一个应用。
	// 令牌不再携带应用上下文，此处是唯一的登录期 fail-closed 门槛（admin 旁路）。
	if !isAdmin && len(admittedAppIds) == 0 {
		return nil, ErrApplicationAccessDenied
	}
	jti := cast.ToString(krand.NewID())
	jwtStr, err := auth.GenerateJWT(auth.ClaimsParam{
		UserId:         cast.ToString(user.Id),
		Username:       user.Username,
		ExpirationTime: expirationTime,
		IsAdmin:        isAdmin,
		Jti:            jti,
		AdmittedAppIds: admittedAppIds,
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
	isAdmin := auth.IsAdminFromContext(ctx)
	admittedApps, err := loadAdmittedApplicationSummaries(ctx, a.db, user.Id, isAdmin)
	if err != nil {
		slog.Warn("load admitted applications for profile failed", "account_id", accountId, "error", err)
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
		IsAdmin:           isAdmin,
		Applications:      admittedApps,
	}, nil
}
