package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	home_v1 "github.com/linjunhao1997/idl/pb/home"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"home-service/pkg/model"
)

var _ home_v1.HomepageServiceServer = (*HomepageService)(nil)

const maxHomepageContentBytes = 200 * 1024

// HomepageService 企业官网首页配置服务（单应用单行，草稿/发布分离）。
type HomepageService struct {
	db            *gorm.DB
	applicationId int64
	// application 归属应用展示摘要（宿主固定应用的配置值，只读回填）。
	application *common_v1.ApplicationSummary
}

// NewHomepageService 构建首页配置服务。
func NewHomepageService(db *gorm.DB, applicationId int64, application ...*common_v1.ApplicationSummary) *HomepageService {
	s := &HomepageService{db: db, applicationId: applicationId}
	if len(application) > 0 {
		s.application = application[0]
	}
	return s
}

// GetHomepage 读取草稿 + 已发布 + 版本；无配置时回落内置默认首页。
func (s *HomepageService) GetHomepage(ctx context.Context, _ *home_v1.GetHomepageRequest) (*home_v1.Homepage, error) {
	var h model.Homepage
	err := s.db.WithContext(ctx).
		Select(model.HomepageFields).
		Where(model.HomepageColumnApplicationId, s.applicationId).
		First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.defaultHomepage(), nil
	}
	if err != nil {
		return nil, err
	}
	return s.homepageToPb(h), nil
}

// SaveDraft 保存草稿；expectedVersion 用于乐观锁，不触碰已发布版本。
func (s *HomepageService) SaveDraft(ctx context.Context, req *home_v1.SaveDraftRequest) (*home_v1.Homepage, error) {
	if err := validateHomepageJSON(req.ContentJson); err != nil {
		return nil, err
	}
	now := time.Now()
	operator := operatorFromContext(ctx)

	var h model.Homepage
	err := s.db.WithContext(ctx).
		Select(model.HomepageFields).
		Where(model.HomepageColumnApplicationId, s.applicationId).
		First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if req.ExpectedVersion > 0 {
			return nil, ErrHomepageVersionConflict
		}
		h = model.Homepage{
			Id:            krand.NewID(),
			ApplicationId: s.applicationId,
			DraftJson:     req.ContentJson,
			Version:       1,
			CreatedBy:     operator,
			UpdatedBy:     operator,
			CreateTime:    now,
			UpdateTime:    now,
		}
		if err := s.db.WithContext(ctx).Create(&h).Error; err != nil {
			// 并发首次创建撞唯一键：按冲突处理，让前端刷新。
			return nil, ErrHomepageVersionConflict
		}
		return s.homepageToPb(h), nil
	}
	if err != nil {
		return nil, err
	}
	if req.ExpectedVersion > 0 && req.ExpectedVersion != h.Version {
		return nil, ErrHomepageVersionConflict
	}

	updates := map[string]any{
		model.HomepageColumnDraftJson:  req.ContentJson,
		model.HomepageColumnVersion:    h.Version + 1,
		model.HomepageColumnUpdatedBy:  operator,
		model.HomepageColumnUpdateTime: now,
	}
	res := s.db.WithContext(ctx).Model(&model.Homepage{}).
		Where(model.HomepageColumnId, h.Id).
		Where(model.HomepageColumnVersion, h.Version).
		Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrHomepageVersionConflict
	}

	h.DraftJson = req.ContentJson
	h.Version++
	h.UpdatedBy = operator
	h.UpdateTime = now
	return s.homepageToPb(h), nil
}

// PublishHomepage 把当前草稿复制为已发布版本，记录发布时间。
func (s *HomepageService) PublishHomepage(ctx context.Context, req *home_v1.PublishHomepageRequest) (*home_v1.Homepage, error) {
	now := time.Now()
	operator := operatorFromContext(ctx)

	var h model.Homepage
	err := s.db.WithContext(ctx).
		Select(model.HomepageFields).
		Where(model.HomepageColumnApplicationId, s.applicationId).
		First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && strings.TrimSpace(h.DraftJson) == "") {
		return nil, ErrHomepageNothingToPublish
	}
	if err != nil {
		return nil, err
	}
	if req.ExpectedVersion > 0 && req.ExpectedVersion != h.Version {
		return nil, ErrHomepageVersionConflict
	}

	res := s.db.WithContext(ctx).Model(&model.Homepage{}).
		Where(model.HomepageColumnId, h.Id).
		Where(model.HomepageColumnVersion, h.Version).
		Updates(map[string]any{
			model.HomepageColumnPublishedJson: h.DraftJson,
			model.HomepageColumnPublishedAt:   now,
			model.HomepageColumnVersion:       h.Version + 1,
			model.HomepageColumnUpdatedBy:     operator,
			model.HomepageColumnUpdateTime:    now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrHomepageVersionConflict
	}

	h.PublishedJson = h.DraftJson
	h.PublishedAt = &now
	h.Version++
	h.UpdatedBy = operator
	h.UpdateTime = now
	slog.Info("homepage published", "applicationId", s.applicationId, "version", h.Version, "operator", operator)
	return s.homepageToPb(h), nil
}

// GetPublishedHomepage 只返回已发布版本；未发布返回空内容。
func (s *HomepageService) GetPublishedHomepage(ctx context.Context, _ *home_v1.GetPublishedHomepageRequest) (*home_v1.PublishedHomepage, error) {
	var h model.Homepage
	err := s.db.WithContext(ctx).
		Select(model.HomepageFields).
		Where(model.HomepageColumnApplicationId, s.applicationId).
		First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &home_v1.PublishedHomepage{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := &home_v1.PublishedHomepage{
		ContentJson: h.PublishedJson,
		Version:     h.Version,
	}
	if h.PublishedAt != nil {
		out.PublishedAt = timestamppb.New(*h.PublishedAt)
	}
	return out, nil
}

func (s *HomepageService) defaultHomepage() *home_v1.Homepage {
	return &home_v1.Homepage{
		Id:          "0",
		DraftJson:   DefaultHomepageJSON,
		Version:     0,
		Application: s.application,
	}
}

func (s *HomepageService) homepageToPb(h model.Homepage) *home_v1.Homepage {
	out := &home_v1.Homepage{
		Id:            toString(h.Id),
		DraftJson:     h.DraftJson,
		PublishedJson: h.PublishedJson,
		Version:       h.Version,
		UpdatedBy:     h.UpdatedBy,
		Application:   s.application,
	}
	if h.PublishedAt != nil {
		out.PublishedAt = timestamppb.New(*h.PublishedAt)
	}
	if !h.UpdateTime.IsZero() {
		out.UpdateTime = timestamppb.New(h.UpdateTime)
	}
	return out
}

// validateHomepageJSON 校验：非空、≤200KB、是 JSON 对象。
func validateHomepageJSON(content string) error {
	if strings.TrimSpace(content) == "" {
		return ErrInvalidHomepageJson
	}
	if len(content) > maxHomepageContentBytes {
		return ErrHomepageTooLarge
	}
	var value any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return ErrInvalidHomepageJson
	}
	if _, ok := value.(map[string]any); !ok {
		return ErrInvalidHomepageJson
	}
	return nil
}
