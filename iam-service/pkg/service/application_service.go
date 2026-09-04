package service

import (
	"context"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"
)

type ApplicationService struct {
	db *gorm.DB
}

var _ iam_v1.ApplicationServiceServer = (*ApplicationService)(nil)

func NewApplicationService(db *gorm.DB) iam_v1.ApplicationServiceServer {
	return &ApplicationService{db: db}
}

func applicationToPb(a model.Application) *iam_v1.Application {
	return &iam_v1.Application{
		Id:            cast.ToString(a.Id),
		Name:          a.Name,
		AppCode:       a.Code,
		Description:   a.Description,
		EnabledStatus: boolToStatus(a.Status == constant.ApplicationStatusEnabled),
	}
}

func (s *ApplicationService) CreateApplication(ctx context.Context, request *iam_v1.CreateApplicationRequest) (*iam_v1.Application, error) {
	pkId := krand.NewID()
	now := time.Now()
	app := model.Application{
		Id:          pkId,
		Code:        cast.ToString(pkId),
		Name:        request.Name,
		Description: request.Description,
		Status:      constant.ApplicationStatusEnabled,
		CreateTime:  now,
		UpdateTime:  now,
	}
	err := s.db.WithContext(ctx).Model(model.Application{}).Create(&app).Error
	if err != nil {
		return nil, err
	}
	return applicationToPb(app), nil
}

func (s *ApplicationService) GetApplication(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Application, error) {
	var app model.Application
	err := s.db.WithContext(ctx).Where(model.ApplicationColumnId, request.Id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return applicationToPb(app), nil
}

func (s *ApplicationService) ListApplications(ctx context.Context, request *iam_v1.ListApplicationsRequest) (*iam_v1.ListApplicationsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	var count int64
	var apps []model.Application
	err := s.db.WithContext(ctx).Model(model.Application{}).Count(&count).Limit(limit).Offset(offset).Find(&apps).Error
	if err != nil {
		return nil, err
	}
	appList := make([]*iam_v1.Application, 0, len(apps))
	for _, app := range apps {
		appList = append(appList, applicationToPb(app))
	}
	return &iam_v1.ListApplicationsResponse{
		Applications: appList,
		Page:         helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (s *ApplicationService) UpdateApplication(ctx context.Context, request *iam_v1.UpdateApplicationRequest) (*emptypb.Empty, error) {
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.ApplicationColumnName] = request.Name
	}
	if len(request.Description) > 0 {
		updates[model.ApplicationColumnDescription] = request.Description
	}
	if len(request.EnabledStatus) > 0 {
		if request.EnabledStatus == "enabled" {
			updates[model.ApplicationColumnStatus] = constant.ApplicationStatusEnabled
		} else {
			updates[model.ApplicationColumnStatus] = constant.ApplicationStatusDisabled
		}
	}
	updates[model.ApplicationColumnUpdateTime] = time.Now()
	result := s.db.WithContext(ctx).Model(model.Application{}).
		Where(model.ApplicationColumnId, request.Id).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

func (s *ApplicationService) DeleteApplication(ctx context.Context, request *iam_v1.DeleteApplicationRequest) (*emptypb.Empty, error) {
	var roleCount int64
	err := s.db.WithContext(ctx).Model(model.Role{}).
		Where(model.RoleColumnApplicationId, request.Id).
		Count(&roleCount).Error
	if err != nil {
		return nil, err
	}
	var permissionCount int64
	err = s.db.WithContext(ctx).Model(model.Permission{}).
		Where(model.PermissionColumnApplicationId, request.Id).
		Count(&permissionCount).Error
	if err != nil {
		return nil, err
	}
	if roleCount > 0 || permissionCount > 0 {
		return nil, ErrApplicationReferenced
	}

	result := s.db.WithContext(ctx).Where(model.ApplicationColumnId, request.Id).Delete(model.Application{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}
