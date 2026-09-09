package service

import (
	"context"
	"encoding/json"
	"errors"
	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"
	"log/slog"
	"net/http"
	"time"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
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

func (a AuthenticationService) PasswordAuth(ctx context.Context, request *iam_v1.PasswordAuthRequest) (*iam_v1.PasswordAuthResponse, error) {
	account := model.Account{}
	err := a.db.WithContext(ctx).Model(model.Account{}).Scopes(func(db *gorm.DB) *gorm.DB {
		if len(request.Email) > 0 {
			db = db.Where(model.AccountColumnEmail, request.Email)
		}
		return db
	}).Select(model.AccountFields).First(&account).Error
	if err != nil {
		return nil, err
	}

	hashed := helper.HashPasswordWithSalt(request.Password, account.Salt)
	if hashed != account.Password {
		return nil, ErrInvalidAccountPassword
	}
	// 绂佺敤璐﹀彿鎷掔粷鐧诲綍锛堟湇鍔＄瀹夊叏瑁佸喅锛屽娉?IV锛?
	if account.Status == constant.AccountStatusDisabled {
		return nil, ErrAccountDisabled
	}

	if err = a.db.WithContext(ctx).Model(model.Account{}).
		Where(model.AccountColumnId, account.Id).
		Preload("RoleList.PermissionList.ResourceList").
		Preload("RoleList.Application").
		Find(&account).Error; err != nil {
		return nil, err
	}

	isAdmin := false
	for _, role := range account.RoleList {
		// 绠＄悊鍛樿鑹插垽瀹氱粺涓€璧板父閲忥紙specs/005-typed-permission-model research.md D8/D9锛?
		if role.Code == model.RoleAdmin {
			isAdmin = true
			break
		}
	}
	// 鏉冮檺骞堕泦璁＄畻涓庤绌垮洖婧愬叡鐢ㄥ悓涓€瀹炵幇锛坮esearch.md D6-鈶★級
	// 蹇収鍊间负甯︾被鍨嬪墠缂€鐨勮祫婧愰敭骞堕泦锛坢enu:/http:/rpc:锛?
	keys := collectAccountResourceKeys(account)
	jsonData, err := json.Marshal(keys)
	if err != nil {
		return nil, err
	}
	const jwtDuration = 30 * time.Minute
	expirationTime := time.Now().Add(jwtDuration)

	if err = a.rdb.Set(ctx, constant.GetAccountPermissionKey(account.Id), jsonData, constant.AccountPermissionSnapshotTTL).Err(); err != nil {
		return nil, err
	}
	jwtStr, err := auth.GenerateJWT(auth.ClaimsParam{
		UserId:         cast.ToString(account.Id),
		Username:       account.Name,
		ExpirationTime: expirationTime,
		IsAdmin:        isAdmin,
	})

	accessTokenCookie := http.Cookie{
		Name:    auth.Authorization,
		Value:   jwtStr,
		Expires: expirationTime,
		Path:    "/",
	}
	err = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", accessTokenCookie.String()))
	if err != nil {
		return nil, err
	}
	return &iam_v1.PasswordAuthResponse{
		AccessToken: jwtStr,
	}, nil
}

// GetMyProfile 杩斿洖褰撳墠鐧诲綍鐢ㄦ埛鐨勪釜浜轰俊鎭紙鍩烘湰淇℃伅 + 鎵€灞炲簲鐢ㄥ垪琛級銆?
// 浠呭嚟鐧诲綍鎬侊紙JWT accountId锛夎幏鍙栵紝涓嶆帴鏀剁敤鎴锋爣璇嗗弬鏁帮紱鍝嶅簲涓嶅惈瀵嗙爜銆佺洂銆?
// AK/SK 绛夋晱鎰熷嚟鎹瓧娈碉紙濂戠害灞傞潰澶╃劧闅旂锛宻pecs/008 FR-004/005锛夈€?
func (a AuthenticationService) GetMyProfile(ctx context.Context, empty *emptypb.Empty) (*iam_v1.GetMyProfileResponse, error) {
	accountId := auth.GetAccountId(ctx)
	if accountId == 0 {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	var account model.Account
	err := a.db.WithContext(ctx).
		Select(model.AccountFields).
		Where(model.AccountColumnId, accountId).
		Preload("ApplicationList").
		First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// token 瀵瑰簲鐨勮处鍙峰凡涓嶅瓨鍦細鎸夋湭璁よ瘉澶勭悊锛屼笉娉勯湶璐﹀彿瀛樺湪鎬?
			return nil, status.Error(codes.Unauthenticated, "authentication required")
		}
		slog.Warn("get my profile failed", "account_id", accountId, "error", err)
		return nil, err
	}

	applications := make([]*iam_v1.MyProfileApplication, 0, len(account.ApplicationList))
	for _, app := range account.ApplicationList {
		applications = append(applications, &iam_v1.MyProfileApplication{
			Id:     cast.ToString(app.Id),
			Code:   app.Code,
			Name:   app.Name,
			Status: func() string { if app.Disabled == 0 { return "enabled" }; return "disabled" }(),
		})
	}
	return &iam_v1.GetMyProfileResponse{
		Id:           cast.ToString(account.Id),
		Name:         account.Name,
		Email:        account.Email,
		Kind:         account.Kind,
		Status:       account.Status,
		CreatedBy:    cast.ToString(account.CreatedBy),
		CreateTime:   timestamppb.New(account.CreateTime),
		Applications: applications,
	}, nil
}
