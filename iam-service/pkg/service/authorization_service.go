package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

func applyKeywordFilter(db *gorm.DB, conditions []string, keyword string) *gorm.DB {
	if len(keyword) == 0 {
		return db
	}
	escapedKeyword := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(keyword)
	likeKeyword := "%" + escapedKeyword + "%"
	return db.Where(strings.Join(conditions, " OR "), likeKeyword, likeKeyword)
}

type AuthorizationService struct {
	db  *gorm.DB
	rdb *redis.Client
}

// loadAccountPermissionSnapshot 读取账号权限快照：缓存命中直接返回，未命中或
// 格式不符走读穿回源重建（与 auth 拦截器共用回源逻辑）。快照内容为纯权限码集合
// （011 权限码模型）。Redis 故障 fail-closed，底层错误仅入日志。
func loadAccountPermissionSnapshot(ctx context.Context, rdb *redis.Client, db *gorm.DB, accountId int64) ([]string, error) {
	str, err := rdb.Get(ctx, constant.GetAccountPermissionKey(accountId)).Result()
	var permKeys []string
	if err == nil {
		err = json.Unmarshal([]byte(str), &permKeys)
		if err == nil {
			return permKeys, nil
		}
		// 格式不符（旧格式残留）视为未命中，走回源重建
		slog.Warn("unmarshal permission snapshot failed, rebuilding from db", "account_id", accountId, "error", err)
	} else if !errors.Is(err, redis.Nil) {
		slog.Error("get permission snapshot from redis failed", "account_id", accountId, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
	}
	permKeys, err = auth.LoadPermissionSnapshot(ctx, rdb, accountId, NewPermissionSnapshotLoader(db))
	if err != nil {
		slog.Error("rebuild permission snapshot from db failed", "account_id", accountId, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
	}
	return permKeys, nil
}

func (a AuthorizationService) CheckPermission(ctx context.Context, request *iam_v1.CheckPermissionRequest) (*iam_v1.CheckPermissionResponse, error) {
	// 权限码是唯一授权词汇（011 裁决）：直接按码匹配快照
	if auth.IsAdminFromContext(ctx) {
		return &iam_v1.CheckPermissionResponse{Ok: true}, nil
	}
	accountId := auth.GetAccountId(ctx)
	permKeys, err := loadAccountPermissionSnapshot(ctx, a.rdb, a.db, accountId)
	if err != nil {
		return nil, err
	}
	for _, key := range permKeys {
		if key == request.PermissionCode {
			return &iam_v1.CheckPermissionResponse{
				Ok: true,
			}, nil
		}
	}
	return &iam_v1.CheckPermissionResponse{
		Ok: false,
	}, nil
}

func permissionToPb(p model.Permission) *iam_v1.Permission {
	return &iam_v1.Permission{
		Id:             cast.ToString(p.Id),
		Name:           p.Name,
		ApplicationId:  p.ApplicationId,
		PermissionCode: p.Code,
		EnabledStatus:  boolToStatus(p.Disabled),
		Version:        p.Version,
		CreatedBy:      cast.ToString(p.CreatedBy),
		UpdatedBy:      cast.ToString(p.UpdatedBy),
		CreateTime:     timestamppb.New(p.CreateTime),
		UpdateTime:     timestamppb.New(p.UpdateTime),
	}
}

func resourceToPb(r model.Resource, application *iam_v1.ApplicationSummary) *iam_v1.Resource {
	codeList := make([]string, 0, len(r.PermissionCodes))
	codeList = append(codeList, r.PermissionCodes...)
	pb := &iam_v1.Resource{
		Id:                 cast.ToString(r.Id),
		Kind:               r.Kind,
		Key:                r.Key,
		Name:               r.Name,
		PermissionCodeList: codeList,
		EnabledStatus:      boolToStatus(r.Disabled),
		CreateTime:         timestamppb.New(r.CreateTime),
		UpdateTime:         timestamppb.New(r.UpdateTime),
		ApplicationId:      r.ApplicationId,
		Version:            r.Version,
		DisplayOrder:       r.DisplayOrder,
		Application:        application,
	}
	if r.ParentId != nil {
		pb.ParentId = *r.ParentId
	}
	return pb
}

func roleToPb(r model.Role, application *iam_v1.ApplicationSummary) *iam_v1.Role {
	response := &iam_v1.Role{
		Id:            cast.ToString(r.Id),
		Name:          r.Name,
		ApplicationId: r.ApplicationId,
		RoleCode:      r.Code,
		EnabledStatus: boolToStatus(r.Disabled),
		Version:       r.Version,
		CreatedBy:     cast.ToString(r.CreatedBy),
		UpdatedBy:     cast.ToString(r.UpdatedBy),
		CreateTime:    timestamppb.New(r.CreateTime),
		UpdateTime:    timestamppb.New(r.UpdateTime),
		PermissionCodeList: func() []string {
			codes := make([]string, 0, len(r.PermissionList))
			for _, permission := range r.PermissionList {
				codes = append(codes, permission.Code)
			}
			return codes
		}(),
	}
	response.Application = application
	response.PermissionList = func() []*iam_v1.PermissionSummary {
		summaries := make([]*iam_v1.PermissionSummary, 0, len(r.PermissionList))
		for _, permission := range r.PermissionList {
			summaries = append(summaries, &iam_v1.PermissionSummary{
				Id:             permission.Id,
				Name:           permission.Name,
				PermissionCode: permission.Code,
				EnabledStatus:  boolToStatus(permission.Disabled),
			})
		}
		return summaries
	}()
	return response
}

func (a AuthorizationService) CreatePermission(ctx context.Context, request *iam_v1.CreatePermissionRequest) (*iam_v1.Permission, error) {
	// 权限码格式校验（011 裁决：服务.模块.操作，如 iam.account.delete）
	if !model.IsValidPermissionCode(request.Code) {
		return nil, ErrInvalidPermissionCode
	}
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者
	operator := auth.GetAccountId(ctx)
	permission := model.Permission{
		Id:            pkId,
		Name:          request.Name,
		Code:          request.Code,
		Description:   request.Description,
		ApplicationId: request.ApplicationId,
		Disabled:      false,
		CreateTime:    now,
		UpdateTime:    now,
		CreatedBy:     operator,
		UpdatedBy:     operator,
	}
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(model.Permission{}).
			Where(model.PermissionColumnCode, request.Code).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrPermissionCodeDuplicated
		}
		return tx.Model(model.Permission{}).Create(&permission).Error
	})
	if err != nil {
		slog.Warn("create permission failed", "permission_code", request.Code, "error", err)
		return nil, err
	}
	var created model.Permission
	if err := a.db.WithContext(ctx).
		Select(model.PermissionFields).
		Where(model.PermissionColumnId, pkId).First(&created).Error; err != nil {
		return nil, err
	}
	return permissionToPb(created), nil
}

