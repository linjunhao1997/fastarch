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
	}
}

func roleToPb(r model.Role) *iam_v1.Role {
	return &iam_v1.Role{
		Id:            cast.ToString(r.Id),
		Name:          r.Name,
		ApplicationId: r.ApplicationId,
		RoleCode:      r.Code,
		EnabledStatus: boolToStatus(r.Disabled),
	}
}

func (a AuthorizationService) CreatePermission(ctx context.Context, request *iam_v1.CreatePermissionRequest) (*iam_v1.Permission, error) {
	pkId := krand.NewID()
	permission := model.Permission{
		Id:            pkId,
		Name:          request.Name,
		Code:          request.Code,
		Value:         request.Value,
		ApplicationId: request.ApplicationId,
		Disabled:      false,
	}
	err := a.db.WithContext(ctx).Model(model.Permission{}).Create(&permission).Error
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
	err := a.db.WithContext(ctx).Model(model.Permission{}).Count(&count).Limit(limit).Offset(offset).Find(&permissions).Error
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
	result := a.db.WithContext(ctx).Model(model.Permission{}).
		Where(model.PermissionColumnId, request.Id).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
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
	err := a.db.WithContext(ctx).Model(model.Role{}).Create(&role).Error
	if err != nil {
		return nil, err
	}
	return roleToPb(role), nil
}

func (a AuthorizationService) GetRole(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Role, error) {
	var role model.Role
	err := a.db.WithContext(ctx).Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	return roleToPb(role), nil
}

func (a AuthorizationService) ListRoles(ctx context.Context, request *iam_v1.ListRolesRequest) (*iam_v1.ListRolesResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	var count int64
	var roles []model.Role
	err := a.db.WithContext(ctx).Model(model.Role{}).Count(&count).Limit(limit).Offset(offset).Find(&roles).Error
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
	result := a.db.WithContext(ctx).Model(model.Role{}).
		Where(model.RoleColumnId, request.Id).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
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
