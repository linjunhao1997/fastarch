package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"sender-service/pkg/model"
)

var _ sender_v1.TemplateServiceServer = (*TemplateService)(nil)

// TemplateService 消息模板 CRUD（GLOSSARY 消息模板），按应用隔离。
type TemplateService struct {
	db            *gorm.DB
	applicationId int64
}

func NewTemplateService(db *gorm.DB, applicationId int64) *TemplateService {
	return &TemplateService{db: db, applicationId: applicationId}
}

func (s *TemplateService) CreateTemplate(ctx context.Context, req *sender_v1.CreateTemplateRequest) (*sender_v1.MessageTemplate, error) {
	now := time.Now()
	operator := operatorFromContext(ctx)
	tpl := model.MessageTemplate{
		Id:            krand.NewID(),
		ApplicationId: s.applicationId,
		Code:          req.Code,
		Name:          req.Name,
		Content:       req.Content,
		Remark:        req.Remark,
		Version:       1,
		CreatedBy:     operator,
		UpdatedBy:     operator,
		CreateTime:    now,
		UpdateTime:    now,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.MessageTemplate{}).
			Where(model.MessageTemplateColumnApplicationId, s.applicationId).
			Where(model.MessageTemplateColumnCode, req.Code).
			Where(model.MessageTemplateColumnIsDeleted, false).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrTemplateCodeDuplicated
		}
		return tx.Select(model.MessageTemplateFields).Create(&tpl).Error
	})
	if err != nil {
		slog.Warn("create template failed", "code", req.Code, "error", err)
		return nil, err
	}
	return templateToPb(tpl), nil
}

func (s *TemplateService) GetTemplate(ctx context.Context, req *common_v1.GetRequest) (*sender_v1.MessageTemplate, error) {
	var tpl model.MessageTemplate
	err := s.db.WithContext(ctx).
		Select(model.MessageTemplateFields).
		Where(model.MessageTemplateColumnId, req.Id).
		Where(model.MessageTemplateColumnApplicationId, s.applicationId).
		Where(model.MessageTemplateColumnIsDeleted, false).
		First(&tpl).Error
	if err != nil {
		return nil, translateNotFound(err, ErrTemplateNotFound)
	}
	return templateToPb(tpl), nil
}

func (s *TemplateService) ListTemplates(ctx context.Context, req *sender_v1.ListTemplatesRequest) (*sender_v1.ListTemplatesResponse, error) {
	limit, offset := limitAndOffset(req.CurrentPage, req.PageSize)
	build := func() *gorm.DB {
		q := s.db.WithContext(ctx).Model(&model.MessageTemplate{}).
			Where(model.MessageTemplateColumnApplicationId, s.applicationId).
			Where(model.MessageTemplateColumnIsDeleted, false)
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("("+model.MessageTemplateColumnCode+" LIKE ? OR "+model.MessageTemplateColumnName+" LIKE ?)", like, like)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MessageTemplate
	if err := build().
		Select(model.MessageTemplateFields).
		Order(model.MessageTemplateColumnId + " ASC").
		Limit(limit).Offset(offset).
		Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]*sender_v1.MessageTemplate, 0, len(list))
	for _, m := range list {
		out = append(out, templateToPb(m))
	}
	return &sender_v1.ListTemplatesResponse{
		Page:      pageOf(req.CurrentPage, req.PageSize, total),
		Templates: out,
	}, nil
}

func (s *TemplateService) UpdateTemplate(ctx context.Context, req *sender_v1.UpdateTemplateRequest) (*emptypb.Empty, error) {
	var tpl model.MessageTemplate
	if err := s.db.WithContext(ctx).
		Select(model.MessageTemplateFields).
		Where(model.MessageTemplateColumnId, req.Id).
		Where(model.MessageTemplateColumnApplicationId, s.applicationId).
		Where(model.MessageTemplateColumnIsDeleted, false).
		First(&tpl).Error; err != nil {
		return nil, translateNotFound(err, ErrTemplateNotFound)
	}

	updates := map[string]any{}
	if req.Code != "" && req.Code != tpl.Code {
		var count int64
		if err := s.db.WithContext(ctx).Model(&model.MessageTemplate{}).
			Where(model.MessageTemplateColumnApplicationId, s.applicationId).
			Where(model.MessageTemplateColumnCode, req.Code).
			Where(model.MessageTemplateColumnIsDeleted, false).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, ErrTemplateCodeDuplicated
		}
		updates[model.MessageTemplateColumnCode] = req.Code
	}
	if req.Name != "" {
		updates[model.MessageTemplateColumnName] = req.Name
	}
	if req.Content != "" {
		updates[model.MessageTemplateColumnContent] = req.Content
	}
	if req.Remark != "" {
		updates[model.MessageTemplateColumnRemark] = req.Remark
	}
	updates[model.MessageTemplateColumnUpdateTime] = time.Now()
	updates[model.MessageTemplateColumnUpdatedBy] = operatorFromContext(ctx)
	updates[model.MessageTemplateColumnVersion] = gorm.Expr(model.MessageTemplateColumnVersion + " + 1")

	result := s.db.WithContext(ctx).Model(&model.MessageTemplate{}).
		Where(model.MessageTemplateColumnId, req.Id).
		Where(model.MessageTemplateColumnApplicationId, s.applicationId).
		Where(model.MessageTemplateColumnVersion, req.Version).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrUpdateTemplateBusy
	}
	return &emptypb.Empty{}, nil
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, req *common_v1.GetRequest) (*emptypb.Empty, error) {
	var tpl model.MessageTemplate
	if err := s.db.WithContext(ctx).
		Select(model.MessageTemplateFields).
		Where(model.MessageTemplateColumnId, req.Id).
		Where(model.MessageTemplateColumnApplicationId, s.applicationId).
		Where(model.MessageTemplateColumnIsDeleted, false).
		First(&tpl).Error; err != nil {
		return nil, translateNotFound(err, ErrTemplateNotFound)
	}
	now := time.Now()
	suffix := "(已删除_" + now.Format("20060102150405.000") + ")"
	updates := map[string]any{
		model.MessageTemplateColumnIsDeleted:    true,
		model.MessageTemplateColumnCode:         tpl.Code + suffix,
		model.MessageTemplateColumnUpdateTime:   now,
		model.MessageTemplateColumnUpdatedBy:    operatorFromContext(ctx),
		model.MessageTemplateColumnVersion:      tpl.Version + 1,
	}
	result := s.db.WithContext(ctx).Model(&model.MessageTemplate{}).
		Where(model.MessageTemplateColumnId, req.Id).
		Where(model.MessageTemplateColumnApplicationId, s.applicationId).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrTemplateNotFound
	}
	return &emptypb.Empty{}, nil
}

func templateToPb(m model.MessageTemplate) *sender_v1.MessageTemplate {
	return &sender_v1.MessageTemplate{
		Id:         toString(m.Id),
		Code:       m.Code,
		Name:       m.Name,
		Content:    m.Content,
		Remark:     m.Remark,
		Version:    m.Version,
		CreatedBy:  m.CreatedBy,
		UpdatedBy:  m.UpdatedBy,
		CreateTime: timestamppb.New(m.CreateTime),
		UpdateTime: timestamppb.New(m.UpdateTime),
	}
}
