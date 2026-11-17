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
		// 管理员角色判定统一走常量
		if role.Code == model.RoleAdmin {
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
	// 应用上下文（多应用裁决 #8 + 模型 A）：签发 jti 作为映射键；
	// 登录时优先绑定所属应用（必须在可进入清单内），否则沿用"仅一个可进入应用"自动绑定；
	// 多应用且未设置所属应用时留空，由前端 ListMyApplications → SelectApplication 选择。
	jti := cast.ToString(krand.NewID())
	accessibleApps, err := loadAccessibleApplications(ctx, a.db, user.Id, user.HomeApplicationId, isAdmin)
	if err != nil {
		return nil, err
	}
	bindAppId := int64(0)
	for _, app := range accessibleApps {
		if app.Id == user.HomeApplicationId {
			bindAppId = app.Id
			break
		}
	}
	if bindAppId == 0 && len(accessibleApps) == 1 {
		bindAppId = accessibleApps[0].Id
	}
	if bindAppId != 0 {
		if err := auth.SetAppContext(ctx, a.rdb, jti, bindAppId, jwtDuration); err != nil {
			return nil, err
		}
	}
	jwtStr, err := auth.GenerateJWT(auth.ClaimsParam{
		UserId:         cast.ToString(user.Id),
		Username:       user.Username,
		ExpirationTime: expirationTime,
		IsAdmin:        isAdmin,
		Jti:            jti,
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
	apps, err := loadAccessibleApplications(ctx, a.db, user.Id, user.HomeApplicationId, auth.IsAdminFromContext(ctx))
	if err != nil {
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
		Applications:      applicationsToMyProfilePb(apps),
		HomeApplicationId: cast.ToString(user.HomeApplicationId),
		IsAdmin:           auth.IsAdminFromContext(ctx),
	}, nil
}

// applicationsToMyProfilePb 应用模型转个人信息/列表响应摘要。
func applicationsToMyProfilePb(apps []model.Application) []*iam_v1.MyProfileApplication {
	list := make([]*iam_v1.MyProfileApplication, 0, len(apps))
	for _, app := range apps {
		list = append(list, &iam_v1.MyProfileApplication{
			Id:     cast.ToString(app.Id),
			Code:   app.Code,
			Name:   app.Name,
			Status: app.Status,
		})
	}
	return list
}

// ListMyApplications 列出当前用户可进入的应用（admin 旁路全部启用应用；
// 其他用户按 t_r_user_application 准入绑定过滤，多应用裁决 #8）。
func (a AuthenticationService) ListMyApplications(ctx context.Context, empty *emptypb.Empty) (*iam_v1.ListMyApplicationsResponse, error) {
	userId := auth.GetAccountId(ctx)
	if userId == 0 {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	var user model.User
	if err := a.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, userId).
		First(&user).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Warn("list my applications: load user failed", "user_id", userId, "error", err)
		return nil, err
	}
	apps, err := loadAccessibleApplications(ctx, a.db, userId, user.HomeApplicationId, auth.IsAdminFromContext(ctx))
	if err != nil {
		slog.Warn("list my applications failed", "user_id", userId, "error", err)
		return nil, err
	}
	return &iam_v1.ListMyApplicationsResponse{
		Applications:      applicationsToMyProfilePb(apps),
		HomeApplicationId: cast.ToString(user.HomeApplicationId),
	}, nil
}

// SelectApplication 选择当前应用上下文：校验准入后写入 jti→应用映射
// （切换不重签令牌，多应用裁决 #8）。人机边界认 code，内部换 application_id。
func (a AuthenticationService) SelectApplication(ctx context.Context, request *iam_v1.SelectApplicationRequest) (*emptypb.Empty, error) {
	userId := auth.GetAccountId(ctx)
	if userId == 0 {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	jti := auth.GetTokenJti(ctx)
	if jti == "" {
		// 机器/外部令牌无 jti：应用上下文在签发时锁定于 claim，不可切换
		return nil, status.Error(codes.FailedPrecondition, "当前令牌不支持切换应用")
	}
	var app model.Application
	err := a.db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnCode, request.ApplicationCode).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "应用不存在")
		}
		slog.Warn("select application: query app failed", "code", request.ApplicationCode, "error", err)
		return nil, err
	}
	if app.Status != model.StatusEnabled {
		return nil, status.Error(codes.FailedPrecondition, "应用已停用")
	}
	// 准入校验（fail-closed）：admin 旁路；其他用户所属应用隐式准入，
	// 其余应用必须在 t_r_user_application 绑定。
	if !auth.IsAdminFromContext(ctx) {
		var user model.User
		if err := a.db.WithContext(ctx).
			Select(model.UserFields).
			Where(model.UserColumnId, userId).
			First(&user).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("select application: load user failed", "user_id", userId, "error", err)
			return nil, err
		}
		if app.Id != user.HomeApplicationId {
			var count int64
			err = a.db.WithContext(ctx).
				Model(model.RUserApplication{}).
				Where(model.RUserApplicationColumnUserId+" = ? AND "+model.RUserApplicationColumnApplicationId+" = ?", userId, app.Id).
				Count(&count).Error
			if err != nil {
				slog.Warn("select application: access check failed", "user_id", userId, "application_id", app.Id, "error", err)
				return nil, err
			}
			if count == 0 {
				return nil, status.Error(codes.PermissionDenied, "无权进入该应用")
			}
		}
	}
	if err := auth.SetAppContext(ctx, a.rdb, jti, app.Id, constant.AppContextTTL); err != nil {
		slog.Error("select application: bind app context failed", "user_id", userId, "application_id", app.Id, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "应用上下文暂不可用，请稍后重试")
	}
	return &emptypb.Empty{}, nil
}
