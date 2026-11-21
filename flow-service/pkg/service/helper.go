package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/linjunhao1997/contrib/pkg/authz"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	"github.com/spf13/cast"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"flow-service/conf"
	"flow-service/pkg/model"
)

const (
	defaultCurrentPage = 1
	defaultPageSize    = 10
	maxPageSize        = 100
)

// Host 是 flow 各业务服务的公共依赖：库、固定应用 id、归属应用摘要、建档策略。
type Host struct {
	DB                *gorm.DB
	AppId             int64
	Summary           *common_v1.ApplicationSummary
	BootstrapAllRoles bool
	DefaultRoleCode   string
}

func NewHost(db *gorm.DB, appId int64, summary *common_v1.ApplicationSummary, cfg *conf.Config) *Host {
	return &Host{
		DB:                db,
		AppId:             appId,
		Summary:           summary,
		BootstrapAllRoles: cfg.Flow.BootstrapAllRoles,
		DefaultRoleCode:   cfg.Flow.DefaultRoleCode,
	}
}

func limitAndOffset(currentPage, pageSize int64) (limit, offset int) {
	if currentPage <= 0 {
		currentPage = defaultCurrentPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	} else if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return int(pageSize), int((currentPage - 1) * pageSize)
}

func pageOf(currentPage, pageSize, total int64) *common_v1.Page {
	if currentPage <= 0 {
		currentPage = defaultCurrentPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	} else if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}
	return &common_v1.Page{
		CurrentPage: currentPage,
		PageSize:    pageSize,
		Total:       total,
		TotalPages:  totalPages,
	}
}

// operatorFromContext 取审计身份（只读回传用）。
func operatorFromContext(ctx context.Context) string {
	return cast.ToString(authz.GetAccountId(ctx))
}

// accountIdFromContext 取登录账号；缺失即未认证。
func accountIdFromContext(ctx context.Context) (int64, error) {
	id := authz.GetAccountId(ctx)
	if id <= 0 {
		return 0, ErrUnauthenticated
	}
	return id, nil
}

// usernameFromContext 从入站 JWT 解出姓名，仅用于建档时写姓名快照（不验签、不做判权）。
func usernameFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, v := range md.Get("authorization") {
		token := strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
		if token == "" {
			continue
		}
		if claims, err := authz.ParseUnverified(token); err == nil && claims != nil {
			return claims.Username
		}
	}
	return ""
}

// now 是统一写入时钟（审计字段服务端注入，宪法 XII）。
func now() time.Time { return time.Now() }

func tsOf(t time.Time) *timestamppb.Timestamp { return timestamppb.New(t) }

func tsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// questionOption 是 options 列（JSON）与 proto 之间共享的结构。
type questionOption struct {
	Id    string `json:"id"`
	Label string `json:"label"`
	Rec   bool   `json:"rec"`
}

func optionsToJSON(opts []questionOption) string {
	if len(opts) == 0 {
		return "[]"
	}
	b, err := json.Marshal(opts)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func optionsFromJSON(raw string) []questionOption {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []questionOption
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// ---- 工作角色读取 ----

func (h *Host) roleByCode(ctx context.Context, code string) (*model.WorkRole, error) {
	var r model.WorkRole
	err := h.DB.WithContext(ctx).Select(model.WorkRoleFields).
		Where(model.WorkRoleColumnApplicationId, h.AppId).
		Where(model.WorkRoleColumnCode, code).
		Where(model.WorkRoleColumnStatus, "enabled").
		First(&r).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (h *Host) roleByID(ctx context.Context, id int64) (*model.WorkRole, error) {
	var r model.WorkRole
	err := h.DB.WithContext(ctx).Select(model.WorkRoleFields).
		Where(model.WorkRoleColumnApplicationId, h.AppId).
		Where(model.WorkRoleColumnId, id).
		First(&r).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (h *Host) enabledRoles(ctx context.Context) ([]model.WorkRole, error) {
	var out []model.WorkRole
	err := h.DB.WithContext(ctx).Select(model.WorkRoleFields).
		Where(model.WorkRoleColumnApplicationId, h.AppId).
		Where(model.WorkRoleColumnStatus, "enabled").
		Order(model.WorkRoleColumnSort + " ASC").
		Find(&out).Error
	return out, err
}

// rolesOfMember 取员工能担任的全部工作角色（按 sort 升序）。
func (h *Host) rolesOfMember(ctx context.Context, memberId int64) ([]model.WorkRole, error) {
	var out []model.WorkRole
	err := h.DB.WithContext(ctx).Select("r."+model.WorkRoleColumnId+" AS id",
		"r."+model.WorkRoleColumnApplicationId+" AS application_id",
		"r."+model.WorkRoleColumnCode+" AS code",
		"r."+model.WorkRoleColumnName+" AS name",
		"r."+model.WorkRoleColumnStage+" AS stage",
		"r."+model.WorkRoleColumnSort+" AS sort",
		"r."+model.WorkRoleColumnStatus+" AS status").
		Table(model.WorkRole{}.TableName()+" AS r").
		Joins("JOIN "+model.MemberRole{}.TableName()+" AS mr ON mr.role_id = r.id").
		Where("mr.member_id = ?", memberId).
		Where("r.application_id = ?", h.AppId).
		Where("r.status = ?", "enabled").
		Order("r.sort ASC").
		Scan(&out).Error
	return out, err
}

// ensureTaskQuestionLink 维护任务↔问题追溯关系（幂等）。
func ensureTaskQuestionLink(tx *gorm.DB, taskId, questionId int64) error {
	var cnt int64
	if err := tx.Table(model.TaskQuestionLink{}.TableName()).
		Where("task_id = ? AND question_id = ?", taskId, questionId).
		Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	return tx.Create(&model.TaskQuestionLink{
		TaskId:     taskId,
		QuestionId: questionId,
		CreateTime: now(),
	}).Error
}

// appendEvent 写一条留痕（只追加）。
func (h *Host) appendEvent(ctx context.Context, tx *gorm.DB, actor int64, action, entity string, entityId int64, detail any) error {
	raw := ""
	if detail != nil {
		if b, err := json.Marshal(detail); err == nil {
			raw = string(b)
		}
	}
	return tx.WithContext(ctx).Create(&model.Event{
		ApplicationId:  h.AppId,
		ActorAccountId: actor,
		Action:         action,
		Entity:         entity,
		EntityId:       entityId,
		Detail:         raw,
		CreateTime:     ptrTime(now()),
	}).Error
}

func ptrTime(t time.Time) *time.Time { return &t }
