package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"github.com/go-dev-frame/sponge/pkg/krand"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"
	"time"
)

type AccountService struct {
	db *gorm.DB
}

var _ iam_v1.AccountServiceServer = (*AccountService)(nil)

func (service *AccountService) GetAccount(ctx context.Context, request *iam_v1.GetRequest) (*iam_v1.GetAccountResponse, error) {
	res := model.Account{}
	err := service.db.WithContext(ctx).Where(model.AccountColumnId, request.Id).First(res).Error
	if err != nil {
		return nil, err
	}
	return &iam_v1.GetAccountResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Account: &iam_v1.Account{
			Id:     cast.ToString(request.Id),
			Name:   res.Name,
			Email:  res.Email,
			KindId: res.Kind,
		},
	}, nil
}

func (service *AccountService) ListAccounts(ctx context.Context, request *iam_v1.ListAccountsRequest) (*iam_v1.ListAccountsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	var count int64
	var res []model.Account
	// 分页查询
	err := service.db.WithContext(ctx).Model(model.Account{}).Scopes(
		func(db *gorm.DB) *gorm.DB {
			if len(request.Id) > 0 {
				db = db.Where(model.AccountColumnId, request.Id)
			}
			return db
		}).
		Count(&count).
		Limit(limit).
		Offset(offset).
		Find(&res).Error
	if err != nil {
		return nil, err
	}
	accounts := make([]*iam_v1.Account, 0, len(res))
	for _, v := range res {
		accounts = append(accounts, &iam_v1.Account{
			Id:     cast.ToString(v.Id),
			Name:   v.Name,
			Email:  v.Email,
			KindId: v.Kind,
		})
	}
	return &iam_v1.ListAccountsResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Accounts: accounts,
		Page:     helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil

}

func (service *AccountService) UpdateAccount(ctx context.Context, request *iam_v1.UpdateAccountRequest) (*iam_v1.StatusResponse, error) {
	// 对account的id加分布式锁
	var account model.Account
	err := service.db.WithContext(ctx).Model(model.Account{}).
		Where(model.AccountColumnId, request.Id).
		First(&account).Error
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if len(request.Status) > 0 {
		updates[model.AccountColumnStatus] = request.Status
	}

	if account.Kind == model.AccountKindIdUser && len(request.Password) > 0 {
		salt, err := service.generateSalt()
		if err != nil {
			return nil, err
		}

		updates[model.AccountColumnSalt] = salt
		updates[model.AccountColumnPassword] = helper.HashPasswordWithSalt(request.Password, salt)
		updates[model.AccountColumnUpdateTime] = time.Now()
		updates[model.AccountColumnVersion] = account.Version + 1
	}
	result := service.db.WithContext(ctx).Model(model.Account{}).
		Where(model.AccountColumnId, request.Id).
		Where(model.AccountColumnVersion, account.Version).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrUpdateAccountBusy
	}
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
	}, nil
}

var _ iam_v1.AccountServiceServer = (*AccountService)(nil)

func NewAccountService(db *gorm.DB) iam_v1.AccountServiceServer {
	return &AccountService{db: db}
}

func (service *AccountService) CreateAccount(ctx context.Context, request *iam_v1.CreateAccountRequest) (*iam_v1.StatusResponse, error) {
	pkId := krand.NewID()
	now := time.Now()
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(model.Account{}).Create(
			&model.Account{
				Id:         pkId,
				Name:       request.Name,
				Email:      request.Email,
				Kind:       request.Kind,
				CreateTime: now,
				UpdateTime: now,
			}).Error
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &iam_v1.StatusResponse{Status: &iam_v1.Status{
		Code:   constant.ResponseCodeSuccess,
		DataId: cast.ToString(pkId),
	}}, nil
}

func (service *AccountService) generateSalt() (string, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}
