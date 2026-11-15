package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"golang.org/x/crypto/bcrypt"
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

func NewAccountService(db *gorm.DB, rdb *redis.Client) *AccountService {
	return &AccountService{db: db, rdb: rdb}
}

// accountToPb 模型转 pb：proto 未随 011 模型更新，Name 映射 username；
// email/kind/application_id_list 在新模型中已不存在，一律留空。
func accountToPb(a model.User) *iam_v1.Account {
	return &iam_v1.Account{
		Id:         cast.ToString(a.Id),
		Name:       a.Username,
		Status:     a.Status,
		Version:    a.Version,
		CreatedBy:  a.CreatedBy,
		UpdatedBy:  a.UpdatedBy,
		CreateTime: timestamppb.New(a.CreateTime),
		UpdateTime: timestamppb.New(a.UpdateTime),
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
	res := model.User{}
	err := service.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, request.Id).
		First(&res).Error
	if err != nil {
		return nil, err
	}
	// 角色清单走显式关联查询（Preload 会生成 SELECT *）
	if err := loadUserRoles(ctx, service.db, &res); err != nil {
		return nil, err
	}
	return accountToPb(res), nil
}

func (service *AccountService) ListAccounts(ctx context.Context, request *iam_v1.ListAccountsRequest) (*iam_v1.ListAccountsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	var count int64
	var res []model.User
	// 分页查询：关键字匹配用户名与昵称
	buildQuery := func() *gorm.DB {
		query := service.db.WithContext(ctx).Model(model.User{})
		if len(request.Id) > 0 {
			query = query.Where(model.UserColumnId, request.Id)
		}
		return applyKeywordFilter(query, []string{
			model.UserColumnUsername + " LIKE ?",
			model.UserColumnNickname + " LIKE ?",
		}, request.Keyword)
	}
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	err = buildQuery().
		Limit(limit).
		Offset(offset).
		Order(model.UserColumnId + " ASC").
		Find(&res).Error
	if err != nil {
		return nil, err
	}
	// 角色清单批量组装（Preload 会生成 SELECT *）
	if err := loadUsersRoles(ctx, service.db, res); err != nil {
		return nil, err
	}
	users := make([]*iam_v1.Account, 0, len(res))
	for _, v := range res {
		users = append(users, accountToPb(v))
	}
	return &iam_v1.ListAccountsResponse{
		Accounts: users,
		Page:     helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (service *AccountService) UpdateAccount(ctx context.Context, request *iam_v1.UpdateAccountRequest) (*emptypb.Empty, error) {
	var user model.User
	err := service.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, request.Id).
		First(&user).Error
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		// username 为唯一登录名不可改；Name 映射为昵称（显示名）
		updates[model.UserColumnNickname] = request.Name
	}
	if len(request.Status) > 0 {
		updates[model.UserColumnStatus] = request.Status
	}
	if len(request.Password) > 0 {
		// 密码统一走 bcrypt：无盐字段，摘要直接落 password_hash
		hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		updates[model.UserColumnPasswordHash] = string(hash)
		updates[model.UserColumnUpdateTime] = time.Now()
	}
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
	}
	operator := cast.ToString(auth.GetAccountId(ctx))
	updates[model.UserColumnUpdateTime] = time.Now()
	// 审计字段服务端注入：仅更新修改人与修改时间（宪法 XII）
	updates[model.UserColumnUpdatedBy] = operator
	updates[model.UserColumnVersion] = user.Version + 1

	result := service.db.WithContext(ctx).Model(model.User{}).
		Where(model.UserColumnId, request.Id).
		Where(model.UserColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		slog.Warn("update user failed", "user_id", request.Id, "version", request.Version, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("update user rejected: version conflict", "user_id", request.Id, "version", request.Version)
		return nil, ErrUpdateAccountBusy
	}
	return &emptypb.Empty{}, nil
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
					"user_id", request.AccountId, "added_role_codes", request.AddedRoleCodeList)
				return nil, ErrRoleNotFound
			}
		}
	}

	var user model.User
	err := service.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, request.AccountId).
		First(&user).Error
	if err != nil {
		slog.Warn("get user for role binding failed", "user_id", request.AccountId, "error", err)
		return nil, err
	}
	if err := loadUserRoles(ctx, service.db, &user); err != nil {
		return nil, err
	}

	boundRoles := make(map[string]model.Role, len(user.RoleList))
	for _, role := range user.RoleList {
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
		association := tx.Model(&user).Association("RoleList")
		// GORM Append 空切片仍会重复插入关联行，必须跳过空清单
		if len(addedRoles) > 0 {
			if err := association.Append(addedRoles); err != nil {
				return err
			}
		}
		if len(removedRoles) > 0 {
			return association.Delete(removedRoles)
		}
		return nil
	})
	if err != nil {
		slog.Warn("commit role binding failed",
			"user_id", request.AccountId,
			"added_role_codes", request.AddedRoleCodeList,
			"removed_role_codes", request.RemovedRoleCodeList,
			"error", err)
		return nil, err
	}
	// 绑定变更立即生效：失效目标用户的权限快照（research.md D6-①）
	service.invalidateUserPermissionSnapshot(ctx, request.AccountId)
	return &emptypb.Empty{}, nil
}

