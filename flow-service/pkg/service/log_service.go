package service

import (
	"context"

	flow_v1 "github.com/linjunhao1997/idl/pb/flow"
	"github.com/spf13/cast"

	"flow-service/pkg/model"
)

// LogService 提供只增不改的留痕查询。
type LogService struct {
	host *Host
}

func NewLogService(host *Host) *LogService { return &LogService{host: host} }

func (s *LogService) ListEvents(ctx context.Context, req *flow_v1.ListEventsRequest) (*flow_v1.ListEventsResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := s.host.DB.WithContext(ctx).Model(&model.Event{}).
		Where(model.EventColumnApplicationId, s.host.AppId)
	if e := req.GetEntity(); e != "" {
		q = q.Where(model.EventColumnEntity, e)
	}
	if id := req.GetEntityId(); id > 0 {
		q = q.Where(model.EventColumnEntityId, id)
	}

	var events []model.Event
	if err := q.Select(model.EventFields).Order(model.EventColumnId + " DESC").Limit(limit).Find(&events).Error; err != nil {
		return nil, err
	}

	names, err := s.host.memberNameMap(ctx, events)
	if err != nil {
		return nil, err
	}
	out := make([]*flow_v1.Event, 0, len(events))
	for i := range events {
		at := ""
		if events[i].CreateTime != nil {
			at = events[i].CreateTime.Format("2006-01-02 15:04:05")
		}
		out = append(out, &flow_v1.Event{
			Id:        cast.ToString(events[i].Id),
			At:        at,
			Actor:     cast.ToString(events[i].ActorAccountId),
			ActorName: names[events[i].ActorAccountId],
			Action:    eventActionLabel(events[i].Action),
			Entity:    events[i].Entity,
			EntityId:  cast.ToString(events[i].EntityId),
			Detail:    events[i].Detail,
		})
	}
	return &flow_v1.ListEventsResponse{Events: out}, nil
}

func (h *Host) memberNameMap(ctx context.Context, events []model.Event) (map[int64]string, error) {
	ids := make([]int64, 0, len(events))
	seen := map[int64]struct{}{}
	for i := range events {
		id := events[i].ActorAccountId
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var members []model.Member
	if err := h.DB.WithContext(ctx).Select(model.MemberColumnAccountId, model.MemberColumnName).
		Where(model.MemberColumnAccountId+" IN ?", ids).Find(&members).Error; err != nil {
		return nil, err
	}
	for i := range members {
		out[members[i].AccountId] = members[i].Name
	}
	return out, nil
}

// eventActionLabel 把动作码翻译成人话（前端/agent 直接读）。
func eventActionLabel(action string) string {
	switch action {
	case model.EventActionRoleSwitch:
		return "切换角色"
	case model.EventActionTaskCreate:
		return "创建任务"
	case model.EventActionTaskClose:
		return "完成任务"
	case model.EventActionTaskDiscard:
		return "作废任务"
	case model.EventActionQuestionRaise:
		return "抛出问题"
	case model.EventActionQuestionEscalate:
		return "升级问题"
	case model.EventActionQuestionAnswer:
		return "闭环问题"
	case model.EventActionQuestionSelfAnswer:
		return "自问自答闭环"
	case model.EventActionQuestionAutoClose:
		return "升级链回填"
	case model.EventActionMemberBootstrap:
		return "首次建档"
	default:
		if action == "" {
			return "未知动作"
		}
		return action
	}
}
