package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"iam-service/pkg/helper"
	"iam-service/pkg/model"
)

type AccountService struct {
	db *gorm.DB
}

var _ iam_v1.AccountServiceServer = (*AccountService)(nil)

func accountToPb(a model.Account) *iam_v1.Account {
	return &iam_v1.Account{
		Id:      cast.ToString(a.Id),
		Name:    a.Name,
		Email:   a.Email,
		Kind:    a.Kind,
		Version: a.Version,
		RoleCodeList: func() []string {
			codes := make([]string, 0, len(a.RoleList))
			for _, role := range a.RoleList {
				codes = append(codes, role.Code)
			}
			return codes
		}(),
	}
}

func (service *AccountService) GetAccount(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Account, error) {
	res := model.Account{}
	err := service.db.WithContext(ctx).Preload("RoleList").
		Where(model.AccountColumnId, request.Id).First(&res).Error
	if err != nil {
		return nil, err
	}
	return accountToPb(res), nil
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
		Order(model.AccountColumnId + " ASC").
		Preload("RoleList").
		Find(&res).Error
	if err != nil {
		return nil, err
	}
	accounts := make([]*iam_v1.Account, 0, len(res))
	for _, v := range res {
		accounts = append(accounts, accountToPb(v))
	}
	return &iam_v1.ListAccountsResponse{
		Accounts: accounts,
		Page:     helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (service *AccountService) UpdateAccount(ctx context.Context, request *iam_v1.UpdateAccountRequest) (*emptypb.Empty, error) {
	// 对account的id加分布式锁
	var account model.Account
	err := service.db.WithContext(ctx).Model(model.Account{}).
		Where(model.AccountColumnId, request.Id).
		First(&account).Error
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.AccountColumnName] = request.Name
	}

	if len(request.Email) > 0 {
		updates[model.AccountColumnEmail] = request.Email
		if request.Email != account.Email {
			var count int64
			err = service.db.WithContext(ctx).Model(model.Account{}).
				Where(model.AccountColumnEmail, request.Email).
				Where(model.AccountColumnId+" <> ?", request.Id).
				Count(&count).Error
			if err != nil {
				return nil, err
			}
			if count > 0 {
				return nil, ErrEmailDuplicated
			}
		}
	}

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
	}
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
	}
	updates[model.AccountColumnUpdateTime] = time.Now()
	updates[model.AccountColumnVersion] = account.Version + 1
	result := service.db.WithContext(ctx).Model(model.Account{}).
		Where(model.AccountColumnId, request.Id).
		Where(model.AccountColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrUpdateAccountBusy
	}
	return &emptypb.Empty{}, nil
}

func NewAccountService(db *gorm.DB) iam_v1.AccountServiceServer {
	return &AccountService{db: db}
}

func (service *AccountService) UpdateAccountRoles(ctx context.Context, request *iam_v1.UpdateAccountRolesRequest) (*emptypb.Empty, error) {
	roleCodes := make([]string, 0, len(request.AddedRoleCodeList)+len(request.RemovedRoleCodeList))
	roleCodes = append(roleCodes, request.AddedRoleCodeList...)
	roleCodes = append(roleCodes, request.RemovedRoleCodeList...)
	roleByCode := make(map[string]model.Role, len(roleCodes))
	if len(roleCodes) > 0 {
		var roles []model.Role
		err := service.db.WithContext(ctx).
			Where(model.RoleColumnCode+" IN ?", roleCodes).
			Find(&roles).Error
		if err != nil {
			return nil, err
		}
		for _, role := range roles {
			roleByCode[role.Code] = role
		}
		for _, code := range request.AddedRoleCodeList {
			if _, ok := roleByCode[code]; !ok {
				return nil, ErrRoleNotFound
			}
		}
	}

	var account model.Account
	err := service.db.WithContext(ctx).Preload("RoleList").
		Where(model.AccountColumnId, request.AccountId).
		First(&account).Error
	if err != nil {
		return nil, err
	}

	boundRoles := make(map[string]model.Role, len(account.RoleList))
	for _, role := range account.RoleList {
		boundRoles[role.Code] = *role
	}
	addedRoles := make([]model.Role, 0, len(request.AddedRoleCodeList))
	for _, code := range request.AddedRoleCodeList {
		if _, exists := boundRoles[code]; !exists {
			addedRoles = append(addedRoles, roleByCode[code])
		}
	}
	removedRoles := make([]model.Role, 0, len(request.RemovedRoleCodeList))
	for _, code := range request.RemovedRoleCodeList {
		if role, exists := boundRoles[code]; exists {
			removedRoles = append(removedRoles, role)
		}
	}

	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		association := tx.Model(&account).Association("RoleList")
		if err := association.Append(addedRoles); err != nil {
			return err
		}
		if len(removedRoles) > 0 {
			return association.Delete(removedRoles)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (service *AccountService) DeleteAccount(ctx context.Context, request *iam_v1.DeleteAccountRequest) (*emptypb.Empty, error) {
	var count int64
	err := service.db.WithContext(ctx).Model(model.RAccountRole{}).
		Where(model.RAccountRoleColumnAccountId, request.Id).
		Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrAccountReferenced
	}

	result := service.db.WithContext(ctx).
		Where(model.AccountColumnId, request.Id).
		Delete(model.Account{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

func (service *AccountService) CreateAccount(ctx context.Context, request *iam_v1.CreateAccountRequest) (*iam_v1.Account, error) {
	pkId := krand.NewID()
	now := time.Now()
	account := model.Account{
		Id:         pkId,
		Name:       request.Name,
		Email:      request.Email,
		Kind:       request.Kind,
		CreateTime: now,
		UpdateTime: now,
		Version:    1,
	}
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		err := tx.Model(model.Account{}).
			Where(model.AccountColumnEmail, request.Email).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrEmailDuplicated
		}
		return tx.Model(model.Account{}).Create(&account).Error
	})
	if err != nil {
		return nil, err
	}

	return accountToPb(account), nil
}

func (service *AccountService) generateSalt() (string, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}
