package service

import (
	"context"
	"encoding/json"
	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"
	"net/http"
	"time"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
	}).First(&account).Error
	if err != nil {
		return nil, err
	}

	hashed := helper.HashPasswordWithSalt(request.Password, account.Salt)
	if hashed != account.Password {
		return nil, ErrInvalidAccountPassword
	}

	if err = a.db.WithContext(ctx).Model(model.Account{}).
		Where(model.AccountColumnId, account.Id).
		Preload("RoleList.PermissionList").
		Preload("RoleList.Application").
		Find(&account).Error; err != nil {
		return nil, err
	}

	isAdmin := false
	for _, role := range account.RoleList {
		if role.Code == "admin" {
			isAdmin = true
			break
		}
	}
	// 权限并集计算与读穿回源共用同一实现（research.md D6-②）
	codes := collectAccountPermissionCodes(account)
	jsonData, err := json.Marshal(codes)
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