func (a AuthorizationService) GetPermission(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Permission, error) {
	var permission model.Permission
	err := a.db.WithContext(ctx).
		Select(model.PermissionFields).
		Where(model.PermissionColumnId, request.Id).First(&permission).Error
	if err != nil {
		return nil, err
	}
	return permissionToPb(permission), nil
}

func (a AuthorizationService) ListPermissions(ctx context.Context, request *iam_v1.ListPermissionsRequest) (*iam_v1.ListPermissionsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.PermissionColumnName + " LIKE ?",
		model.PermissionColumnCode + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		return applyKeywordFilter(a.db.WithContext(ctx).Model(model.Permission{}), keywordConditions, request.Keyword)
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var permissions []model.Permission
	err = buildQuery().
		Order(model.PermissionColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&permissions).Error
	if err != nil {
		return nil, err
	}
	permList := make([]*iam_v1.Permission, 0, len(permissions))
	for _, p := range permissions {
		permList = append(permList, permissionToPb(p))
	}
	return &iam_v1.ListPermissionsResponse{
		Permissions: permList,
		Page:        helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a AuthorizationService) UpdatePermission(ctx context.Context, request *iam_v1.UpdatePermissionRequest) (*emptypb.Empty, error) {
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.PermissionColumnName] = request.Name
	}
	if len(request.EnabledStatus) > 0 {
		updates[model.PermissionColumnDisabled] = statusToBool(request.EnabledStatus)
	}
	updates[model.PermissionColumnUpdateTime] = time.Now()
	// 审计字段服务端注入：仅更新修改人与修改时间
	updates[model.PermissionColumnUpdatedBy] = auth.GetAccountId(ctx)
	updates[model.PermissionColumnVersion] = gorm.Expr(model.PermissionColumnVersion + " + 1")

	changedStatus := len(request.EnabledStatus) > 0
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(model.Permission{}).
			Where(model.PermissionColumnId, request.Id).
			Where(model.PermissionColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUpdatePermissionBusy
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrUpdatePermissionBusy) {
			slog.Warn("update permission rejected: version conflict", "permission_id", request.Id, "version", request.Version)
			return nil, err
		}
		slog.Warn("update permission failed", "permission_id", request.Id, "version", request.Version, "error", err)
		return nil, err
	}
	// 禁用状态变更立即生效：失效引用该权限账号的权限快照
	if changedStatus {
		a.invalidatePermissionAccountsSnapshot(ctx, request.Id)
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) CreateRole(ctx context.Context, request *iam_v1.CreateRoleRequest) (*iam_v1.Role, error) {
	pkId := krand.NewID()
	now := time.Now()
	if request.ApplicationId <= 0 {
		return nil, ErrApplicationRequired
	}
	var application model.Application
	if err := a.db.WithContext(ctx).
		Where(model.ApplicationColumnId, request.ApplicationId).
		Select(model.ApplicationFields).First(&application).Error; err != nil {
		return nil, ErrApplicationNotFound
	}
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者
	operator := auth.GetAccountId(ctx)
	role := model.Role{
		Id:            pkId,
		ApplicationId: request.ApplicationId,
		Name:          request.Name,
		Code:          request.RoleCode,
		Disabled:      false,
		CreateTime:    now,
		UpdateTime:    now,
		CreatedBy:     operator,
		UpdatedBy:     operator,
	}
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(model.Role{}).
			Where(model.RoleColumnCode, request.RoleCode).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrRoleCodeDuplicated
		}
		return tx.Model(model.Role{}).Create(&role).Error
	})
	if err != nil {
		slog.Warn("create role failed", "role_code", request.RoleCode, "error", err)
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{role.ApplicationId})
	if err != nil {
		return nil, err
	}
	return roleToPb(role, summaries[role.ApplicationId]), nil
}

