package service

import (
	"context"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/protobuf/types/known/emptypb"
)

// 组件资源模型已废弃（011 裁决）：资源 CRUD 与资源树接口一律返回
// ErrResourceModelDeprecated（Unimplemented），前端改用菜单树接口。
func (a *AuthorizationService) CreateResource(ctx context.Context, request *iam_v1.CreateResourceRequest) (*iam_v1.Resource, error) {
	return nil, ErrResourceModelDeprecated
}

func (a *AuthorizationService) GetResource(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Resource, error) {
	return nil, ErrResourceModelDeprecated
}

func (a *AuthorizationService) UpdateResource(ctx context.Context, request *iam_v1.UpdateResourceRequest) (*iam_v1.Resource, error) {
	return nil, ErrResourceModelDeprecated
}

func (a *AuthorizationService) DeleteResource(ctx context.Context, request *iam_v1.DeleteResourceRequest) (*emptypb.Empty, error) {
	return nil, ErrResourceModelDeprecated
}

func (a *AuthorizationService) ListResources(ctx context.Context, request *iam_v1.ListResourcesRequest) (*iam_v1.ListResourcesResponse, error) {
	return nil, ErrResourceModelDeprecated
}

func (a *AuthorizationService) ListResourceTree(ctx context.Context, request *iam_v1.ListResourceTreeRequest) (*iam_v1.ListResourceTreeResponse, error) {
	return nil, ErrResourceModelDeprecated
}
