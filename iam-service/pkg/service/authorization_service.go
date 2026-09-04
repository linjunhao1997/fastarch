package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

type AuthorizationService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func (a AuthorizationService) CheckPermission(ctx context.Context, request *iam_v1.CheckPermissionRequest) (*iam_v1.CheckPermissionResponse, error) {
	accountId := auth.GetAccountId(ctx)
	str, err := a.rdb.Get(ctx, constant.GetAccountPermissionKey(accountId)).Result()
	var permCodes []string
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			// Redis 其他故障 fail-closed，底层错误仅入日志（research.md D12/D13）
			slog.Error("get permission snapshot from redis failed", "account_id", accountId, "error", err)
			return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
		}
		// 缓存未命中：读穿回源（与 auth 拦截器共用回源逻辑，research.md D6-②）
		permCodes, err = auth.LoadPermissionSnapshot(ctx, a.rdb, accountId, NewPermissionSnapshotLoader(a.db))
		if err != nil {
			slog.Error("rebuild permission snapshot from db failed", "account_id", accountId, "error", err)
			return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
		}
	} else if err = json.Unmarshal([]byte(str), &permCodes); err != nil {
		slog.Error("unmarshal permission snapshot failed", "account_id", accountId, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
	}

	for _, perm := range permCodes {
		if perm == constant.ApplicationCode+":"+request.PermissionCode {
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

func roleToPb(r model.Role) *iam_v1.Role {
	return &iam_v1.Role{
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
}

func (a AuthorizationService) CreatePermission(ctx context.Context, request *iam_v1.CreatePermissionRequest) (*iam_v1.Permission, error) {
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者，无认证上下文时 0（research.md D16）
	operator := auth.GetAccountId(ctx)
	permission := model.Permission{
		Id:            pkId,
		Name:          request.Name,
		Code:          request.Code,
		Value:         request.Value,
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
	return permissionToPb(permission), nil
}

func (a AuthorizationService) GetPermission(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Permission, error) {
	var permission model.Permission
	err := a.db.WithContext(ctx).Where(model.PermissionColumnId, request.Id).First(&permission).Error
	if err != nil {
		return nil, err
	}
	return permissionToPb(permission), nil
}

func (a AuthorizationService) ListPermissions(ctx context.Context, request *iam_v1.ListPermissionsRequest) (*iam_v1.ListPermissionsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	var count int64
	var permissions []model.Permission
	err := a.db.WithContext(ctx).Model(model.Permission{}).Count(&count).
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
	// 审计字段服务端注入：仅更新修改人与修改时间（research.md D16，宪法 XII）
	updates[model.PermissionColumnUpdatedBy] = auth.GetAccountId(ctx)
	updates[model.PermissionColumnVersion] = gorm.Expr(model.PermissionColumnVersion + " + 1")
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
	}
	result := a.db.WithContext(ctx).Model(model.Permission{}).
		Where(model.PermissionColumnId, request.Id).
		Where(model.PermissionColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		slog.Warn("update permission failed", "permission_id", request.Id, "version", request.Version, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := a.db.WithContext(ctx).Model(model.Permission{}).
			Where(model.PermissionColumnId, request.Id).
			Count(&count).Error; err != nil {
			slog.Warn("check permission existence failed", "permission_id", request.Id, "error", err)
			return nil, err
		}
		if count == 0 {
			slog.Warn("update permission rejected: permission not found", "permission_id", request.Id)
			return nil, gorm.ErrRecordNotFound
		}
		slog.Warn("update permission rejected: version conflict", "permission_id", request.Id, "version", request.Version)
		return nil, ErrUpdatePermissionBusy
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) CreateRole(ctx context.Context, request *iam_v1.CreateRoleRequest) (*iam_v1.Role, error) {
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者，无认证上下文时 0（research.md D16）
	operator := auth.GetAccountId(ctx)
	role := model.Role{
		Id:         pkId,
		Name:       request.Name,
		Code:       request.RoleCode,
		Disabled:   false,
		CreateTime: now,
		UpdateTime: now,
		CreatedBy:  operator,
		UpdatedBy:  operator,
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
	return roleToPb(role), nil
}

func (a AuthorizationService) GetRole(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Role, error) {
	var role model.Role
	err := a.db.WithContext(ctx).Preload("PermissionList").
		Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	return roleToPb(role), nil
}

func (a AuthorizationService) ListRoles(ctx context.Context, request *iam_v1.ListRolesRequest) (*iam_v1.ListRolesResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	var count int64
	var roles []model.Role
	err := a.db.WithContext(ctx).Model(model.Role{}).Count(&count).
		Order(model.RoleColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Preload("PermissionList").
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	roleList := make([]*iam_v1.Role, 0, len(roles))
	for _, r := range roles {
		roleList = append(roleList, roleToPb(r))
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
	// 审计字段服务端注入：仅更新修改人与修改时间（research.md D16，宪法 XII）
	updates[model.RoleColumnUpdatedBy] = auth.GetAccountId(ctx)
	updates[model.RoleColumnVersion] = gorm.Expr(model.RoleColumnVersion + " + 1")
	resp, err := a.updateRoleInTransaction(ctx, request, updates)
	if err != nil {
		return nil, err
	}
	// 权限集合增减或禁用状态变更立即生效：失效绑定账号的权限快照（research.md D6-①）
	if len(request.AddedPermissionCodeList) > 0 || len(request.RemovedPermissionCodeList) > 0 || len(request.EnabledStatus) > 0 {
		a.invalidateRoleAccountsSnapshot(ctx, request.Id)
	}
	return resp, nil
}

// invalidateRoleAccountsSnapshot 删除绑定该角色的全部账号的权限快照键（research.md D6-①）。
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
		if err := a.rdb.Del(ctx, constant.GetAccountPermissionKey(accountId)).Err(); err != nil {
			slog.Warn("invalidate account permission snapshot failed",
				"account_id", accountId, "role_id", roleId, "error", err)
		}
	}
}

func (a AuthorizationService) updateRoleInTransaction(ctx context.Context, request *iam_v1.UpdateRoleRequest, updates map[string]interface{}) (*emptypb.Empty, error) {
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.Where(model.RoleColumnId, request.Id).First(&role).Error; err != nil {
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
		if err := tx.Where(model.PermissionColumnCode+" IN ?", permissionCodes).Find(&permissions).Error; err != nil {
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

	result := a.db.WithContext(ctx).
		Where(model.PermissionColumnId, request.Id).
		Delete(model.Permission{})
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
	// 级联清除权限关联立即生效：失效（残余）绑定账号的权限快照（research.md D6-①）
	a.invalidateRoleAccountsSnapshot(ctx, request.Id)
	return &emptypb.Empty{}, nil
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