func (a AuthorizationService) GetRole(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Role, error) {
	var role model.Role
	err := a.db.WithContext(ctx).
		Select(model.RoleFields).
		Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	if err := loadRolePermissions(ctx, a.db, &role); err != nil {
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{role.ApplicationId})
	if err != nil {
		return nil, err
	}
	return roleToPb(role, summaries[role.ApplicationId]), nil
}

func (a AuthorizationService) ListRoles(ctx context.Context, request *iam_v1.ListRolesRequest) (*iam_v1.ListRolesResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.RoleColumnName + " LIKE ?",
		model.RoleColumnCode + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		return applyKeywordFilter(a.db.WithContext(ctx).Model(model.Role{}), keywordConditions, request.Keyword)
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var roles []model.Role
	err = buildQuery().
		Order(model.RoleColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	if err := loadRolePermissionsForRoles(ctx, a.db, roles); err != nil {
		return nil, err
	}
	applicationIds := make([]int64, 0, len(roles))
	for _, role := range roles {
		applicationIds = append(applicationIds, role.ApplicationId)
	}
	roleApplications, err := loadApplicationSummaries(ctx, a.db, applicationIds)
	if err != nil {
		return nil, err
	}
	roleList := make([]*iam_v1.Role, 0, len(roles))
	for _, r := range roles {
		roleList = append(roleList, roleToPb(r, roleApplications[r.ApplicationId]))
	}
	return &iam_v1.ListRolesResponse{
		Roles: roleList,
		Page:  helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a AuthorizationService) UpdateRole(ctx context.Context, request *iam_v1.UpdateRoleRequest) (*emptypb.Empty, error) {
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.RoleColumnName] = request.Name
	}
	if len(request.EnabledStatus) > 0 {
		updates[model.RoleColumnDisabled] = statusToBool(request.EnabledStatus)
	}
	updates[model.RoleColumnUpdateTime] = time.Now()
	// 审计字段服务端注入：仅更新修改人与修改时间
	updates[model.RoleColumnUpdatedBy] = auth.GetAccountId(ctx)
	updates[model.RoleColumnVersion] = gorm.Expr(model.RoleColumnVersion + " + 1")
	resp, err := a.updateRoleInTransaction(ctx, request, updates)
	if err != nil {
		return nil, err
	}
	// 权限集合增减或禁用状态变更立即生效：失效绑定账号的权限快照
	if len(request.AddedPermissionCodeList) > 0 || len(request.RemovedPermissionCodeList) > 0 || len(request.EnabledStatus) > 0 {
		a.invalidateRoleAccountsSnapshot(ctx, request.Id)
	}
	return resp, nil
}

// invalidateRoleAccountsSnapshot 删除绑定该角色的全部账号的权限快照键。
// 快照为派生数据：删除失败仅记日志，由短 TTL 兜底，不影响已提交的业务变更。
func (a AuthorizationService) invalidateRoleAccountsSnapshot(ctx context.Context, roleId int64) {
	if a.rdb == nil {
		return
	}
	var accountIds []int64
	err := a.db.WithContext(ctx).Model(model.RAccountRole{}).
		Where(model.RAccountRoleColumnRoleId, roleId).
		Pluck(model.RAccountRoleColumnAccountId, &accountIds).Error
	if err != nil {
		slog.Warn("query role bound accounts failed", "role_id", roleId, "error", err)
		return
	}
	for _, accountId := range accountIds {
		a.deleteAccountPermissionSnapshot(ctx, accountId, roleId)
	}
}

