package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"
)

type AccountService struct {
	db  *gorm.DB
	rdb *redis.Client
}

var _ iam_v1.AccountServiceServer = (*AccountService)(nil)

func accountToPb(a model.Account) *iam_v1.Account {
	return &iam_v1.Account{
		Id:         cast.ToString(a.Id),
		Name:       a.Name,
		Email:      a.Email,
		Kind:       a.Kind,
		Status:     a.Status,
		Version:    a.Version,
		CreatedBy:  cast.ToString(a.CreatedBy),
		UpdatedBy:  cast.ToString(a.UpdatedBy),
		CreateTime: timestamppb.New(a.CreateTime),
		UpdateTime: timestamppb.New(a.UpdateTime),
		RoleCodeList: func() []string {
			codes := make([]string, 0, len(a.RoleList))
			for _, role := range a.RoleList {
				codes = append(codes, role.Code)
			}
			return codes
		}(),
		ApplicationIdList: func() []string {
			ids := make([]string, 0, len(a.ApplicationList))
			for _, app := range a.ApplicationList {
				ids = append(ids, cast.ToString(app.Id))
			}
			return ids
		}(),
	}
}

func (service *AccountService) GetAccount(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Account, error) {
	res := model.Account{}
	err := service.db.WithContext(ctx).
		Select(model.AccountFields).
		Where(model.AccountColumnId, request.Id).
		Preload("ApplicationList").
		First(&res).Error
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
	buildQuery := func() *gorm.DB {
		query := service.db.WithContext(ctx).Model(model.Account{})
		if len(request.Id) > 0 {
			query = query.Where(model.AccountColumnId, request.Id)
		}
		return applyKeywordFilter(query, []string{
			model.AccountColumnName + " LIKE ?",
			model.AccountColumnEmail + " LIKE ?",
		}, request.Keyword)
	}
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	err = buildQuery().
		Limit(limit).
		Offset(offset).
		Order(model.AccountColumnId + " ASC").
		Preload("RoleList").
		Preload("ApplicationList").
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
				slog.Warn("check email uniqueness failed", "account_id", request.Id, "email", request.Email, "error", err)
				return nil, err
			}
			if count > 0 {
				slog.Warn("update account rejected: email duplicated", "account_id", request.Id, "email", request.Email)
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
	// application_id_list 的 presence 语义：nil = 未提供（不触碰归属，兼容密码等
	// 局部更新）；非 nil（含空集）= 全量替换（specs/008 FR-002，research.md D8）
	hasApplicationUpdate := request.ApplicationIdList != nil
	if len(updates) == 0 && !hasApplicationUpdate {
		return &emptypb.Empty{}, nil
	}
	operator := auth.GetAccountId(ctx)
	if len(updates) > 0 {
		updates[model.AccountColumnUpdateTime] = time.Now()
		// 审计字段服务端注入：仅更新修改人与修改时间（research.md D16，宪法 XII）
		updates[model.AccountColumnUpdatedBy] = operator
		updates[model.AccountColumnVersion] = account.Version + 1
	}
	// 账号字段更新与归属全量替换在同一事务内完成（specs/008 data-model 变更语义）
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			result := tx.Model(model.Account{}).
				Where(model.AccountColumnId, request.Id).
				Where(model.AccountColumnVersion, request.Version).
				Updates(updates)
			if result.Error != nil {
				slog.Warn("update account failed", "account_id", request.Id, "version", request.Version, "error", result.Error)
				return result.Error
			}
			if result.RowsAffected == 0 {
				slog.Warn("update account rejected: version conflict", "account_id", request.Id, "version", request.Version)
				return ErrUpdateAccountBusy
			}
		}
		if hasApplicationUpdate {
			return replaceAccountApplications(ctx, tx, request.Id, request.ApplicationIdList, operator)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// replaceAccountApplications 以 applicationIdList 为目标集合全量替换账号-应用归属
// （specs/008 FR-002；research.md D8）。差集增删在同一事务内完成，并记录归属变更
// 审计日志：操作人、账号、变更前后应用集合（FR-016）。
func replaceAccountApplications(ctx context.Context, tx *gorm.DB, accountId int64, applicationIdList []int64, operator int64) error {
	// 请求集合去重，重复归属幂等
	wanted := make([]int64, 0, len(applicationIdList))
	wantedSet := make(map[int64]struct{}, len(applicationIdList))
	for _, id := range applicationIdList {
		if _, ok := wantedSet[id]; ok {
			continue
		}
		wantedSet[id] = struct{}{}
		wanted = append(wanted, id)
	}
	// 目标应用必须存在（IN 批量校验，禁止 N+1）
	if len(wanted) > 0 {
		var count int64
		if err := tx.Model(model.Application{}).
			Where(model.ApplicationColumnId+" IN ?", wanted).
			Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(wanted)) {
			slog.Warn("replace account applications rejected: application not found",
				"account_id", accountId, "application_ids", wanted)
			return ErrApplicationNotFound
		}
	}
	// 现有关联一次性读出，内存计算差集
	var existing []model.RAccountApplication
	if err := tx.Where(model.RAccountApplicationColumnAccountId, accountId).
		Find(&existing).Error; err != nil {
		return err
	}
	existingSet := make(map[int64]struct{}, len(existing))
	before := make([]int64, 0, len(existing))
	for _, row := range existing {
		existingSet[row.ApplicationId] = struct{}{}
		before = append(before, row.ApplicationId)
	}
	added := make([]int64, 0, len(wanted))
	for _, id := range wanted {
		if _, ok := existingSet[id]; !ok {
			added = append(added, id)
		}
	}
	removed := make([]int64, 0, len(existing))
	for _, row := range existing {
		if _, ok := wantedSet[row.ApplicationId]; !ok {
			removed = append(removed, row.ApplicationId)
		}
	}
	if len(added) > 0 {
		rows := make([]model.RAccountApplication, 0, len(added))
		for _, id := range added {
			rows = append(rows, model.RAccountApplication{AccountId: accountId, ApplicationId: id})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(removed) > 0 {
		if err := tx.Where(model.RAccountApplicationColumnAccountId, accountId).
			Where(model.RAccountApplicationColumnApplicationId+" IN ?", removed).
			Delete(&model.RAccountApplication{}).Error; err != nil {
			return err
		}
	}
	if len(added) > 0 || len(removed) > 0 {
		// 归属变更审计：操作人、账号、变更前后应用集合（specs/008 FR-016）
		slog.Info("account applications changed",
			"operator", operator,
			"account_id", accountId,
			"before", before,
			"after", wanted,
			"added", added,
			"removed", removed)
	}
	return nil
}

func NewAccountService(db *gorm.DB, rdb *redis.Client) iam_v1.AccountServiceServer {
	return &AccountService{db: db, rdb: rdb}
}

// invalidateAccountPermissionSnapshot 删除账号的权限快照键，使绑定变更立即生效
// （research.md D6-①）。快照为派生数据：删除失败仅记日志，由短 TTL 兜底。
func (service *AccountService) invalidateAccountPermissionSnapshot(ctx context.Context, accountId int64) {
	if service.rdb == nil {
		return
	}
	if err := service.rdb.Del(ctx, constant.GetAccountPermissionKey(accountId)).Err(); err != nil {
		slog.Warn("invalidate account permission snapshot failed",
			"account_id", accountId, "error", err)
	}
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
				slog.Warn("bind roles rejected: role code not found",
					"account_id", request.AccountId, "added_role_codes", request.AddedRoleCodeList)
				return nil, ErrRoleNotFound
			}
		}
	}

	var account model.Account
	err := service.db.WithContext(ctx).
		Select(model.AccountFields).
		Where(model.AccountColumnId, request.AccountId).
		First(&account).Error
	if err != nil {
		slog.Warn("get account for role binding failed", "account_id", request.AccountId, "error", err)
		return nil, err
	}
	if err := loadAccountRoles(ctx, service.db, &account); err != nil {
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
		slog.Warn("commit role binding failed",
			"account_id", request.AccountId,
			"added_role_codes", request.AddedRoleCodeList,
			"removed_role_codes", request.RemovedRoleCodeList,
			"error", err)
		return nil, err
	}
	// 绑定变更立即生效：失效目标账号的权限快照（research.md D6-①）
	service.invalidateAccountPermissionSnapshot(ctx, request.AccountId)
	return &emptypb.Empty{}, nil
}

