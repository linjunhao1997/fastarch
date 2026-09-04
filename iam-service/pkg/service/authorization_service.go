package service

import (
	"context"
	"encoding/json"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

type AuthorizationService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func (a AuthorizationService) CheckPermission(ctx context.Context, request *iam_v1.CheckPermissionRequest) (*iam_v1.CheckPermissionResponse, error) {
	str, err := a.rdb.Get(ctx, constant.GetAccountPermissionKey(auth.GetAccountId(ctx))).Result()
	if err != nil {
		return nil, err
	}

	var permCodes []string
	if err = json.Unmarshal([]byte(str), &permCodes); err != nil {
		return nil, err
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
	permission := model.Permission{
		Id:            pkId,
		Name:          request.Name,
		Code:          request.Code,
		Value:         request.Value,
		Description:   request.Description,
		ApplicationId: request.ApplicationId,
		Disabled:      false,
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
	updates[model.PermissionColumnVersion] = gorm.Expr(model.PermissionColumnVersion + " + 1")
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
	}
	result := a.db.WithContext(ctx).Model(model.Permission{}).
		Where(model.PermissionColumnId, request.Id).
		Where(model.PermissionColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := a.db.WithContext(ctx).Model(model.Permission{}).
			Where(model.PermissionColumnId, request.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, ErrUpdatePermissionBusy
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) CreateRole(ctx context.Context, request *iam_v1.CreateRoleRequest) (*iam_v1.Role, error) {
	pkId := krand.NewID()
	role := model.Role{
		Id:       pkId,
		Name:     request.Name,
		Code:     request.RoleCode,
		Disabled: false,
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
	updates[model.RoleColumnVersion] = gorm.Expr(model.RoleColumnVersion + " + 1")
	return a.updateRoleInTransaction(ctx, request, updates)
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
		return nil, err
	}
	if count > 0 {
		return nil, ErrPermissionReferenced
	}

	result := a.db.WithContext(ctx).
		Where(model.PermissionColumnId, request.Id).
		Delete(model.Permission{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
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
		return nil, err
	}
	if count > 0 {
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
		return nil, err
	}
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
