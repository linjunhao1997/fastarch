package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
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

// applicationToPb 模型转 pb：proto 未随 011 模型更新，AppCode 映射 app_id；
// description 随列删除留空；EnabledStatus 映射 status。
func applicationToPb(a model.Application) *iam_v1.Application {
	return &iam_v1.Application{
		Id:            cast.ToString(a.Id),
		Name:          a.Name,
		AppCode:       a.AppId,
		EnabledStatus: a.Status,
	}
}

func (s *ApplicationService) CreateApplication(ctx context.Context, request *iam_v1.CreateApplicationRequest) (*iam_v1.Application, error) {
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入（宪法 XII）
	operator := cast.ToString(auth.GetAccountId(ctx))
	status := request.EnabledStatus
	if status == "" {
		status = constant.ApplicationStatusEnabled
	}
	app := model.Application{
		Id:         pkId,
		AppId:      cast.ToString(pkId),
		Name:       request.Name,
		Status:     status,
		Version:    1,
		CreatedBy:  operator,
		UpdatedBy:  operator,
		CreateTime: now,
		UpdateTime: now,
	}
	err := s.db.WithContext(ctx).Select(model.ApplicationFields).Create(&app).Error
	if err != nil {
		slog.Warn("create application failed", "app_name", request.Name, "error", err)
		return nil, err
	}
	return applicationToPb(app), nil
}

func (s *ApplicationService) GetApplication(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Application, error) {
	var app model.Application
	err := s.db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnId, request.Id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return applicationToPb(app), nil
}

func (s *ApplicationService) ListApplications(ctx context.Context, request *iam_v1.ListApplicationsRequest) (*iam_v1.ListApplicationsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.ApplicationColumnName + " LIKE ?",
		model.ApplicationColumnAppId + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		return applyKeywordFilter(s.db.WithContext(ctx).Model(model.Application{}), keywordConditions, request.Keyword)
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var apps []model.Application
	err = buildQuery().
		Order(model.ApplicationColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&apps).Error
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
	if len(request.EnabledStatus) > 0 {
		updates[model.ApplicationColumnStatus] = request.EnabledStatus
	}
	updates[model.ApplicationColumnUpdateTime] = time.Now()
	// 审计字段服务端注入：仅更新修改人与修改时间
	updates[model.ApplicationColumnUpdatedBy] = cast.ToString(auth.GetAccountId(ctx))
	updates[model.ApplicationColumnVersion] = gorm.Expr(model.ApplicationColumnVersion + " + 1")
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
	// 011 模型：角色/权限不再归属应用，应用为叶子资源直接物理删除
	// （删除策略: 基础架构型，物理删除 + 审计日志）
	result := s.db.WithContext(ctx).
		Where(model.ApplicationColumnId, request.Id).
		Delete(model.Application{})
	if result.Error != nil {
		slog.Warn("delete application failed", "app_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}
