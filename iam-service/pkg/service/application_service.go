package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	contribmodel "github.com/linjunhao1997/contrib/model"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func NewApplicationService(db *gorm.DB) *ApplicationService {
	return &ApplicationService{db: db}
}

// applicationToPb 模型转 pb：id/app_id/name/status/audit（不回传 secret 摘要）。
func applicationToPb(a model.Application) *iam_v1.Application {
	return &iam_v1.Application{
		Id:         cast.ToString(a.Id),
		AppId:      a.AppId,
		Code:       a.Code,
		Name:       a.Name,
		Status:     a.Status,
		Version:    a.Version,
		CreatedBy:  a.CreatedBy,
		UpdatedBy:  a.UpdatedBy,
		CreateTime: timestamppb.New(a.CreateTime),
		UpdateTime: timestamppb.New(a.UpdateTime),
	}
}

// generateApplicationSecret 生成 32 字节随机 secret（64 位 hex 明文），
// 服务端仅存 sha256 hex 摘要；明文只在创建响应中一次性返回。
func generateApplicationSecret() (plaintext string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plaintext = hex.EncodeToString(buf)
	digest := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(digest[:])
	return plaintext, hash, nil
}

// CreateApplication 创建应用：服务端生成 app_id 与随机 secret，响应一次性返回
// secret 明文（服务端只存 sha256 hex 摘要，之后任何接口不再返回）。
func (s *ApplicationService) CreateApplication(ctx context.Context, request *iam_v1.CreateApplicationRequest) (*iam_v1.CreateApplicationResponse, error) {
	// 应用域写操作硬门槛（ADR 0006 补充）：仅 admin/系统调用，码门槛不足以约束租户
	if err := requireAdminOperator(ctx); err != nil {
		return nil, err
	}
	// 服务段编码唯一性预检（多应用裁决 #8：code 是权限码命名空间，全局唯一）
	var codeCount int64
	err := s.db.WithContext(ctx).Model(model.Application{}).
		Where(model.ApplicationColumnCode, request.Code).
		Count(&codeCount).Error
	if err != nil {
		slog.Warn("check application code failed", "code", request.Code, "error", err)
		return nil, err
	}
	if codeCount > 0 {
		return nil, ErrApplicationCodeDuplicated
	}
	secret, secretHash, err := generateApplicationSecret()
	if err != nil {
		slog.Warn("generate application secret failed", "error", err)
		return nil, err
	}
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入（宪法 XII）
	operator := cast.ToString(auth.GetAccountId(ctx))
	status := request.Status
	if status == "" {
		status = constant.ApplicationStatusEnabled
	}
	app := model.Application{
		Id:         pkId,
		AppId:      cast.ToString(pkId),
		Code:       request.Code,
		Name:       request.Name,
		SecretHash: secretHash,
		Status:     status,
		Version:    1,
		CreatedBy:  operator,
		UpdatedBy:  operator,
		CreateTime: now,
		UpdateTime: now,
	}
	err = s.db.WithContext(ctx).Select(model.ApplicationFields).Create(&app).Error
	if err != nil {
		slog.Warn("create application failed", "app_name", request.Name, "error", err)
		return nil, err
	}
	return &iam_v1.CreateApplicationResponse{
		Application: applicationToPb(app),
		Secret:      secret,
	}, nil
}

// GetApplication 获取应用信息（不回传 secret）。
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

// ListApplications 应用分页列表：keyword 模糊匹配 name/app_id（不回传 secret）。
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

// UpdateApplication 修改应用：name/status 可选更新（空串 = 不修改）。
func (s *ApplicationService) UpdateApplication(ctx context.Context, request *iam_v1.UpdateApplicationRequest) (*emptypb.Empty, error) {
	// 应用域写操作硬门槛（ADR 0006 补充）：仅 admin/系统调用
	if err := requireAdminOperator(ctx); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.ApplicationColumnName] = request.Name
	}
	if len(request.Status) > 0 {
		updates[model.ApplicationColumnStatus] = request.Status
	}
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
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

// countApplicationReferences 统计引用该应用的数据行数：角色/权限/菜单/字典归属、
// 用户接入绑定。固定五项 Count（非数据驱动 N+1），任一引用即禁删。
func (s *ApplicationService) countApplicationReferences(ctx context.Context, appId int64) (int64, error) {
	refs := []struct {
		model  interface{}
		column string
	}{
		{model.Role{}, model.RoleColumnApplicationId},
		{model.Permission{}, model.PermissionColumnApplicationId},
		{contribmodel.Menu{}, contribmodel.MenuColumnApplicationId},
		{contribmodel.DictType{}, contribmodel.DictTypeColumnApplicationId},
		{model.RUserApplication{}, model.RUserApplicationColumnApplicationId},
	}
	var total int64
	for _, ref := range refs {
		var count int64
		if err := s.db.WithContext(ctx).Model(ref.model).Where(ref.column, appId).Count(&count).Error; err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// DeleteApplication 删除应用（删除策略: 基础架构型，物理删除 + 审计日志）：
// admin-only 硬门槛 + 引用校验（ADR 0006 补充）。000013 起角色/权限重新归属应用，
// 叠加菜单/字典归属与用户 home/准入绑定，存在引用时拒绝删除，避免孤儿数据。
func (s *ApplicationService) DeleteApplication(ctx context.Context, request *iam_v1.DeleteApplicationRequest) (*emptypb.Empty, error) {
	if err := requireAdminOperator(ctx); err != nil {
		return nil, err
	}
	refs, err := s.countApplicationReferences(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	if refs > 0 {
		slog.Warn("delete application rejected: referenced by data", "app_id", request.Id, "reference_count", refs)
		return nil, ErrApplicationReferenced
	}
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