func (service *AccountService) DeleteAccount(ctx context.Context, request *iam_v1.DeleteAccountRequest) (*emptypb.Empty, error) {
	var count int64
	err := service.db.WithContext(ctx).Model(model.RAccountRole{}).
		Where(model.RAccountRoleColumnAccountId, request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check account role bindings failed", "account_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete account rejected: bound by roles", "account_id", request.Id, "binding_count", count)
		return nil, ErrAccountReferenced
	}

	// 名下访问凭证引用校验（specs/009-ak-sk-auth data-model：删除用户前凭证数须为 0）
	var credentialCount int64
	err = service.db.WithContext(ctx).Model(model.AccessCredential{}).
		Where(model.AccessCredentialColumnAccountId, request.Id).
		Count(&credentialCount).Error
	if err != nil {
		slog.Warn("check account credential references failed", "account_id", request.Id, "error", err)
		return nil, err
	}
	if credentialCount > 0 {
		slog.Warn("delete account rejected: bound by access credentials", "account_id", request.Id, "credential_count", credentialCount)
		return nil, ErrAccountReferencedByCredential
	}

	result := service.db.WithContext(ctx).
		Where(model.AccountColumnId, request.Id).
		Delete(model.Account{})
	if result.Error != nil {
		slog.Warn("delete account failed", "account_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete account rejected: account not found", "account_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

func (service *AccountService) CreateAccount(ctx context.Context, request *iam_v1.CreateAccountRequest) (*iam_v1.Account, error) {
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者，无认证上下文时 0（research.md D16）
	operator := auth.GetAccountId(ctx)
	// 状态缺省为启用（契约：status 字段可省略）
	status := request.Status
	if status == "" {
		status = constant.AccountStatusEnabled
	}
	account := model.Account{
		Id:         pkId,
		Name:       request.Name,
		Email:      request.Email,
		Kind:       request.Kind,
		Status:     status,
		CreateTime: now,
		UpdateTime: now,
		CreatedBy:  operator,
		UpdatedBy:  operator,
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
		if err := tx.Model(model.Account{}).Create(&account).Error; err != nil {
			return err
		}
		// 账号创建即写入应用归属（可为空 = 无归属账号，specs/008 FR-001/002）
		if len(request.ApplicationIdList) > 0 {
			return replaceAccountApplications(ctx, tx, account.Id, request.ApplicationIdList, operator)
		}
		return nil
	})
	if err != nil {
		slog.Warn("create account failed", "email", request.Email, "error", err)
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
