package service

import (
	"context"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
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

func (s *ApplicationService) CreateApplication(ctx context.Context, request *iam_v1.CreateApplicationRequest) (*iam_v1.StatusResponse, error) {
	pkId := krand.NewID()
	now := time.Now()
	app := model.Application{
		Id:         pkId,
		Code:       cast.ToString(pkId),
		Name:       request.Name,
		Status:     constant.ApplicationStatusEnabled,
		CreateTime: now,
		UpdateTime: now,
	}
	err := s.db.WithContext(ctx).Model(model.Application{}).Create(&app).Error
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

func (s *ApplicationService) GetApplication(ctx context.Context, request *iam_v1.GetRequest) (*iam_v1.GetApplicationResponse, error) {
	var app model.Application
	err := s.db.WithContext(ctx).Where(model.ApplicationColumnId, request.Id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &iam_v1.GetApplicationResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Application: &iam_v1.Application{
			Id:            cast.ToString(app.Id),
			Name:          app.Name,
			AppCode:       app.Code,
			EnabledStatus: boolToStatus(app.Status == constant.ApplicationStatusEnabled),
		},
	}, nil
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
		appList = append(appList, &iam_v1.Application{
			Id:            cast.ToString(app.Id),
			Name:          app.Name,
			AppCode:       app.Code,
			EnabledStatus: boolToStatus(app.Status == constant.ApplicationStatusEnabled),
		})
	}
	return &iam_v1.ListApplicationsResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
		Applications: appList,
		Page:         helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (s *ApplicationService) UpdateApplication(ctx context.Context, request *iam_v1.UpdateApplicationRequest) (*iam_v1.StatusResponse, error) {
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.ApplicationColumnName] = request.Name
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
	return &iam_v1.StatusResponse{
		Status: &iam_v1.Status{
			Code: constant.ResponseCodeSuccess,
		},
	}, nil
}