// invalidatePermissionAccountsSnapshot 删除引用该权限（经任意角色绑定）的
// 全部账号的权限快照键。
func (a AuthorizationService) invalidatePermissionAccountsSnapshot(ctx context.Context, permissionId int64) {
	if a.rdb == nil {
		return
	}
	var accountIds []int64
	err := a.db.WithContext(ctx).Raw(
		"SELECT DISTINCT ar.account_id FROM t_r_account_role ar "+
			"JOIN t_r_role_permission rp ON rp.role_id = ar.role_id "+
			"WHERE rp.permission_id = ?", permissionId).Scan(&accountIds).Error
	if err != nil {
		slog.Warn("query permission bound accounts failed", "permission_id", permissionId, "error", err)
		return
	}
	for _, accountId := range accountIds {
		a.deleteAccountPermissionSnapshot(ctx, accountId, permissionId)
	}
}

// deleteAccountPermissionSnapshot 删除单个账号的权限快照键，失败仅记日志（派生数据兜底可重建）。
func (a AuthorizationService) deleteAccountPermissionSnapshot(ctx context.Context, accountId, sourceId int64) {
	if err := a.rdb.Del(ctx, constant.GetAccountPermissionKey(accountId)).Err(); err != nil {
		slog.Warn("invalidate account permission snapshot failed",
			"account_id", accountId, "source_id", sourceId, "error", err)
	}
}

