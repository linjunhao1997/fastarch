package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"sender-service/pkg/model"
)

var _ sender_v1.MessageServiceServer = (*MessageService)(nil)

// MessageService 发送消息并维护发送记录（消息中心，GLOSSARY）。
// 投递本期为模拟：创建记录 pending→sent 并记日志（真实渠道留给 #15）。
type MessageService struct {
	db            *gorm.DB
	applicationId int64
	// application 归属应用展示摘要（宿主固定应用的配置值，只读回填）。
	application *common_v1.ApplicationSummary
}

// NewMessageService 构建消息服务。application 为可选展示摘要：生产由 server 注入配置，
// 省略时响应不回填摘要（测试等场景保持向后兼容）。
func NewMessageService(db *gorm.DB, applicationId int64, application ...*common_v1.ApplicationSummary) *MessageService {
	s := &MessageService{db: db, applicationId: applicationId}
	if len(application) > 0 {
		s.application = application[0]
	}
	return s
}

// SendMessage 发送消息：在单事务内创建发送记录（pending），模拟投递后流转到 sent。
// 审计字段由服务端注入；归属应用固定为宿主 application_id。
func (s *MessageService) SendMessage(ctx context.Context, req *sender_v1.SendMessageRequest) (*sender_v1.MessageRecord, error) {
	now := time.Now()
	operator := operatorFromContext(ctx)
	appId := dataApplicationId(ctx, s.applicationId)
	record := model.MessageRecord{
		Id:             krand.NewID(),
		ApplicationId:  appId,
		Recipient:      req.Recipient,
		TemplateCode:   req.TemplateCode,
		Content:        req.Content,
		Channel:        req.Channel,
		SendStatus:     model.SendStatusPending,
		Version:        1,
		CreatedBy:      operator,
		UpdatedBy:      operator,
		CreateTime:     now,
		UpdateTime:     now,
	}
	// 事务内仅做落库：创建 pending 记录 + 模拟投递流转到 sent（#12 MVP）。
	// 日志属外部 IO，移到事务外（宪法 XIV 4.3 事务禁外部 IO）。
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select(model.MessageRecordFields).Create(&record).Error; err != nil {
			return err
		}
		record.SendStatus = model.SendStatusSent
		record.Version = 2
		record.UpdatedBy = operator
		record.UpdateTime = time.Now()
		return tx.Model(&model.MessageRecord{}).
			Where(model.MessageRecordColumnId, record.Id).
			Updates(map[string]any{
				model.MessageRecordColumnSendStatus: record.SendStatus,
				model.MessageRecordColumnVersion:    record.Version,
				model.MessageRecordColumnUpdatedBy:  record.UpdatedBy,
				model.MessageRecordColumnUpdateTime: record.UpdateTime,
			}).Error
	}); err != nil {
		slog.Warn("send message failed", "record_id", record.Id, "error", err)
		return nil, err
	}
	// 模拟投递日志（真实渠道留给 #15）：接收对象脱敏，仅保留可诊断的 record_id/channel。
	slog.Info("deliver message (mock)",
		"domain", "sender-service", "record_id", record.Id,
		"channel", record.Channel, "status", record.SendStatus)
	return s.messageRecordToPb(ctx, record), nil
}

// GetMessage 获取发送记录（限当前应用，跨应用按不存在处理；过滤已删除）。
func (s *MessageService) GetMessage(ctx context.Context, req *common_v1.GetRequest) (*sender_v1.MessageRecord, error) {
	appId := dataApplicationId(ctx, s.applicationId)
	var record model.MessageRecord
	err := s.db.WithContext(ctx).
		Select(model.MessageRecordFields).
		Where(model.MessageRecordColumnId, req.Id).
		Where(model.MessageRecordColumnApplicationId, appId).
		Where(model.MessageRecordColumnIsDeleted, false).
		First(&record).Error
	if err != nil {
		return nil, translateNotFound(err, ErrMessageNotFound)
	}
	return s.messageRecordToPb(ctx, record), nil
}

// ListMessages 发送记录分页列表（限当前应用，过滤已删除）。
func (s *MessageService) ListMessages(ctx context.Context, req *sender_v1.ListMessagesRequest) (*sender_v1.ListMessagesResponse, error) {
	limit, offset := limitAndOffset(req.CurrentPage, req.PageSize)
	appId := dataApplicationId(ctx, s.applicationId)
	build := func() *gorm.DB {
		q := s.db.WithContext(ctx).Model(&model.MessageRecord{}).
			Where(model.MessageRecordColumnApplicationId, appId).
			Where(model.MessageRecordColumnIsDeleted, false)
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("("+model.MessageRecordColumnRecipient+" LIKE ? OR "+model.MessageRecordColumnTemplateCode+" LIKE ?)", like, like)
		}
		if req.SendStatus != "" {
			q = q.Where(model.MessageRecordColumnSendStatus, req.SendStatus)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, err
	}
	var records []model.MessageRecord
	if err := build().
		Select(model.MessageRecordFields).
		Order(model.MessageRecordColumnCreateTime + " DESC").
		Limit(limit).Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	list := make([]*sender_v1.MessageRecord, 0, len(records))
	for _, r := range records {
		list = append(list, s.messageRecordToPb(ctx, r))
	}
	return &sender_v1.ListMessagesResponse{
		Page:    pageOf(req.CurrentPage, req.PageSize, total),
		Records: list,
	}, nil
}

// DeleteMessage 删除发送记录（业务型模型：软删除 + 审计字段 + 名称字段加后缀）。
func (s *MessageService) DeleteMessage(ctx context.Context, req *common_v1.GetRequest) (*emptypb.Empty, error) {
	appId := dataApplicationId(ctx, s.applicationId)
	var record model.MessageRecord
	if err := s.db.WithContext(ctx).
		Select(model.MessageRecordFields).
		Where(model.MessageRecordColumnId, req.Id).
		Where(model.MessageRecordColumnApplicationId, appId).
		Where(model.MessageRecordColumnIsDeleted, false).
		First(&record).Error; err != nil {
		return nil, translateNotFound(err, ErrMessageNotFound)
	}
	now := time.Now()
	// 发送记录无唯一键业务名称字段，软删除只需 is_deleted + 审计字段 + 版本递增；
	// 不改写 recipient（它是业务数据，加后缀会污染留痕）。
	updates := map[string]any{
		model.MessageRecordColumnIsDeleted:  true,
		model.MessageRecordColumnUpdateTime: now,
		model.MessageRecordColumnUpdatedBy:  operatorFromContext(ctx),
		model.MessageRecordColumnVersion:    record.Version + 1,
	}
	result := s.db.WithContext(ctx).Model(&model.MessageRecord{}).
		Where(model.MessageRecordColumnId, record.Id).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrMessageNotFound
	}
	return &emptypb.Empty{}, nil
}

func (s *MessageService) messageRecordToPb(ctx context.Context, m model.MessageRecord) *sender_v1.MessageRecord {
	return &sender_v1.MessageRecord{
		Id:          toString(m.Id),
		Recipient:   m.Recipient,
		TemplateCode: m.TemplateCode,
		Content:     m.Content,
		Channel:     m.Channel,
		SendStatus:  m.SendStatus,
		Version:     m.Version,
		CreatedBy:   m.CreatedBy,
		UpdatedBy:   m.UpdatedBy,
		CreateTime:  timestamppb.New(m.CreateTime),
		UpdateTime:  timestamppb.New(m.UpdateTime),
		Application: dataApplicationSummary(ctx, s.application),
	}
}
