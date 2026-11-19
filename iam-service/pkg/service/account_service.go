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

// filterUserRolesByScope 账号角色清单的读侧数据范围隔离（ADR-0009）：
// 非 admin 只保留归属应用 == 其 dataAppId 的角色（sender 侧看不到目标账号的
// iam 角色）；admin/系统调用保留全部；未注入数据范围则 fail-closed 清空。
// scoped/scopeId 来自 operatorDataScope，由调用方复用避免重复查库。
func filterUserRolesByScope(u *model.User, scopeId int64, scoped bool) {
	if !scoped {
		return
	}
	if u == nil {
		return
	}
	if scopeId <= 0 {
		u.RoleList = []*model.Role{}
		return
	}
	filtered := make([]*model.Role, 0, len(u.RoleList))
	for _, role := range u.RoleList {
		if role.ApplicationId == scopeId {
			filtered = append(filtered, role)
		}
	}
	u.RoleList = filtered
}

// accountToPb 模型转 pb：id/username/email/nickname/status/audit/role_code_list。
func accountToPb(a model.User) *iam_v1.Account {
	return &iam_v1.Account{
		Id:                cast.ToString(a.Id),
		Username:          a.Username,
		Email:             a.EmailString(),
		Nickname:          a.Nickname,
		Status:            a.Status,
		Version:           a.Version,
		CreatedBy:         a.CreatedBy,
		UpdatedBy:         a.UpdatedBy,
		CreateTime:        timestamppb.New(a.CreateTime),
		UpdateTime:        timestamppb.New(a.UpdateTime),
		RoleCodeList: func() []string {
			codes := make([]string, 0, len(a.RoleList))
			for _, role := range a.RoleList {
				codes = append(codes, role.Code)
			}
			return codes
		}(),
		// 与 role_code_list 同源同序；绑定接口按 id 承载（ADR-0009）。
		RoleIds: func() []string {
			ids := make([]string, 0, len(a.RoleList))
			for _, role := range a.RoleList {
				ids = append(ids, cast.ToString(role.Id))
			}
			return ids
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
	if err := service.requireAccountScope(ctx, res.Id); err != nil {
		return nil, err
	}
	// 角色清单走显式关联查询（Preload 会生成 SELECT *）
	if err := loadUserRoles(ctx, service.db, &res); err != nil {
		return nil, err
	}
	// 读侧隔离（ADR-0009）：非 admin 仅可见本数据范围应用的角色。
	scopeId, scoped, err := operatorDataScope(ctx, service.db)
	if err != nil {
		return nil, err
	}
	filterUserRolesByScope(&res, scopeId, scoped)
	account := accountToPb(res)
	// 可接入应用清单（ADR 0008）：详情/列表统一经批量查询回填（含展示字段）
	appsByUser, err := loadUsersApplications(ctx, service.db, []model.User{res})
	if err != nil {
		return nil, err
	}
	account.Applications = applicationsToPb(appsByUser[res.Id])
	return account, nil
}

// applicationsToPb 准入应用模型转摘要 pb（仅展示字段，不含机器字段）。
func applicationsToPb(apps []*model.Application) []*iam_v1.AccountApplication {
	list := make([]*iam_v1.AccountApplication, 0, len(apps))
	for _, app := range apps {
		list = append(list, applicationSummaryToPb(app))
	}
	return list
}

// applicationSummaryToPb 单个应用摘要；所属应用可能不在准入清单中，需单独回填。
func applicationSummaryToPb(app *model.Application) *iam_v1.AccountApplication {
	if app == nil {
		return nil
	}
	return &iam_v1.AccountApplication{
		Id:     cast.ToString(app.Id),
		Code:   app.Code,
		Name:   app.Name,
		Status: app.Status,
	}
}

// loadApplicationsByIds 批量取应用摘要，避免为每个用户单独查所属应用。
func loadApplicationsByIds(ctx context.Context, db *gorm.DB, ids []int64) (map[int64]*model.Application, error) {
	result := make(map[int64]*model.Application, len(ids))
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return result, nil
	}
	var apps []*model.Application
	if err := db.WithContext(ctx).
		Where(model.ApplicationColumnId+" IN ?", unique).
		Select(model.ApplicationFields).
		Find(&apps).Error; err != nil {
		return nil, err
	}
	for _, app := range apps {
		result[app.Id] = app
	}
	return result, nil
}

// requireAccountScope 数据边界校验（ADR 0008）：非 admin 操作者仅可查看/操作
// 接入了当前数据范围应用的账号；admin/系统调用旁路。
func (service *AccountService) requireAccountScope(ctx context.Context, targetUserId int64) error {
	return requireAccountScope(ctx, service.db, targetUserId)
}

// UpdateAccountApplications 设置用户接入应用（ADR 0008）：
// application_ids 全量覆盖语义（空数组 = 清空接入集合）。只写关联表，
// 不触碰应用行或用户行。
func (service *AccountService) UpdateAccountApplications(ctx context.Context, request *iam_v1.UpdateAccountApplicationsRequest) (*emptypb.Empty, error) {
	var user model.User
	err := service.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, request.AccountId).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	if err := service.requireAccountScope(ctx, user.Id); err != nil {
		return nil, err
	}
	appIds := make([]int64, 0, len(request.ApplicationIds))
	seen := make(map[int64]struct{}, len(request.ApplicationIds))
	for _, raw := range request.ApplicationIds {
		id := cast.ToInt64(raw)
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		appIds = append(appIds, id)
	}
	if len(appIds) > 0 {
		var count int64
		err = service.db.WithContext(ctx).Model(model.Application{}).
			Where(model.ApplicationColumnId+" IN ?", appIds).
			Count(&count).Error
		if err != nil {
			return nil, err
		}
		if count != int64(len(appIds)) {
			slog.Warn("update account applications rejected: some applications not found",
				"account_id", request.AccountId, "application_ids", request.ApplicationIds)
			return nil, ErrApplicationNotFoundForScope
		}
	}

	var currentIds []int64
	err = service.db.WithContext(ctx).Model(model.RUserApplication{}).
		Where(model.RUserApplicationColumnUserId, user.Id).
		Pluck(model.RUserApplicationColumnApplicationId, &currentIds).Error
	if err != nil {
		return nil, err
	}
	currentSet := make(map[int64]struct{}, len(currentIds))
	for _, id := range currentIds {
		currentSet[id] = struct{}{}
	}
	targetSet := make(map[int64]struct{}, len(appIds))
	for _, id := range appIds {
		targetSet[id] = struct{}{}
	}
	addedIds := make([]int64, 0, len(appIds))
	for _, id := range appIds {
		if _, exists := currentSet[id]; !exists {
			addedIds = append(addedIds, id)
		}
	}
	removedIds := make([]int64, 0, len(currentIds))
	for _, id := range currentIds {
		if _, exists := targetSet[id]; !exists {
			removedIds = append(removedIds, id)
		}
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(removedIds) > 0 {
			if err := tx.Where(model.RUserApplicationColumnUserId+" = ? AND "+model.RUserApplicationColumnApplicationId+" IN ?",
				user.Id, removedIds).Delete(&model.RUserApplication{}).Error; err != nil {
				return err
			}
		}
		if len(addedIds) > 0 {
			rows := make([]model.RUserApplication, 0, len(addedIds))
			for _, id := range addedIds {
				rows = append(rows, model.RUserApplication{UserId: user.Id, ApplicationId: id})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("commit account application binding failed",
			"account_id", request.AccountId, "application_ids", request.ApplicationIds, "error", err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (service *AccountService) ListAccounts(ctx context.Context, request *iam_v1.ListAccountsRequest) (*iam_v1.ListAccountsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	var count int64
	var res []model.User
	// 数据边界（ADR 0008）：非 admin 仅见接入了当前数据范围应用的用户；
	// 未注入数据范围应用 fail-closed 空列表。
	scopeId, scoped, err := operatorDataScope(ctx, service.db)
	if err != nil {
		return nil, err
	}
	if scoped && scopeId <= 0 {
		return &iam_v1.ListAccountsResponse{
			Accounts: []*iam_v1.Account{},
			Page:     helper.GetPage(request.CurrentPage, request.PageSize, 0),
		}, nil
	}
	// 分页查询：关键字模糊匹配 username/nickname/email（双标识登录词汇全覆盖）
	buildQuery := func() *gorm.DB {
		query := service.db.WithContext(ctx).Model(model.User{})
		if scoped {
			// 接入集合过滤：用户须显式接入当前数据范围应用（t_r_user_application）。
			sub := service.db.WithContext(ctx).Model(model.RUserApplication{}).
				Select(model.RUserApplicationColumnUserId).
				Where(model.RUserApplicationColumnApplicationId, scopeId)
			query = query.Where(model.UserColumnId+" IN (?)", sub)
		}
		return applyKeywordFilter(query, []string{
			model.UserColumnUsername + " LIKE ?",
			model.UserColumnNickname + " LIKE ?",
			model.UserColumnEmail + " LIKE ?",
		}, request.Keyword)
	}
	err = buildQuery().Count(&count).Error
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
	// 读侧隔离（ADR-0009）：非 admin 仅可见本数据范围应用的角色，复用上方 scopeId。
	for i := range res {
		filterUserRolesByScope(&res[i], scopeId, scoped)
	}
	// 准入应用清单批量回填（绑定 + 应用两次 IN 查询按用户分组，避免 N+1）
	appsByUser, err := loadUsersApplications(ctx, service.db, res)
	if err != nil {
		return nil, err
	}
	users := make([]*iam_v1.Account, 0, len(res))
	for _, v := range res {
		account := accountToPb(v)
		account.Applications = applicationsToPb(appsByUser[v.Id])
		users = append(users, account)
	}
	return &iam_v1.ListAccountsResponse{
		Accounts: users,
		Page:     helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

// UpdateAccount 修改用户：nickname/email/status 可选更新（空串 = 不修改），
// username 为唯一登录名不可改，乐观锁防并发覆盖。
func (service *AccountService) UpdateAccount(ctx context.Context, request *iam_v1.UpdateAccountRequest) (*emptypb.Empty, error) {
	var user model.User
	err := service.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, request.Id).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	if err := service.requireAccountScope(ctx, user.Id); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if len(request.Nickname) > 0 {
		updates[model.UserColumnNickname] = request.Nickname
	}
	if len(request.Email) > 0 {
		// 邮箱唯一：先查重转友好错误（DB 层唯一索引兜底）
		var count int64
		err := service.db.WithContext(ctx).Model(model.User{}).
			Where(model.UserColumnEmail+" = ?", request.Email).
			Where(model.UserColumnId+" <> ?", request.Id).
			Count(&count).Error
		if err != nil {
			return nil, err
		}
		if count > 0 {
			slog.Warn("update user rejected: email duplicated", "user_id", request.Id)
			return nil, ErrEmailDuplicated
		}
		email := request.Email
		updates[model.UserColumnEmail] = &email
	}
	if len(request.Status) > 0 {
		updates[model.UserColumnStatus] = request.Status
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
	// 数据边界先行：越界调用者连角色存在性都不校验（fail-closed）
	var user model.User
	err := service.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, request.AccountId).
		First(&user).Error
	if err != nil {
		slog.Warn("get user for role binding failed", "user_id", request.AccountId, "error", err)
		return nil, err
	}
	if err := service.requireAccountScope(ctx, user.Id); err != nil {
		return nil, err
	}
	// 按全局唯一 roleId 解析（ADR-0009）：id 天然消解跨应用同名码歧义，
	// 超管/系统调用不加应用过滤可跨应用分配；非 admin 的归属与提权校验
	// 交给 requireRoleBindScope 统一兜底。added/removed 走同一套解析与校验，
	// 任何无法解析或越界的 id 一律 fail-closed。
	requestedRoleIds := make([]int64, 0, len(request.AddedRoleIds)+len(request.RemovedRoleIds))
	seenRequest := make(map[int64]struct{}, len(request.AddedRoleIds)+len(request.RemovedRoleIds))
	for _, raw := range request.AddedRoleIds {
		if id, ok := parseRoleId(raw); ok {
			if _, dup := seenRequest[id]; !dup {
				seenRequest[id] = struct{}{}
				requestedRoleIds = append(requestedRoleIds, id)
			}
		} else {
			return nil, ErrRoleNotFound
		}
	}
	for _, raw := range request.RemovedRoleIds {
		if id, ok := parseRoleId(raw); ok {
			if _, dup := seenRequest[id]; !dup {
				seenRequest[id] = struct{}{}
				requestedRoleIds = append(requestedRoleIds, id)
			}
		} else {
			return nil, ErrRoleNotFound
		}
	}
	roleById := make(map[int64]model.Role, len(requestedRoleIds))
	if len(requestedRoleIds) > 0 {
		var roles []model.Role
		if err := service.db.WithContext(ctx).
			Where(model.RoleColumnId+" IN ?", requestedRoleIds).
			Select(model.RoleFields).
			Find(&roles).Error; err != nil {
			return nil, err
		}
		for _, role := range roles {
			roleById[role.Id] = role
		}
		// 不存在（或被删）的 id 一律拒绝，不静默忽略（fail-closed）。
		if len(roleById) != len(requestedRoleIds) {
			slog.Warn("bind roles rejected: role id not found",
				"account_id", request.AccountId, "requested_role_ids", requestedRoleIds)
			return nil, ErrRoleNotFound
		}
		// added 与 removed 共用同一套归属/提权校验（ADR-0009 对称 fail-closed）：
		// 非 admin 解绑越界角色同样被拒，杜绝旧实现里删掉 iam 角色的越权路径。
		allRoles := make([]model.Role, 0, len(requestedRoleIds))
		for _, id := range requestedRoleIds {
			allRoles = append(allRoles, roleById[id])
		}
		if err := requireRoleBindScope(ctx, service.db, allRoles); err != nil {
			return nil, err
		}
	}

	if err := loadUserRoles(ctx, service.db, &user); err != nil {
		return nil, err
	}

	boundRoleIds := make(map[int64]struct{}, len(user.RoleList))
	for _, role := range user.RoleList {
		boundRoleIds[role.Id] = struct{}{}
	}
	addedRoleIds := make([]int64, 0, len(request.AddedRoleIds))
	for _, raw := range request.AddedRoleIds {
		id, _ := parseRoleId(raw)
		if _, exists := boundRoleIds[id]; !exists {
			addedRoleIds = append(addedRoleIds, id)
		}
	}
	removedRoleIds := make([]int64, 0, len(request.RemovedRoleIds))
	for _, raw := range request.RemovedRoleIds {
		id, _ := parseRoleId(raw)
		if _, exists := boundRoleIds[id]; exists {
			removedRoleIds = append(removedRoleIds, id)
		}
	}

	// 绑定只写关联表 t_r_user_role，绝不触碰 role 行本身；
	// 不用 Association.Append——它会对关联记录做 upsert（零值时间触发
	// MySQL 0000-00-00），且可能重复插入。
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(addedRoleIds) > 0 {
			rows := make([]model.RUserRole, 0, len(addedRoleIds))
			for _, id := range addedRoleIds {
				rows = append(rows, model.RUserRole{UserId: user.Id, RoleId: id})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		if len(removedRoleIds) > 0 {
			if err := tx.Where(model.RUserRoleColumnUserId+" = ? AND "+model.RUserRoleColumnRoleId+" IN ?",
				user.Id, removedRoleIds).Delete(&model.RUserRole{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("commit role binding failed",
			"user_id", request.AccountId,
			"added_role_ids", request.AddedRoleIds,
			"removed_role_ids", request.RemovedRoleIds,
			"error", err)
		return nil, err
	}
	// 绑定变更立即生效：失效目标用户的权限快照（research.md D6-①）
	service.invalidateUserPermissionSnapshot(ctx, request.AccountId)
	return &emptypb.Empty{}, nil
}

// parseRoleId 解析角色 id 字符串（proto 用 string 编码 int64）：须为正整数，
// 非法输入 fail-closed 交给调用方拒绝，不静默跳过。
func parseRoleId(raw string) (int64, bool) {
	id := cast.ToInt64(raw)
	return id, id > 0
}

// ResetAccountPassword 管理员重置用户密码：新密码 bcrypt 摘要落库，
// 不要求提供旧密码（管理员权威通道）；version+1 并注入审计字段。
func (service *AccountService) ResetAccountPassword(ctx context.Context, request *iam_v1.ResetAccountPasswordRequest) (*emptypb.Empty, error) {
	var user model.User
	err := service.db.WithContext(ctx).
		Select(model.UserFields).
		Where(model.UserColumnId, request.Id).
		First(&user).Error
	if err != nil {
		slog.Warn("get user for password reset failed", "user_id", request.Id, "error", err)
		return nil, err
	}
	if err := service.requireAccountScope(ctx, user.Id); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.Warn("hash new password failed", "user_id", request.Id, "error", err)
		return nil, err
	}
	newPasswordHash := string(hash)
	operator := cast.ToString(auth.GetAccountId(ctx))
	result := service.db.WithContext(ctx).Model(model.User{}).
		Where(model.UserColumnId, request.Id).
		Updates(map[string]interface{}{
			model.UserColumnPasswordHash: newPasswordHash,
			model.UserColumnUpdatedBy:    operator,
			model.UserColumnUpdateTime:   time.Now(),
			model.UserColumnVersion:      user.Version + 1,
		})
	if result.Error != nil {
		slog.Warn("reset user password failed", "user_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("reset password rejected: user not found", "user_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

func (service *AccountService) DeleteAccount(ctx context.Context, request *iam_v1.DeleteAccountRequest) (*emptypb.Empty, error) {
	var target model.User
	err := service.db.WithContext(ctx).
		Select(model.UserColumnId).
		Where(model.UserColumnId, request.Id).
		First(&target).Error
	if err != nil {
		return nil, err
	}
	if err := service.requireAccountScope(ctx, target.Id); err != nil {
		return nil, err
	}
	var count int64
	err = service.db.WithContext(ctx).Model(model.RUserRole{}).
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

// CreateAccount 创建用户：初始密码必填（bcrypt 摘要落库），email 可选（唯一），
// role_ids 可选初始角色绑定；username 唯一。接入应用必填且均须启用（ADR 0008），
// 全量写入 t_r_user_application；非 admin 的接入集合须包含其数据范围应用。
func (service *AccountService) CreateAccount(ctx context.Context, request *iam_v1.CreateAccountRequest) (*iam_v1.Account, error) {
	// 初始密码必填（创建路径无旧密码可校验，直接拒绝缺省）
	if len(request.Password) == 0 {
		slog.Warn("create user rejected: password required", "username", request.Username)
		return nil, ErrPasswordRequired
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Warn("hash initial password failed", "username", request.Username, "error", err)
		return nil, err
	}
	passwordHash := string(hash)

	// 接入应用必填（ADR 0008）：去重、排除非法 id，全部须存在且已启用
	appIds := make([]int64, 0, len(request.ApplicationIds))
	seenApps := make(map[int64]struct{}, len(request.ApplicationIds))
	for _, raw := range request.ApplicationIds {
		id := cast.ToInt64(raw)
		if id <= 0 {
			continue
		}
		if _, dup := seenApps[id]; dup {
			continue
		}
		seenApps[id] = struct{}{}
		appIds = append(appIds, id)
	}
	if len(appIds) == 0 {
		slog.Warn("create user rejected: admitted application required", "username", request.Username)
		return nil, ErrDataScopeRequired
	}
	// 数据边界（ADR 0008）：非 admin 创建的账号须接入其数据范围应用
	if scopeId, scoped, scopeErr := operatorDataScope(ctx, service.db); scopeErr != nil {
		return nil, scopeErr
	} else if scoped {
		if _, hasScopeApp := seenApps[scopeId]; scopeId <= 0 || !hasScopeApp {
			return nil, ErrAccountOutOfScope
		}
	}
	var admittedApps []*model.Application
	if err := service.db.WithContext(ctx).
		Where(model.ApplicationColumnId+" IN ?", appIds).
		Where(model.ApplicationColumnStatus, model.StatusEnabled).
		Select(model.ApplicationFields).
		Find(&admittedApps).Error; err != nil {
		return nil, err
	}
	if len(admittedApps) != len(appIds) {
		slog.Warn("create user rejected: some applications not found or disabled",
			"username", request.Username, "application_ids", request.ApplicationIds)
		return nil, ErrApplicationInvalid
	}

	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者，无认证上下文时空串（research.md D16）
	operator := cast.ToString(auth.GetAccountId(ctx))
	// email 可空：nil 落库为 NULL，避免空串占用唯一索引位
	var email *string
	if len(request.Email) > 0 {
		e := request.Email
		email = &e
	}
	user := model.User{
		Id:           pkId,
		Username:     request.Username,
		Email:        email,
		Nickname:     request.Nickname,
		PasswordHash: passwordHash,
		Status:       constant.AccountStatusEnabled,
		Version:      1,
		CreatedBy:    operator,
		UpdatedBy:    operator,
		CreateTime:   now,
		UpdateTime:   now,
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		err := tx.Model(model.User{}).
			Where(model.UserColumnUsername, request.Username).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrUsernameDuplicated
		}
		// email 唯一：非空时查重转友好错误（DB 层唯一索引兜底）
		if email != nil {
			var emailCount int64
			err := tx.Model(model.User{}).
				Where(model.UserColumnEmail+" = ?", *email).
				Count(&emailCount).Error
			if err != nil {
				return err
			}
			if emailCount > 0 {
				return ErrEmailDuplicated
			}
		}
		if err := tx.Select(model.UserFields).Create(&user).Error; err != nil {
			return err
		}
		// 可选初始角色绑定：全部校验存在后整批绑定（len>0 才 Append，见历史坑）
		if len(request.RoleIds) > 0 {
			roleIds := make([]int64, 0, len(request.RoleIds))
			seen := make(map[int64]struct{}, len(request.RoleIds))
			for _, raw := range request.RoleIds {
				id := cast.ToInt64(raw)
				if id <= 0 {
					continue
				}
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
				roleIds = append(roleIds, id)
			}
			if len(roleIds) > 0 {
				var roles []model.Role
				if err := tx.Where(model.RoleColumnId+" IN ?", roleIds).
					Select(model.RoleFields).
					Find(&roles).Error; err != nil {
					return err
				}
				if len(roles) != len(roleIds) {
					slog.Warn("create user rejected: some roles not found", "role_ids", request.RoleIds)
					return ErrRoleNotFound
				}
				// 数据边界（ADR 0006 补充）：初始角色绑定与 UpdateAccountRoles 同门槛
				if err := requireRoleBindScope(ctx, tx, roles); err != nil {
					return err
				}
				// 绑定只写关联表（t_r_user_role）；Association.Append 会连带
				// upsert 关联行，stub/零值时间对象将触发 MySQL 严格模式 0000-00-00 报错
				rows := make([]model.RUserRole, 0, len(roleIds))
				user.RoleList = make([]*model.Role, 0, len(roles))
				for i := range roles {
					rows = append(rows, model.RUserRole{UserId: user.Id, RoleId: roles[i].Id})
					user.RoleList = append(user.RoleList, &roles[i])
				}
				if err := tx.Create(&rows).Error; err != nil {
					return err
				}
			}
		}
		// 接入应用绑定只写关联表（t_r_user_application），全量写入。
		if len(admittedApps) > 0 {
			appRows := make([]model.RUserApplication, 0, len(admittedApps))
			for _, app := range admittedApps {
				appRows = append(appRows, model.RUserApplication{UserId: user.Id, ApplicationId: app.Id})
			}
			if err := tx.Create(&appRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("create user failed", "username", request.Username, "error", err)
		return nil, err
	}
	account := accountToPb(user)
	account.Applications = applicationsToPb(admittedApps)
	return account, nil
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