func (a AuthorizationService) updateRoleInTransaction(ctx context.Context, request *iam_v1.UpdateRoleRequest, updates map[string]interface{}) (*emptypb.Empty, error) {
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.Where(model.RoleColumnId, request.Id).Select(model.RoleFields).First(&role).Error; err != nil {
			return err
		}
		result := tx.Model(model.Role{}).
			Where(model.RoleColumnId, request.Id).
			Where(model.RoleColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUpdateRoleBusy
		}

		if len(request.AddedPermissionCodeList) == 0 && len(request.RemovedPermissionCodeList) == 0 {
			return nil
		}
		permissionCodes := make([]string, 0, len(request.AddedPermissionCodeList)+len(request.RemovedPermissionCodeList))
		permissionCodes = append(permissionCodes, request.AddedPermissionCodeList...)
		permissionCodes = append(permissionCodes, request.RemovedPermissionCodeList...)
		var permissions []model.Permission
		if err := tx.Where(model.PermissionColumnCode+" IN ?", permissionCodes).Select(model.PermissionFields).Find(&permissions).Error; err != nil {
			return err
		}
		if len(permissions) != len(permissionCodes) {
			return ErrPermissionNotFound
		}

		permissionByCode := make(map[string]model.Permission, len(permissions))
		for _, permission := range permissions {
			permissionByCode[permission.Code] = permission
		}
		added := make([]model.Permission, 0, len(request.AddedPermissionCodeList))
		for _, code := range request.AddedPermissionCodeList {
			added = append(added, permissionByCode[code])
		}
		removed := make([]model.Permission, 0, len(request.RemovedPermissionCodeList))
		for _, code := range request.RemovedPermissionCodeList {
			removed = append(removed, permissionByCode[code])
		}
		association := tx.Model(&role).Association("PermissionList")
		if err := association.Append(added); err != nil {
			return err
		}
		if len(removed) > 0 {
			return association.Delete(removed)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPermissionNotFound) {
			slog.Warn("update role rejected: permission code not found",
				"role_id", request.Id,
				"added_permission_codes", request.AddedPermissionCodeList,
				"removed_permission_codes", request.RemovedPermissionCodeList)
		} else {
			slog.Warn("commit role update failed",
				"role_id", request.Id, "version", request.Version, "error", err)
		}
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) DeletePermission(ctx context.Context, request *iam_v1.DeletePermissionRequest) (*emptypb.Empty, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("t_r_role_permission").
		Where("permission_id = ?", request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check permission references failed", "permission_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete permission rejected: referenced by roles", "permission_id", request.Id, "reference_count", count)
		return nil, ErrPermissionReferenced
	}

	result := a.db.WithContext(ctx).Where(model.PermissionColumnId, request.Id).Delete(model.Permission{})
	if result.Error != nil {
		slog.Warn("delete permission failed", "permission_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete permission rejected: permission not found", "permission_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) DeleteRole(ctx context.Context, request *iam_v1.DeleteRoleRequest) (*emptypb.Empty, error) {
	// 管理员角色为系统内置，禁止删除
	var role model.Role
	if err := a.db.WithContext(ctx).Where(model.RoleColumnId, request.Id).Select(model.RoleFields).First(&role).Error; err != nil {
		slog.Warn("delete role rejected: role not found", "role_id", request.Id, "error", err)
		return nil, err
	}
	if role.Code == model.RoleAdmin {
		slog.Warn("delete role rejected: builtin admin role", "role_id", request.Id)
		return nil, ErrRoleAdminProtected
	}

	var count int64
	err := a.db.WithContext(ctx).Model(model.RAccountRole{}).
		Where(model.RAccountRoleColumnRoleId, request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check role bindings failed", "role_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete role rejected: bound by accounts", "role_id", request.Id, "binding_count", count)
		return nil, ErrRoleReferenced
	}

	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM t_r_role_permission WHERE role_id = ?", request.Id).Error; err != nil {
			return err
		}
		result := tx.Where(model.RoleColumnId, request.Id).Delete(model.Role{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		slog.Warn("delete role failed", "role_id", request.Id, "error", err)
		return nil, err
	}
	// 级联清理权限关联立即生效：失效（残余）绑定账号的权限快照
	a.invalidateRoleAccountsSnapshot(ctx, request.Id)
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) ListResources(ctx context.Context, request *iam_v1.ListResourcesRequest) (*iam_v1.ListResourcesResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		// key 是 MySQL 保留字，拼入原生 SQL 必须反引号包裹
		"`key` LIKE ?",
		model.ResourceColumnName + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		query := applyKeywordFilter(a.db.WithContext(ctx).Model(model.Resource{}), keywordConditions, request.Keyword)
		if len(request.Kind) > 0 {
			query = query.Where(model.ResourceColumnKind, request.Kind)
		}
		if request.ApplicationId > 0 {
			query = query.Where(model.ResourceColumnApplicationId, request.ApplicationId)
		}
		return query
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var resources []model.Resource
	err = buildQuery().
		Select(model.ResourceFields).
		Order(model.ResourceColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&resources).Error
	if err != nil {
		return nil, err
	}
	resourceList := make([]*iam_v1.Resource, 0, len(resources))
	for _, r := range resources {
		resourceList = append(resourceList, resourceToPb(r, nil))
	}
	return &iam_v1.ListResourcesResponse{
		Resources: resourceList,
		Page:      helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

// validateComponentInput 校验组件类型与绑定权限码格式：
// kind 须为 menu/page/button；绑定码须为合法权限码（服务.模块.操作）。
func validateComponentInput(kind string, permissionCodes []string) error {
	if !model.IsValidComponentKind(kind) {
		return ErrInvalidComponentKind
	}
	for _, code := range permissionCodes {
		if !model.IsValidPermissionCode(code) {
			return ErrInvalidPermissionCode
		}
	}
	return nil
}

func (a AuthorizationService) CreateResource(ctx context.Context, request *iam_v1.CreateResourceRequest) (*iam_v1.Resource, error) {
	if request.ApplicationId <= 0 {
		return nil, ErrResourceApplicationRequired
	}
	if err := validateComponentInput(request.Kind, request.PermissionCodeList); err != nil {
		return nil, err
	}
	now := time.Now()
	operator := auth.GetAccountId(ctx)
	resource := model.Resource{
		Id:              krand.NewID(),
		ApplicationId:   request.ApplicationId,
		Kind:            request.Kind,
		Key:             request.Key,
		Name:            request.Name,
		PermissionCodes: append([]string{}, request.PermissionCodeList...),
		Disabled:        request.Disabled,
		DisplayOrder:    request.DisplayOrder,
		CreateTime:      now,
		UpdateTime:      now,
		CreatedBy:       operator,
		UpdatedBy:       operator,
		Version:         1,
	}
	if request.ParentId > 0 {
		resource.ParentId = &request.ParentId
	}
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var app model.Application
		if err := tx.Where(model.ApplicationColumnId, request.ApplicationId).
			Select(model.ApplicationFields).First(&app).Error; err != nil {
			return ErrResourceApplicationInvalid
		}
		var count int64
		if err := tx.Model(model.Resource{}).
			Where(model.ResourceColumnApplicationId, request.ApplicationId).
			Where(model.ResourceColumnKey, request.Key).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrResourceKeyDuplicated
		}
		if request.ParentId > 0 {
			if err := validateParent(ctx, tx, request.ParentId, request.ApplicationId, 0); err != nil {
				return err
			}
		}
		return tx.Create(&resource).Error
	})
	if err != nil {
		slog.Warn("create resource failed", "key", request.Key, "error", err)
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{resource.ApplicationId})
	if err != nil {
		return nil, err
	}
	return resourceToPb(resource, summaries[resource.ApplicationId]), nil
}

func (a AuthorizationService) GetResource(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Resource, error) {
	var resource model.Resource
	if err := a.db.WithContext(ctx).
		Select(model.ResourceFields).
		Where(model.ResourceColumnId, request.Id).First(&resource).Error; err != nil {
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{resource.ApplicationId})
	if err != nil {
		return nil, err
	}
	return resourceToPb(resource, summaries[resource.ApplicationId]), nil
}

func (a AuthorizationService) UpdateResource(ctx context.Context, request *iam_v1.UpdateResourceRequest) (*iam_v1.Resource, error) {
	updates := map[string]interface{}{
		model.ResourceColumnUpdateTime: time.Now(),
		model.ResourceColumnUpdatedBy:  auth.GetAccountId(ctx),
		model.ResourceColumnVersion:    gorm.Expr(model.ResourceColumnVersion + " + 1"),
	}
	if len(request.Name) > 0 {
		updates[model.ResourceColumnName] = request.Name
	}
	// permission_code_list 替换式更新：未提供（nil）不动，提供空清单即解绑全部权限码
	if request.PermissionCodeList != nil {
		for _, code := range request.PermissionCodeList {
			if !model.IsValidPermissionCode(code) {
				return nil, ErrInvalidPermissionCode
			}
		}
	}
	updates[model.ResourceColumnDisabled] = request.Disabled
	if request.DisplayOrder != 0 {
		updates[model.ResourceColumnDisplayOrder] = request.DisplayOrder
	}

	var parentId *int64
	if request.ParentCleared {
		parentId = nil
	} else if request.ParentId > 0 {
		parentId = &request.ParentId
	}

	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var resource model.Resource
		if err := tx.Where(model.ResourceColumnId, request.Id).
			Select(model.ResourceFields).First(&resource).Error; err != nil {
			return err
		}
		if parentId != nil {
			if err := validateParent(ctx, tx, *parentId, resource.ApplicationId, resource.Id); err != nil {
				return err
			}
		}
		updates[model.ResourceColumnParentId] = parentId
		result := tx.Model(model.Resource{}).
			Where(model.ResourceColumnId, request.Id).
			Where(model.ResourceColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrResourceUpdateBusy
		}
		// permission_codes 走 struct Updates（gorm serializer 对 map 更新不生效）
		if request.PermissionCodeList != nil {
			if err := tx.Model(&model.Resource{}).
				Where(model.ResourceColumnId, request.Id).
				Select(model.ResourceColumnPermissionCodes).
				Updates(&model.Resource{PermissionCodes: append([]string{}, request.PermissionCodeList...)}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("update resource failed", "id", request.Id, "error", err)
		return nil, err
	}
	return a.GetResource(ctx, &common_v1.GetRequest{Id: request.Id})
}

func (a AuthorizationService) DeleteResource(ctx context.Context, request *iam_v1.DeleteResourceRequest) (*emptypb.Empty, error) {
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var resource model.Resource
		if err := tx.Where(model.ResourceColumnId, request.Id).
			Select(model.ResourceFields).First(&resource).Error; err != nil {
			return err
		}
		var childCount int64
		if err := tx.Model(model.Resource{}).
			Where(model.ResourceColumnParentId, request.Id).
			Count(&childCount).Error; err != nil {
			return err
		}
		if childCount > 0 {
			return ErrResourceHasChildren
		}
		result := tx.Where(model.ResourceColumnId, request.Id).Delete(model.Resource{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		slog.Warn("delete resource failed", "id", request.Id, "error", err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) ListResourceTree(ctx context.Context, request *iam_v1.ListResourceTreeRequest) (*iam_v1.ListResourceTreeResponse, error) {
	var resources []model.Resource
	if err := a.db.WithContext(ctx).
		Where(model.ResourceColumnApplicationId, request.ApplicationId).
		Select(model.ResourceFields).
		Order(model.ResourceColumnDisplayOrder + " ASC, " + model.ResourceColumnId + " ASC").
		Find(&resources).Error; err != nil {
		return nil, err
	}
	appSummaries, err := loadApplicationSummaries(ctx, a.db, []int64{request.ApplicationId})
	if err != nil {
		return nil, err
	}
	resourceById := make(map[int64]*model.Resource, len(resources))
	for i := range resources {
		resourceById[resources[i].Id] = &resources[i]
	}
	var roots []*iam_v1.ResourceTreeNode
	for _, r := range resources {
		if r.ParentId == nil {
			roots = append(roots, buildResourceTreeNode(r, resourceById, appSummaries[request.ApplicationId]))
		}
	}
	return &iam_v1.ListResourceTreeResponse{Roots: roots}, nil
}

// ListMyMenuTree 返回当前用户在指定应用下可见的组件树（menu/page/button 全量节点，
// 前端按 kind 决定渲染方式）。可见性由服务端裁决：组件未绑定权限码时全员可见；
// 绑定了权限码时，账号权限码集合与绑定码存在交集即可见。命中节点的祖先路径一并返回，
// 保证树可渲染；应用不存在或已停用返回业务错误；空清单返回空 roots。
func (a AuthorizationService) ListMyMenuTree(ctx context.Context, request *iam_v1.ListMyMenuTreeRequest) (*iam_v1.ListResourceTreeResponse, error) {
	var application model.Application
	err := a.db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnId, request.ApplicationId).
		First(&application).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		slog.Warn("get application for menu tree failed", "application_id", request.ApplicationId, "error", err)
		return nil, err
	}
	if application.Disabled == 1 {
		slog.Warn("list my menu tree rejected: application disabled", "application_id", request.ApplicationId)
		return nil, ErrApplicationDisabled
	}

	// 加载该应用的全部组件资源（display_order 排序）
	var resources []model.Resource
	if err := a.db.WithContext(ctx).
		Where(model.ResourceColumnApplicationId, request.ApplicationId).
		Select(model.ResourceFields).
		Order(model.ResourceColumnDisplayOrder + " ASC, " + model.ResourceColumnId + " ASC").
		Find(&resources).Error; err != nil {
		slog.Warn("load component resources failed", "application_id", request.ApplicationId, "error", err)
		return nil, err
	}

	codes, err := a.loadAccountPermissionCodes(ctx)
	if err != nil {
		return nil, err
	}
	visible := make(map[int64]struct{}, len(resources))
	byId := make(map[int64]model.Resource, len(resources))
	for _, r := range resources {
		byId[r.Id] = r
		// 可见性：未绑定权限码的组件全员可见；绑定了则按码交集判定
		if len(r.PermissionCodes) == 0 {
			visible[r.Id] = struct{}{}
			continue
		}
		for _, code := range r.PermissionCodes {
			if _, ok := codes[code]; ok {
				visible[r.Id] = struct{}{}
				break
			}
		}
	}
	// 命中节点的祖先路径一并返回，保证树可渲染
	for _, r := range resources {
		if _, ok := visible[r.Id]; !ok {
			continue
		}
		for parent := r.ParentId; parent != nil; parent = byId[*parent].ParentId {
			if _, seen := visible[*parent]; seen {
				break
			}
			// 祖先可能不在查询结果中（数据异常），缺失即止
			node, ok := byId[*parent]
			if !ok {
				break
			}
			visible[node.Id] = struct{}{}
		}
	}

	byParent := make(map[int64][]model.Resource, len(resources))
	var rootCandidates []model.Resource
	for _, r := range resources {
		if _, ok := visible[r.Id]; !ok {
			continue
		}
		if r.ParentId == nil {
			rootCandidates = append(rootCandidates, r)
		} else {
			byParent[*r.ParentId] = append(byParent[*r.ParentId], r)
		}
	}

	appSummaries, err := loadApplicationSummaries(ctx, a.db, []int64{request.ApplicationId})
	if err != nil {
		return nil, err
	}
	summary := appSummaries[request.ApplicationId]
	roots := make([]*iam_v1.ResourceTreeNode, 0, len(rootCandidates))
	for _, r := range rootCandidates {
		roots = append(roots, buildMyMenuNode(r, byParent, summary))
	}
	return &iam_v1.ListResourceTreeResponse{Roots: roots}, nil
}

// buildMyMenuNode 从可见集合构建组件树节点（children 仅含过滤后仍保留的节点）
func buildMyMenuNode(r model.Resource, byParent map[int64][]model.Resource, appSummary *iam_v1.ApplicationSummary) *iam_v1.ResourceTreeNode {
	node := &iam_v1.ResourceTreeNode{
		Resource: resourceToPb(r, appSummary),
		Children: []*iam_v1.ResourceTreeNode{},
	}
	for _, child := range byParent[r.Id] {
		node.Children = append(node.Children, buildMyMenuNode(child, byParent, appSummary))
	}
	return node
}

// loadAccountPermissionCodes 读取账号权限码集合。快照缺失时读穿回源重建（同一实现路径）。
func (a AuthorizationService) loadAccountPermissionCodes(ctx context.Context) (map[string]struct{}, error) {
	accountId := auth.GetAccountId(ctx)
	if a.rdb != nil {
		raw, err := a.rdb.Get(ctx, constant.GetAccountPermissionKey(accountId)).Result()
		if err == nil {
			var keys []string
			if err := json.Unmarshal([]byte(raw), &keys); err != nil {
				slog.Warn("unmarshal permission snapshot failed", "account_id", accountId, "error", err)
				return nil, err
			}
			codeDict := make(map[string]struct{}, len(keys))
			for _, key := range keys {
				codeDict[key] = struct{}{}
			}
			return codeDict, nil
		}
		if !errors.Is(err, redis.Nil) {
			slog.Warn("get permission snapshot failed", "account_id", accountId, "error", err)
			return nil, err
		}
	}
	// 快照缺失：读穿回源
	keys, err := NewPermissionSnapshotLoader(a.db)(ctx, accountId)
	if err != nil {
		slog.Warn("load permission snapshot read-through failed", "account_id", accountId, "error", err)
		return nil, err
	}
	codeDict := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		codeDict[key] = struct{}{}
	}
	return codeDict, nil
}

// validateParent checks that the parent exists, belongs to the same application, and
// that placing the child under it would not create a cycle or exceed max depth.
// It loads all resources for the application once and walks in memory.
func validateParent(ctx context.Context, tx *gorm.DB, parentId, applicationId, childId int64) error {
	var resources []model.Resource
	if err := tx.Where(model.ResourceColumnApplicationId, applicationId).
		Select(model.ResourceFields).Find(&resources).Error; err != nil {
		return err
	}
	byId := make(map[int64]model.Resource, len(resources))
	for _, r := range resources {
		byId[r.Id] = r
	}
	parent, ok := byId[parentId]
	if !ok {
		return ErrResourceParentInvalid
	}
	if parent.ApplicationId != applicationId {
		return ErrResourceParentInvalid
	}
	// Cycle detection: walk from parent to root; if we encounter childId, it's a cycle
	current := &parent
	visited := make(map[int64]bool)
	visited[parent.Id] = true
	for current.ParentId != nil {
		if *current.ParentId == childId && childId != 0 {
			return ErrResourceHierarchyCircular
		}
		grandParent, exists := byId[*current.ParentId]
		if !exists {
			break
		}
		if visited[grandParent.Id] {
			break
		}
		visited[grandParent.Id] = true
		current = &grandParent
	}
	// Depth check: compute parent's depth and ensure child would stay within 5 levels
	parentDepth := 1
	walker := &parent
	for walker.ParentId != nil {
		parentDepth++
		p, exists := byId[*walker.ParentId]
		if !exists {
			break
		}
		walker = &p
	}
	// childDepth = parentDepth + 1 (child goes under parent)
	if parentDepth >= 5 {
		return ErrResourceHierarchyDepthExceeded
	}
	return nil
}

func buildResourceTreeNode(r model.Resource, byId map[int64]*model.Resource, appSummary *iam_v1.ApplicationSummary) *iam_v1.ResourceTreeNode {
	node := &iam_v1.ResourceTreeNode{
		Resource: resourceToPb(r, appSummary),
		Children: []*iam_v1.ResourceTreeNode{},
	}
	for _, child := range byId {
		if child.ParentId != nil && *child.ParentId == r.Id {
			node.Children = append(node.Children, buildResourceTreeNode(*child, byId, appSummary))
		}
	}
	return node
}

func (a AuthorizationService) CreateSubscription(ctx context.Context, request *iam_v1.CreateSubscriptionRequest) (*iam_v1.Subscription, error) {
	return &iam_v1.Subscription{}, nil
}

func (a AuthorizationService) GetSubscription(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Subscription, error) {
	return &iam_v1.Subscription{}, nil
}

func (a AuthorizationService) UpdateSubscription(ctx context.Context, request *iam_v1.UpdateSubscriptionRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) ListSubscriptions(ctx context.Context, request *iam_v1.ListSubscriptionsRequest) (*iam_v1.ListSubscriptionsResponse, error) {
	return &iam_v1.ListSubscriptionsResponse{
		Subscriptions: []*iam_v1.Subscription{},
		Page:          &common_v1.Page{},
	}, nil
}

func (a AuthorizationService) GetMyPermissions(ctx context.Context, _ *emptypb.Empty) (*iam_v1.GetMyPermissionsResponse, error) {
	// admin 与普通账号统一走快照真实数据：前端渲染数据与 DB 授权一致
	accountId := auth.GetAccountId(ctx)
	permKeys, err := loadAccountPermissionSnapshot(ctx, a.rdb, a.db, accountId)
	if err != nil {
		return nil, err
	}
	codeList := make([]string, 0, len(permKeys))
	codeList = append(codeList, permKeys...)
	return &iam_v1.GetMyPermissionsResponse{
		PermissionCodeList: codeList,
	}, nil
}

func boolToStatus(disabled bool) string {
	if disabled {
		return "disabled"
	}
	return "enabled"
}

func statusToBool(status string) bool {
	return status == "disabled"
}

var _ iam_v1.AuthorizationServiceServer = (*AuthorizationService)(nil)

func NewAuthorizationService(db *gorm.DB, rdb *redis.Client) iam_v1.AuthorizationServiceServer {
	return &AuthorizationService{db: db, rdb: rdb}
}
