package service

import (
	"context"
	"sort"
	"strings"
	"time"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	flow_v1 "github.com/linjunhao1997/idl/pb/flow"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"flow-service/pkg/model"
)

// TaskService 实现任务清单/详情/建/完成/作废。
// 注意：任务的 ready/blocked 是查询期派生，任何写路径都不要去同步任务状态。
type TaskService struct {
	host *Host
}

func NewTaskService(host *Host) *TaskService { return &TaskService{host: host} }

func (s *TaskService) ListMyTasks(ctx context.Context, req *flow_v1.ListMyTasksRequest) (*flow_v1.ListTasksResponse, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	role := currentRole(m, roles)
	scope := strings.TrimSpace(req.GetScope())
	if scope == "" {
		scope = "mine"
	}
	if scope != "mine" && scope != "created" && scope != "upstream" && scope != "all" {
		return nil, ErrTaskScopeInvalid
	}
	if scope == "all" && (role == nil || (role.Code != "boss" && role.Code != "pjm")) {
		return nil, ErrAllTasksForbidden
	}
	if scope == "mine" && role == nil {
		resp := &flow_v1.ListTasksResponse{}
		resp.Page = pageOf(req.GetCurrentPage(), req.GetPageSize(), 0)
		return resp, nil
	}

	resp := &flow_v1.ListTasksResponse{}
	if role != nil {
		resp.RoleId = cast.ToString(role.Id)
		resp.RoleName = role.Name
	}

	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := limitAndOffset(req.GetCurrentPage(), req.GetPageSize())
	q := s.host.DB.WithContext(ctx).Model(&model.Task{}).
		Where(model.TaskColumnApplicationId, s.host.AppId).
		Where(model.TaskColumnIsDeleted, false)
	switch scope {
	case "created":
		q = q.Where(model.TaskColumnCreatedBy, cast.ToString(accountId))
	case "upstream":
		if role == nil {
			q = q.Where("1 = 0")
		} else {
			sub := s.host.DB.Table(model.WorkRole{}.TableName()).
				Select(model.WorkRoleColumnId).
				Where(model.WorkRoleColumnApplicationId, s.host.AppId).
				Where(model.WorkRoleColumnSort+" < ?", role.Sort)
			q = q.Where(model.TaskColumnRoleId+" IN (?)", sub)
		}
	case "all":
		// 全链路只读：不按当前角色过滤，权限在上方已校验。
	default:
		// v1.2：任务归人，同时限定当前角色；下游不能替上游执行。
		q = q.Where(model.TaskColumnOwnerId, accountId).
			Where(model.TaskColumnRoleId, role.Id)
	}
	if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
		q = q.Where(model.TaskColumnTitle+" LIKE ?", "%"+kw+"%")
	}
	switch req.GetState() {
	case model.TaskStateReady:
		q = q.Where(model.TaskColumnStatus, model.TaskStatusOpen).
			Where("NOT EXISTS (SELECT 1 FROM " + model.Question{}.TableName() +
				" q WHERE q.task_id = " + model.Task{}.TableName() + ".id AND q.status = 'open' AND q.is_deleted = 0)")
	case model.TaskStateBlocked:
		q = q.Where(model.TaskColumnStatus, model.TaskStatusOpen).
			Where("EXISTS (SELECT 1 FROM " + model.Question{}.TableName() +
				" q WHERE q.task_id = " + model.Task{}.TableName() + ".id AND q.status = 'open' AND q.is_deleted = 0)")
	case model.TaskStateDone:
		q = q.Where(model.TaskColumnStatus, model.TaskStatusDone)
	case model.TaskStateDiscarded:
		q = q.Where(model.TaskColumnStatus, model.TaskStatusDiscarded)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var tasks []model.Task
	if err := q.Select(model.TaskFields).Order(model.TaskColumnId + " DESC").
		Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(tasks))
	parentIds := make([]int64, 0, len(tasks))
	ownerIds := make([]int64, 0, len(tasks))
	for i := range tasks {
		ids = append(ids, tasks[i].Id)
		if tasks[i].ParentTaskId > 0 {
			parentIds = append(parentIds, tasks[i].ParentTaskId)
		}
		if tasks[i].OwnerId > 0 {
			ownerIds = append(ownerIds, tasks[i].OwnerId)
		}
	}
	blockers, err := s.host.blockersOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	roleNames, err := s.host.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	ownerNames, err := s.host.memberNameMapByAccountIDs(ctx, ownerIds)
	if err != nil {
		return nil, err
	}
	parentTitles, err := s.host.taskTitleMapByIDs(ctx, parentIds)
	if err != nil {
		return nil, err
	}
	childStats, err := s.host.childStatsOf(ctx, ids)
	if err != nil {
		return nil, err
	}

	resp.Page = pageOf(req.GetCurrentPage(), req.GetPageSize(), total)
	for i := range tasks {
		meta := taskMeta{
			roleName:        roleNames[tasks[i].RoleId],
			createdRoleName: roleNameOf(roleNames, tasks[i].CreatedRoleId),
			ownerName:       ownerNames[tasks[i].OwnerId],
			parentTitle:     parentTitles[tasks[i].ParentTaskId],
		}
		if st, ok := childStats[tasks[i].Id]; ok {
			meta.childTotal, meta.childDone, meta.childOpen = st[0], st[1], st[2]
		}
		resp.Tasks = append(resp.Tasks, taskToPb(&tasks[i], meta, blockers[tasks[i].Id]))
	}
	if role != nil && scope == "mine" {
		ready, blocked, err := s.host.taskStatsForOwner(ctx, role.Id, accountId)
		if err != nil {
			return nil, err
		}
		resp.ReadyCount = int32(ready)
		resp.BlockedCount = int32(blocked)
	}
	return resp, nil
}

