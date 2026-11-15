package service

import (
	"context"

	"iam-service/pkg/auth"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// 权限订阅（申请记录）域在 011 模型中暂无对应表结构（DDL 仅含
// t_user/t_role/t_permission/t_menu/t_dict/...），接口保留但返回 Unimplemented，
// 待订阅表落地后再实现。
func (a *AuthorizationService) CreateSubscription(ctx context.Context, request *iam_v1.CreateSubscriptionRequest) (*iam_v1.Subscription, error) {
	return nil, status.Error(codes.Unimplemented, "权限订阅暂未开放")
}

func (a *AuthorizationService) GetSubscription(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Subscription, error) {
	return nil, status.Error(codes.Unimplemented, "权限订阅暂未开放")
}

func (a *AuthorizationService) UpdateSubscription(ctx context.Context, request *iam_v1.UpdateSubscriptionRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "权限订阅暂未开放")
}

func (a *AuthorizationService) ListSubscriptions(ctx context.Context, request *iam_v1.ListSubscriptionsRequest) (*iam_v1.ListSubscriptionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "权限订阅暂未开放")
}

// GetMyPermissions 返回当前认证账号持有的全部权限码（011 模型：纯权限码快照）。
func (a *AuthorizationService) GetMyPermissions(ctx context.Context, request *emptypb.Empty) (*iam_v1.GetMyPermissionsResponse, error) {
	permKeys, err := loadAccountPermissionSnapshot(ctx, a.rdb, a.db, auth.GetAccountId(ctx))
	if err != nil {
		return nil, err
	}
	return &iam_v1.GetMyPermissionsResponse{
		PermissionCodeList: permKeys,
	}, nil
}
