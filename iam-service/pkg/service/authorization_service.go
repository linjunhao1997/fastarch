package service

import (
	"context"
	"encoding/json"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"

	"github.com/go-dev-frame/sponge/pkg/krand"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
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

func (a AuthorizationService) CreatePermission(ctx context.Context, request *iam_v1.CreatePermissionRequest) (*iam_v1.StatusResponse, error) {
	pkId := krand.NewID()
	permission := model.Permission{
		Id:       pkId,
		Name:     request.Name,
		Code:     "",
		Value:    request.Email,
		Disabled: false,
	}
	err := a.db.WithContext(ctx).Model(model.Permission{}).Create(&permission).Error
	if err != nil {
		return nil, err
	}
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code:   constant.ResponseCodeSuccess,
			DataId: cast.ToString(pkId),
		},
	}, nil
}

func (a AuthorizationService) GetPermission(ctx context.Context, request *iam_v1.GetRequest) (*iam_v1.GetPermissionResponse, error) {
	var permission model.Permission
	err := a.db.WithContext(ctx).Where(model.PermissionColumnId, request.Id).First(&permission).Error
	if err != nil {
		return nil, err
	}
	return &iam_v1.GetPermissionResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Permission: &iam_v1.Permission{
			Id:             cast.ToString(permission.Id),
			Name:           permission.Name,
			ApplicationId:  permission.ApplicationId,
			PermissionCode: permission.Code,
			EnabledStatus:  boolToStatus(permission.Disabled),
		},
	}, nil
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
		permList = append(permList, &iam_v1.Permission{
			Id:             cast.ToString(p.Id),
			Name:           p.Name,
			ApplicationId:  p.ApplicationId,
			PermissionCode: p.Code,
			EnabledStatus:  boolToStatus(p.Disabled),
		})
	}
	return &iam_v1.ListPermissionsResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Permissions: permList,
		Page:        helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a AuthorizationService) UpdatePermission(ctx context.Context, request *iam_v1.UpdatePermissionRequest) (*iam_v1.StatusResponse, error) {
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
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
	}, nil
}

func (a AuthorizationService) CreateRole(ctx context.Context, request *iam_v1.CreateRoleRequest) (*iam_v1.StatusResponse, error) {
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
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code:   constant.ResponseCodeSuccess,
			DataId: cast.ToString(pkId),
		},
	}, nil
}

func (a AuthorizationService) GetRole(ctx context.Context, request *iam_v1.GetRequest) (*iam_v1.GetRoleResponse, error) {
	var role model.Role
	err := a.db.WithContext(ctx).Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &iam_v1.GetRoleResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Role: &iam_v1.Role{
			Id:            cast.ToString(role.Id),
			Name:          role.Name,
			ApplicationId: role.ApplicationId,
			RoleCode:      role.Code,
			EnabledStatus: boolToStatus(role.Disabled),
		},
	}, nil
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
		roleList = append(roleList, &iam_v1.Role{
			Id:            cast.ToString(r.Id),
			Name:          r.Name,
			ApplicationId: r.ApplicationId,
			RoleCode:      r.Code,
			EnabledStatus: boolToStatus(r.Disabled),
		})
	}
	return &iam_v1.ListRolesResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Roles: roleList,
		Page:  helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a AuthorizationService) UpdateRole(ctx context.Context, request *iam_v1.UpdateRoleRequest) (*iam_v1.StatusResponse, error) {
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
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
	}, nil
}

func (a AuthorizationService) CreateSubscription(ctx context.Context, request *iam_v1.CreateSubscriptionRequest) (*iam_v1.StatusResponse, error) {
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
	}, nil
}

func (a AuthorizationService) GetSubscription(ctx context.Context, request *iam_v1.GetRequest) (*iam_v1.GetSubscriptionResponse, error) {
	return &iam_v1.GetSubscriptionResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Subscription: &iam_v1.Subscription{},
	}, nil
}

func (a AuthorizationService) UpdateSubscription(ctx context.Context, request *iam_v1.UpdateSubscriptionRequest) (*iam_v1.StatusResponse, error) {
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
	}, nil
}

func (a AuthorizationService) ListSubscription(ctx context.Context, request *iam_v1.ListSubscriptionRequest) (*iam_v1.ListSubscriptionResponse, error) {
	return &iam_v1.ListSubscriptionResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Subscriptions: []*iam_v1.Subscription{},
		Page:          &iam_v1.Page{},
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