func (service *AccountService) DeleteAccount(ctx context.Context, request *iam_v1.DeleteAccountRequest) (*emptypb.Empty, error) {
	var count int64
	err := service.db.WithContext(ctx).Model(model.RUserRole{}).
		Where(model.RUserRoleColumnUserId, request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check user role bindings failed", "user_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete user rejected: bound by roles", "user_id", request.Id, "binding_count", count)
		return nil, ErrAccountReferenced
	}

	// 名下访问凭证引用校验（specs/009-ak-sk-auth data-model：删除用户前凭证数须为 0）
	var credentialCount int64
	err = service.db.WithContext(ctx).Model(model.AccessCredential{}).
		Where(model.AccessCredentialColumnUserId, request.Id).
		Count(&credentialCount).Error
	if err != nil {
		slog.Warn("check user credential references failed", "user_id", request.Id, "error", err)
		return nil, err
	}
	if credentialCount > 0 {
		slog.Warn("delete user rejected: bound by access credentials", "user_id", request.Id, "credential_count", credentialCount)
		return nil, ErrAccountReferencedByCredential
	}

	result := service.db.WithContext(ctx).
		Where(model.UserColumnId, request.Id).
		Delete(model.User{})
	if result.Error != nil {
		slog.Warn("delete user failed", "user_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete user rejected: user not found", "user_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

func (service *AccountService) CreateAccount(ctx context.Context, request *iam_v1.CreateAccountRequest) (*iam_v1.Account, error) {
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者，无认证上下文时空串（research.md D16）
	operator := cast.ToString(auth.GetAccountId(ctx))
	// 状态缺省为启用（契约：status 字段可省略）
	status := request.Status
	if status == "" {
		status = constant.AccountStatusEnabled
	}
	// proto 未随 011 模型更新：Name 映射为 username（唯一登录名）；
	// email/kind/application_id_list 已随模型删除，忽略
	user := model.User{
		Id:         pkId,
		Username:   request.Name,
		Nickname:   request.Name,
		Status:     status,
		Version:    1,
		CreatedBy:  operator,
		UpdatedBy:  operator,
		CreateTime: now,
		UpdateTime: now,
	}
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		err := tx.Model(model.User{}).
			Where(model.UserColumnUsername, request.Name).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrUsernameDuplicated
		}
		return tx.Select(model.UserFields).Create(&user).Error
	})
	if err != nil {
		slog.Warn("create user failed", "username", request.Name, "error", err)
		return nil, err
	}
	return accountToPb(user), nil
}

// invalidateUserPermissionSnapshot 删除用户的权限快照键，使绑定变更立即生效
// （research.md D6-①）。快照为派生数据：删除失败仅记日志，由短 TTL 兜底。
func (service *AccountService) invalidateUserPermissionSnapshot(ctx context.Context, userId int64) {
	if service.rdb == nil {
		return
	}
	if err := service.rdb.Del(ctx, constant.GetAccountPermissionKey(userId)).Err(); err != nil {
		slog.Warn("invalidate user permission snapshot failed",
			"user_id", userId, "error", err)
	}
}