func (s *TaskService) GetTask(ctx context.Context, req *common_v1.GetRequest) (*flow_v1.Task, error) {
	if _, _, err := s.host.ensureMember(ctx); err != nil {
		return nil, err
	}
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	roleNames, err := s.host.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	blockers, err := s.host.blockersOf(ctx, []int64{t.Id})
	if err != nil {
		return nil, err
	}
	meta := taskMeta{
		roleName:        roleNames[t.RoleId],
		createdRoleName: roleNameOf(roleNames, t.CreatedRoleId),
	}
	if t.OwnerId > 0 {
		ownerNames, err := s.host.memberNameMapByAccountIDs(ctx, []int64{t.OwnerId})
		if err != nil {
			return nil, err
		}
		meta.ownerName = ownerNames[t.OwnerId]
	}
	if t.ParentTaskId > 0 {
		parentTitles, err := s.host.taskTitleMapByIDs(ctx, []int64{t.ParentTaskId})
		if err != nil {
			return nil, err
		}
		meta.parentTitle = parentTitles[t.ParentTaskId]
	}
	childStats, err := s.host.childStatsOf(ctx, []int64{t.Id})
	if err != nil {
		return nil, err
	}
	if st, ok := childStats[t.Id]; ok {
		meta.childTotal, meta.childDone, meta.childOpen = st[0], st[1], st[2]
	}
	return taskToPb(t, meta, blockers[t.Id]), nil
}

func (s *TaskService) CreateTask(ctx context.Context, req *flow_v1.CreateTaskRequest) (*flow_v1.Task, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	creatorRole := currentRole(m, roles)
	role := creatorRole
	if role == nil {
		return nil, ErrRoleNotFound
	}
	// v1.2：任务只能归当前人 + 当前角色，不允许把任务改派给别人。
	if code := strings.TrimSpace(req.GetRoleCode()); code != "" {
		held := findRole(roles, code)
		if held == nil {
			return nil, ErrRoleNotHeld
		}
		if held.Id != role.Id {
			return nil, ErrTaskReassignForbidden
		}
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}

	parentId := cast.ToInt64(req.GetParentTaskId())
	var parent *model.Task
	if parentId > 0 {
		parent, err = s.host.taskByID(ctx, parentId)
		if err != nil {
			return nil, err
		}
		parentRole, err := s.host.roleByID(ctx, parent.RoleId)
		if err != nil {
			return nil, err
		}
		if parent.RoleId == role.Id || parentRole.Sort >= role.Sort {
			return nil, ErrParentTaskInvalid
		}
		openQ, err := s.host.openQuestionCount(ctx, 0, parent.Id)
		if err != nil {
			return nil, err
		}
		if openQ > 0 {
			return nil, ErrParentTaskBlocked
		}
	}

	t := model.Task{
		ApplicationId: s.host.AppId,
		RoleId:        role.Id,
		OwnerId:       accountId,
		ParentTaskId:  parentId,
		Title:         strings.TrimSpace(req.GetTitle()),
		Brief:         req.GetBrief(),
		Status:        model.TaskStatusOpen,
		Version:       0,
		IsDeleted:     false,
		CreatedBy:     cast.ToString(accountId),
		CreatedRoleId: creatorRole.Id,
		UpdatedBy:     cast.ToString(accountId),
		CreateTime:    now(),
		UpdateTime:    now(),
	}
	tx := s.host.DB.WithContext(ctx).Begin()
	if err := tx.Create(&t).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	detail := map[string]any{"role_id": t.RoleId, "title": t.Title,
		"created_role_id": t.CreatedRoleId, "created_role_name": creatorRole.Name,
		"owner_id": t.OwnerId}
	if t.ParentTaskId > 0 {
		detail["parent_task_id"] = t.ParentTaskId
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionTaskCreate, model.EntityTask, t.Id, detail); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	meta := taskMeta{roleName: role.Name, createdRoleName: creatorRole.Name, ownerName: m.Name}
	if parent != nil {
		meta.parentTitle = parent.Title
	}
	return taskToPb(&t, meta, nil), nil
}

func (s *TaskService) CloseTask(ctx context.Context, req *common_v1.GetRequest) (*emptypb.Empty, error) {
	if _, _, err := s.host.ensureMember(ctx); err != nil {
		return nil, err
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if t.OwnerId <= 0 || t.OwnerId != accountId {
		return nil, ErrTaskOwnerMismatch
	}
	if t.Status == model.TaskStatusDone {
		return &emptypb.Empty{}, nil
	}
	if t.Status != model.TaskStatusOpen {
		return nil, ErrTaskEnded
	}
	// v1.2：父任务必须等直接下级任务全部完成（作废的下级视为已取消，不阻塞）。
	var openChildren int64
	if err := s.host.DB.WithContext(ctx).Model(&model.Task{}).
		Where(model.TaskColumnParentTaskId, t.Id).
		Where(model.TaskColumnStatus, model.TaskStatusOpen).
		Where(model.TaskColumnIsDeleted, false).
		Count(&openChildren).Error; err != nil {
		return nil, err
	}
	if openChildren > 0 {
		return nil, ErrChildTasksOpen
	}
	openQ, err := s.host.openQuestionCount(ctx, 0, t.Id)
	if err != nil {
		return nil, err
	}
	if openQ > 0 {
		return nil, ErrTaskBlocked
	}

	tx := s.host.DB.WithContext(ctx).Begin()
	res := tx.Model(&model.Task{}).
		Where(model.TaskColumnId, t.Id).
		Where(model.TaskColumnStatus, model.TaskStatusOpen).
		Updates(map[string]any{
			model.TaskColumnStatus:     model.TaskStatusDone,
			model.TaskColumnVersion:    t.Version + 1,
			model.TaskColumnUpdatedBy:  cast.ToString(accountId),
			model.TaskColumnUpdateTime: now(),
		})
	if res.Error != nil {
		tx.Rollback()
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return nil, ErrTaskNotOpen
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionTaskClose, model.EntityTask, t.Id, nil); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *TaskService) DiscardTask(ctx context.Context, req *common_v1.GetRequest) (*emptypb.Empty, error) {
	if _, _, err := s.host.ensureMember(ctx); err != nil {
		return nil, err
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if t.OwnerId <= 0 || t.OwnerId != accountId {
		return nil, ErrTaskOwnerMismatch
	}
	if t.Status == model.TaskStatusDiscarded {
		return &emptypb.Empty{}, nil
	}

	tx := s.host.DB.WithContext(ctx).Begin()
	res := tx.Model(&model.Task{}).
		Where(model.TaskColumnId, t.Id).
		Where(model.TaskColumnStatus, model.TaskStatusOpen).
		Updates(map[string]any{
			model.TaskColumnStatus:     model.TaskStatusDiscarded,
			model.TaskColumnVersion:    t.Version + 1,
			model.TaskColumnUpdatedBy:  cast.ToString(accountId),
			model.TaskColumnUpdateTime: now(),
		})
	if res.Error != nil {
		tx.Rollback()
		return nil, res.Error
	}
	// 只有任务确实从 open 翻成 discarded，才算作废成功；
	// 否则说明它已被完成/被别处作废，必须报错而不是返回"假成功"。
	if res.RowsAffected == 0 {
		tx.Rollback()
		return nil, ErrTaskEnded
	}
	// 任务作废后，挂在上面的 open 问题一并丢弃，避免永远留在"待我回答"。
	if err := tx.Model(&model.Question{}).
		Where(model.QuestionColumnTaskId, t.Id).
		Where(model.QuestionColumnStatus, model.QuestionStatusOpen).
		Updates(map[string]any{
			model.QuestionColumnStatus:     model.QuestionStatusDiscarded,
			model.QuestionColumnUpdatedBy:  cast.ToString(accountId),
			model.QuestionColumnUpdateTime: now(),
		}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionTaskDiscard, model.EntityTask, t.Id, nil); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GetMyMetrics 返回当前角色的闭环质量指标（只读）。
// 口径刻意区分“任务队列是否在转”和“问题闭环是否可信”：
// 任务数看 open/ready/blocked，问题数看闭环时长、自答率、重开率、闭环后停滞。
func (s *TaskService) GetMyMetrics(ctx context.Context, _ *emptypb.Empty) (*flow_v1.FlowMetrics, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	role := currentRole(m, roles)
	if role == nil {
		return nil, ErrRoleNotFound
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}

	taskBase := func() *gorm.DB {
		return s.host.DB.WithContext(ctx).Table(model.Task{}.TableName()+" AS t").
			Where("t.application_id = ?", s.host.AppId).
			Where("t.role_id = ?", role.Id).
			Where("t.owner_id = ?", accountId).
			Where("t.is_deleted = ?", false)
	}
	var openTasks int64
	if err := taskBase().Where("t.status = ?", model.TaskStatusOpen).Count(&openTasks).Error; err != nil {
		return nil, err
	}
	var readyTasks int64
	if err := taskBase().Where("t.status = ?", model.TaskStatusOpen).
		Where("NOT EXISTS (SELECT 1 FROM " + model.Question{}.TableName() +
			" q WHERE q.task_id = t.id AND q.status = 'open' AND q.is_deleted = 0)").
		Count(&readyTasks).Error; err != nil {
		return nil, err
	}
	blockedTasks := openTasks - readyTasks

	questionBase := func() *gorm.DB {
		return s.host.DB.WithContext(ctx).Model(&model.Question{}).
			Where(model.QuestionColumnApplicationId, s.host.AppId).
			Where(model.QuestionColumnToRoleId, role.Id).
			Where(model.QuestionColumnIsDeleted, false)
	}
	var openQuestions, answeredQuestions, selfAnswered int64
	if err := questionBase().Where(model.QuestionColumnStatus, model.QuestionStatusOpen).Count(&openQuestions).Error; err != nil {
		return nil, err
	}
	if err := questionBase().Where(model.QuestionColumnStatus, model.QuestionStatusAnswered).Count(&answeredQuestions).Error; err != nil {
		return nil, err
	}
	if err := questionBase().Where(model.QuestionColumnStatus, model.QuestionStatusAnswered).
		Where(model.QuestionColumnSelfAnswer, true).Count(&selfAnswered).Error; err != nil {
		return nil, err
	}
	var reopened int64
	if err := questionBase().Where(model.QuestionColumnStatus, model.QuestionStatusAnswered).
		Select("COALESCE(SUM(" + model.QuestionColumnReopenCount + "), 0)").Scan(&reopened).Error; err != nil {
		return nil, err
	}

	// 闭环时长样本：只取 answered_at 非空的已闭环问题。
	var samples []struct {
		CreateTime time.Time  `gorm:"column:create_time"`
		AnsweredAt *time.Time `gorm:"column:answered_at"`
	}
	if err := questionBase().Where(model.QuestionColumnStatus, model.QuestionStatusAnswered).
		Select(model.QuestionColumnCreateTime + ", " + model.QuestionColumnAnsweredAt).
		Find(&samples).Error; err != nil {
		return nil, err
	}
	minutes := make([]float64, 0, len(samples))
	for _, sample := range samples {
		if sample.AnsweredAt == nil {
			continue
		}
		d := sample.AnsweredAt.Sub(sample.CreateTime).Minutes()
		if d >= 0 {
			minutes = append(minutes, d)
		}
	}
	avgMinutes, p50Minutes := 0.0, 0.0
	if len(minutes) > 0 {
		sort.Float64s(minutes)
		sum := 0.0
		for _, v := range minutes {
			sum += v
		}
		avgMinutes = sum / float64(len(minutes))
		mid := len(minutes) / 2
		if len(minutes)%2 == 0 {
			p50Minutes = (minutes[mid-1] + minutes[mid]) / 2
		} else {
			p50Minutes = minutes[mid]
		}
	}

	// 闭环后停滞：任务还 open，但已有问题闭环超过 7 天，说明“答了没推进”。
	var stalled int64
	if err := taskBase().Where("t.status = ?", model.TaskStatusOpen).
		Where("EXISTS (SELECT 1 FROM "+model.Question{}.TableName()+" q WHERE q.task_id = t.id"+
			" AND q.status = 'answered' AND q.answered_at IS NOT NULL AND q.answered_at < ? AND q.is_deleted = 0)",
			now().Add(-7*24*time.Hour)).
		Count(&stalled).Error; err != nil {
		return nil, err
	}

	reopenRate, selfRate := 0.0, 0.0
	if answeredQuestions > 0 {
		reopenRate = float64(reopened) / float64(answeredQuestions)
		selfRate = float64(selfAnswered) / float64(answeredQuestions)
	}
	return &flow_v1.FlowMetrics{
		RoleId:            cast.ToString(role.Id),
		RoleName:          role.Name,
		OpenTasks:         int32(openTasks),
		ReadyTasks:        int32(readyTasks),
		BlockedTasks:      int32(blockedTasks),
		OpenQuestions:     int32(openQuestions),
		AnsweredQuestions: int32(answeredQuestions),
		ReopenedQuestions: int32(reopened),
		AvgAnswerMinutes:  avgMinutes,
		P50AnswerMinutes:  p50Minutes,
		ReopenRate:        reopenRate,
		SelfAnswerRate:    selfRate,
		StalledTasks:      int32(stalled),
	}, nil
}

// ---- 共用读取 ----

func (h *Host) taskByID(ctx context.Context, id int64) (*model.Task, error) {
	if id <= 0 {
		return nil, ErrTaskNotFound
	}
	var t model.Task
	err := h.DB.WithContext(ctx).Select(model.TaskFields).
		Where(model.TaskColumnId, id).
		Where(model.TaskColumnApplicationId, h.AppId).
		Where(model.TaskColumnIsDeleted, false).
		First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (h *Host) roleNameMap(ctx context.Context) (map[int64]string, error) {
	var roles []model.WorkRole
	if err := h.DB.WithContext(ctx).Select(model.WorkRoleFields).
		Where(model.WorkRoleColumnApplicationId, h.AppId).Find(&roles).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(roles))
	for i := range roles {
		out[roles[i].Id] = roles[i].Name
	}
	return out, nil
}

// blockersOf 批量取任务下所有 open 问题，组装成阻塞摘要。
func (h *Host) blockersOf(ctx context.Context, taskIds []int64) (map[int64][]*flow_v1.TaskBlocker, error) {
	out := make(map[int64][]*flow_v1.TaskBlocker, len(taskIds))
	if len(taskIds) == 0 {
		return out, nil
	}
	var qs []model.Question
	if err := h.DB.WithContext(ctx).Select(model.QuestionFields).
		Where(model.QuestionColumnTaskId+" IN ?", taskIds).
		Where(model.QuestionColumnStatus, model.QuestionStatusOpen).
		Where(model.QuestionColumnIsDeleted, false).
		Order(model.QuestionColumnId + " ASC").Find(&qs).Error; err != nil {
		return nil, err
	}
	roleNames, err := h.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	for i := range qs {
		out[qs[i].TaskId] = append(out[qs[i].TaskId], &flow_v1.TaskBlocker{
			QuestionId: cast.ToString(qs[i].Id),
			Body:       qs[i].Body,
			ToRoleName: roleNames[qs[i].ToRoleId],
			AnswerKind: qs[i].AnswerKind,
			CreatedAt:  qs[i].CreateTime.Format("2006-01-02 15:04"),
		})
	}
	return out, nil
}

type taskMeta struct {
	roleName        string
	createdRoleName string
	ownerName       string
	parentTitle     string
	childTotal      int32
	childDone       int32
	childOpen       int32
}

func taskToPb(t *model.Task, meta taskMeta, blockers []*flow_v1.TaskBlocker) *flow_v1.Task {
	state := model.TaskStateReady
	switch t.Status {
	case model.TaskStatusDone:
		state = model.TaskStateDone
	case model.TaskStatusDiscarded:
		state = model.TaskStateDiscarded
	default:
		if len(blockers) > 0 {
			state = model.TaskStateBlocked
		}
	}
	return &flow_v1.Task{
		Id:              cast.ToString(t.Id),
		RoleId:          cast.ToString(t.RoleId),
		RoleName:        meta.roleName,
		Title:           t.Title,
		Brief:           t.Brief,
		Status:          t.Status,
		State:           state,
		OpenQuestions:   int32(len(blockers)),
		Blockers:        blockers,
		CreatedBy:       t.CreatedBy,
		CreatedRoleId:   cast.ToString(t.CreatedRoleId),
		CreatedRoleName: meta.createdRoleName,
		UpdatedBy:       t.UpdatedBy,
		CreateTime:      tsOf(t.CreateTime),
		UpdateTime:      tsOf(t.UpdateTime),
		ParentTaskId:    cast.ToString(t.ParentTaskId),
		ParentTaskTitle: meta.parentTitle,
		OwnerId:         cast.ToString(t.OwnerId),
		OwnerName:       meta.ownerName,
		ChildTotal:      meta.childTotal,
		ChildDone:       meta.childDone,
		ChildOpen:       meta.childOpen,
	}
}

func (h *Host) memberNameMapByAccountIDs(ctx context.Context, accountIds []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(accountIds))
	if len(accountIds) == 0 {
		return out, nil
	}
	var members []model.Member
	if err := h.DB.WithContext(ctx).Select([]string{model.MemberColumnAccountId, model.MemberColumnName}).
		Where(model.MemberColumnAccountId+" IN ?", accountIds).
		Where(model.MemberColumnIsDeleted, false).
		Find(&members).Error; err != nil {
		return nil, err
	}
	for i := range members {
		out[members[i].AccountId] = members[i].Name
	}
	return out, nil
}

func (h *Host) taskTitleMapByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var tasks []model.Task
	if err := h.DB.WithContext(ctx).Select([]string{model.TaskColumnId, model.TaskColumnTitle}).
		Where(model.TaskColumnId+" IN ?", ids).
		Where(model.TaskColumnApplicationId, h.AppId).
		Where(model.TaskColumnIsDeleted, false).
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	for i := range tasks {
		out[tasks[i].Id] = tasks[i].Title
	}
	return out, nil
}

// childStatsOf 返回 parent_task_id 的直接下级任务数、已完成数、仍 open 数。
func (h *Host) childStatsOf(ctx context.Context, parentIds []int64) (map[int64][3]int32, error) {
	out := make(map[int64][3]int32, len(parentIds))
	if len(parentIds) == 0 {
		return out, nil
	}
	type row struct {
		ParentTaskId int64
		Total        int64
		Done         int64
		Open         int64
	}
	var rows []row
	if err := h.DB.WithContext(ctx).Table(model.Task{}.TableName()).
		Select("parent_task_id, COUNT(*) AS total, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS done, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS open", model.TaskStatusDone, model.TaskStatusOpen).
		Where(model.TaskColumnApplicationId, h.AppId).
		Where(model.TaskColumnParentTaskId+" IN ?", parentIds).
		Where(model.TaskColumnIsDeleted, false).
		Group(model.TaskColumnParentTaskId).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ParentTaskId] = [3]int32{int32(r.Total), int32(r.Done), int32(r.Open)}
	}
	return out, nil
}

func roleNameOf(m map[int64]string, id int64) string {
	if id <= 0 {
		return ""
	}
	return m[id]
}

func currentRole(m *model.Member, roles []model.WorkRole) *model.WorkRole {
	for i := range roles {
		if roles[i].Id == m.CurrentRoleId {
			return &roles[i]
		}
	}
	if len(roles) > 0 {
		return &roles[0]
	}
	return nil
}

func findRole(roles []model.WorkRole, key string) *model.WorkRole {
	for i := range roles {
		if roles[i].Code == key || roles[i].Name == key || cast.ToString(roles[i].Id) == key {
			return &roles[i]
		}
	}
	return nil
}
