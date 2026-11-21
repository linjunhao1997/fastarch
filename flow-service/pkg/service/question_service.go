package service

import (
	"context"
	"strings"

	flow_v1 "github.com/linjunhao1997/idl/pb/flow"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"flow-service/pkg/model"
)

// QuestionService 是系统的引擎：一条 open 问题 = 对应任务被阻塞。
// 闭环只改问题表一行，任务状态由查询期派生，天然无需同步代码。
type QuestionService struct {
	host *Host
}

func NewQuestionService(host *Host) *QuestionService { return &QuestionService{host: host} }

func (s *QuestionService) ListMyQuestions(ctx context.Context, req *flow_v1.ListMyQuestionsRequest) (*flow_v1.ListQuestionsResponse, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	role := currentRole(m, roles)
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}

	limit, offset := limitAndOffset(req.GetCurrentPage(), req.GetPageSize())
	q := s.host.DB.WithContext(ctx).Model(&model.Question{}).
		Where(model.QuestionColumnApplicationId, s.host.AppId).
		Where(model.QuestionColumnIsDeleted, false)
	if req.GetScope() == "raised" {
		q = q.Where(model.QuestionColumnAskedBy, accountId)
	} else {
		if role == nil {
			return &flow_v1.ListQuestionsResponse{Page: pageOf(req.GetCurrentPage(), req.GetPageSize(), 0)}, nil
		}
		q = q.Where(model.QuestionColumnToRoleId, role.Id)
	}
	if tid := req.GetTaskId(); tid > 0 {
		q = q.Where(model.QuestionColumnTaskId, tid)
	}
	switch req.GetStatus() {
	case "", model.QuestionStatusOpen:
		q = q.Where(model.QuestionColumnStatus, model.QuestionStatusOpen)
	case model.QuestionStatusAnswered:
		q = q.Where(model.QuestionColumnStatus, model.QuestionStatusAnswered)
	case "all":
		// all = 未闭环 + 已闭环；作废问题默认不出现在任何列表里
		q = q.Where(model.QuestionColumnStatus+" IN ?",
			[]string{model.QuestionStatusOpen, model.QuestionStatusAnswered})
	default:
		// 未登记的状态一律按 open 处理，避免把全量问题当结果返回
		q = q.Where(model.QuestionColumnStatus, model.QuestionStatusOpen)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var qs []model.Question
	if err := q.Select(model.QuestionFields).Order(model.QuestionColumnId + " DESC").
		Limit(limit).Offset(offset).Find(&qs).Error; err != nil {
		return nil, err
	}

	resp := &flow_v1.ListQuestionsResponse{Page: pageOf(req.GetCurrentPage(), req.GetPageSize(), total)}
	roleNames, err := s.host.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	taskTitles, err := s.host.taskTitleMap(ctx, qs)
	if err != nil {
		return nil, err
	}
	for i := range qs {
		resp.Questions = append(resp.Questions, questionToPb(&qs[i], taskTitles[qs[i].TaskId], roleNames))
	}
	if role != nil {
		open, err := s.host.openQuestionCount(ctx, role.Id, 0)
		if err != nil {
			return nil, err
		}
		resp.OpenCount = int32(open)
	}
	return resp, nil
}

func (s *QuestionService) RaiseQuestion(ctx context.Context, req *flow_v1.RaiseQuestionRequest) (*flow_v1.Question, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	role := currentRole(m, roles)
	if role == nil {
		return nil, ErrRoleNotFound
	}
	t, err := s.host.taskByID(ctx, req.GetTaskId())
	if err != nil {
		return nil, err
	}
	if t.RoleId != role.Id {
		return nil, ErrRoleMismatch
	}
	if t.Status != model.TaskStatusOpen {
		return nil, ErrTaskNotOpen
	}
	toRole, err := s.host.roleByCode(ctx, strings.TrimSpace(req.GetToRoleCode()))
	if err != nil {
		return nil, err
	}
	if toRole.Id == role.Id {
		return nil, ErrSelfQuestion
	}
	kind := req.GetAnswerKind()
	if kind == "" {
		kind = model.AnswerKindDecision
	}
	if kind != model.AnswerKindDecision && kind != model.AnswerKindEvidence {
		return nil, ErrInvalidAnswerKind
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}

	opts := make([]questionOption, 0, len(req.GetOptions()))
	for _, o := range req.GetOptions() {
		if o.GetId() == "" || o.GetLabel() == "" {
			continue
		}
		opts = append(opts, questionOption{Id: o.GetId(), Label: o.GetLabel(), Rec: o.GetRec()})
	}

	ts := now()
	q := model.Question{
		ApplicationId:    s.host.AppId,
		TaskId:           t.Id,
		FromRoleId:       role.Id,
		ToRoleId:         toRole.Id,
		AskedBy:          accountId,
		Body:             strings.TrimSpace(req.GetBody()),
		Options:          optionsToJSON(opts),
		AnswerKind:       kind,
		Status:           model.QuestionStatusOpen,
		ParentQuestionId: 0,
		Version:          0,
		IsDeleted:        false,
		CreatedBy:        cast.ToString(accountId),
		UpdatedBy:        cast.ToString(accountId),
		CreateTime:       ts,
		UpdateTime:       ts,
	}

	tx := s.host.DB.WithContext(ctx).Begin()
	if err := tx.Create(&q).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := ensureTaskQuestionLink(tx, q.TaskId, q.Id); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionQuestionRaise, model.EntityQuestion, q.Id,
		map[string]any{"task_id": q.TaskId, "to_role": toRole.Code, "answer_kind": kind}); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	roleNames, err := s.host.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	return questionToPb(&q, t.Title, roleNames), nil
}

func (s *QuestionService) EscalateQuestion(ctx context.Context, req *flow_v1.EscalateQuestionRequest) (*flow_v1.Question, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	role := currentRole(m, roles)
	if role == nil {
		return nil, ErrRoleNotFound
	}
	parent, err := s.host.questionByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if parent.Status != model.QuestionStatusOpen {
		return nil, ErrQuestionClosed
	}
	if parent.ToRoleId != role.Id {
		return nil, ErrRoleMismatch
	}
	// 升级链深度上限 3：超过 3 层说明分工本身没定清，应改为当面定责，而不是继续往上转。
	depth, err := s.depthOf(ctx, parent.Id)
	if err != nil {
		return nil, err
	}
	if depth >= 3 {
		return nil, ErrEscalateTooDeep
	}
	toRole, err := s.host.roleByCode(ctx, strings.TrimSpace(req.GetToRoleCode()))
	if err != nil {
		return nil, err
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	opts := make([]questionOption, 0, len(req.GetOptions()))
	for _, o := range req.GetOptions() {
		if o.GetId() == "" || o.GetLabel() == "" {
			continue
		}
		opts = append(opts, questionOption{Id: o.GetId(), Label: o.GetLabel(), Rec: o.GetRec()})
	}
	if len(opts) == 0 {
		opts = optionsFromJSON(parent.Options)
	}

	t := now()
	child := model.Question{
		ApplicationId:    s.host.AppId,
		TaskId:           parent.TaskId,
		FromRoleId:       role.Id,
		ToRoleId:         toRole.Id,
		AskedBy:          accountId,
		Body:             strings.TrimSpace(req.GetBody()),
		Options:          optionsToJSON(opts),
		AnswerKind:       parent.AnswerKind,
		Status:           model.QuestionStatusOpen,
		ParentQuestionId: parent.Id,
		Version:          0,
		IsDeleted:        false,
		CreatedBy:        cast.ToString(accountId),
		UpdatedBy:        cast.ToString(accountId),
		CreateTime:       t,
		UpdateTime:       t,
	}
	tx := s.host.DB.WithContext(ctx).Begin()
	if err := tx.Create(&child).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := ensureTaskQuestionLink(tx, child.TaskId, child.Id); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionQuestionEscalate, model.EntityQuestion, child.Id,
		map[string]any{"parent_question_id": parent.Id, "to_role": toRole.Code}); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	roleNames, err := s.host.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	taskTitles, err := s.host.taskTitleMap(ctx, []model.Question{child})
	if err != nil {
		return nil, err
	}
	return questionToPb(&child, taskTitles[child.TaskId], roleNames), nil
}

func (s *QuestionService) AnswerQuestion(ctx context.Context, req *flow_v1.AnswerQuestionRequest) (*flow_v1.Question, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	role := currentRole(m, roles)
	if role == nil {
		return nil, ErrRoleNotFound
	}
	q, err := s.host.questionByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if q.Status != model.QuestionStatusOpen {
		return nil, ErrQuestionClosed
	}
	if q.ToRoleId != role.Id {
		return nil, ErrRoleMismatch
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 证据门槛：evidence 类问题不填证据不得闭环（3001001）。
	// v1.1 起还要校验证据形态（3001023），避免“done / 已修好了”这种空头承诺。
	evidence := strings.TrimSpace(req.GetEvidence())
	if q.AnswerKind == model.AnswerKindEvidence {
		if evidence == "" {
			return nil, ErrEvidenceRequired
		}
		if err := validateEvidence(evidence); err != nil {
			return nil, err
		}
	}

	opts := optionsFromJSON(q.Options)
	picked := strings.TrimSpace(req.GetOptionId())
	text := strings.TrimSpace(req.GetText())
	answer := ""
	if picked != "" {
		var hit *questionOption
		for i := range opts {
			if opts[i].Id == picked {
				hit = &opts[i]
				break
			}
		}
		if hit == nil {
			return nil, ErrOptionNotFound
		}
		answer = hit.Label
		if text != "" {
			answer += "；" + text
		}
	} else {
		if text == "" {
			return nil, ErrAnswerRequired
		}
		answer = text
	}

	// self 是"同一账号"语义：同一个人换了角色回答自己抛出/上报的问题，审计上必须留痕。
	// 但报表需要分开两类，所以再派生 crossRole：问的人角色 ≠ 答的人角色（一人多角色的正常协作）。
	self := q.AskedBy != 0 && q.AskedBy == accountId
	crossRole := self && q.FromRoleId != role.Id
	t := now()
	updates := map[string]any{
		model.QuestionColumnAnswer:     answer,
		model.QuestionColumnPicked:     picked,
		model.QuestionColumnEvidence:   evidence,
		model.QuestionColumnStatus:     model.QuestionStatusAnswered,
		model.QuestionColumnAnsweredBy: accountId,
		model.QuestionColumnAnsweredAt: &t,
		model.QuestionColumnSelfAnswer: self,
		model.QuestionColumnVersion:    q.Version + 1,
		model.QuestionColumnUpdatedBy:  cast.ToString(accountId),
		model.QuestionColumnUpdateTime: &t,
	}

	tx := s.host.DB.WithContext(ctx).Begin()
	res := tx.Model(&model.Question{}).
		Where(model.QuestionColumnId, q.Id).
		Where(model.QuestionColumnStatus, model.QuestionStatusOpen).
		Updates(updates)
	if res.Error != nil {
		tx.Rollback()
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return nil, ErrQuestionClosed
	}
	action := model.EventActionQuestionAnswer
	switch {
	case self && !crossRole:
		action = model.EventActionQuestionSelfAnswer
	case self:
		action = model.EventActionQuestionSelfCrossRole
	}
	if err := s.host.appendEvent(ctx, tx, accountId, action, model.EntityQuestion, q.Id,
		map[string]any{"task_id": q.TaskId, "picked": picked, "answer_kind": q.AnswerKind, "evidence": evidence,
			"self_answer": self, "cross_role": crossRole, "from_role_id": q.FromRoleId, "answer_role_id": role.Id}); err != nil {
		tx.Rollback()
		return nil, err
	}
	// 升级链回填：子问题闭环后逐级向上；
	// 只有父问题已无其它 open 子问题时才自动闭环，最早阻塞点随之解除。
	if err := s.cascadeUp(ctx, tx, q.ParentQuestionId, q.Id, accountId); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	q.Answer = answer
	q.Picked = picked
	q.Evidence = evidence
	q.Status = model.QuestionStatusAnswered
	q.AnsweredBy = accountId
	q.AnsweredAt = &t
	q.SelfAnswer = self
	q.Version++
	roleNames, err := s.host.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	taskTitles, err := s.host.taskTitleMap(ctx, []model.Question{*q})
	if err != nil {
		return nil, err
	}
	// 重新读一次，保证回填后的父问题信息与库一致（本级只回本级）。
	if fresh, err := s.host.questionByID(ctx, q.Id); err == nil {
		q = fresh
	}
	return questionToPb(q, taskTitles[q.TaskId], roleNames), nil
}

// ReopenQuestion 把已闭环问题打回 open：任务因派生自动重新 blocked，无需改任务状态。
// 权限：原提问人，或当前角色是老板（兜底）；后者会在事件里标 boss_override。
func (s *QuestionService) ReopenQuestion(ctx context.Context, req *flow_v1.ReopenQuestionRequest) (*flow_v1.Question, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	role := currentRole(m, roles)
	if role == nil {
		return nil, ErrRoleNotFound
	}
	q, err := s.host.questionByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if q.Status != model.QuestionStatusAnswered {
		return nil, ErrQuestionNotAnswered
	}
	t, err := s.host.taskByID(ctx, q.TaskId)
	if err != nil {
		return nil, err
	}
	if t.Status != model.TaskStatusOpen {
		return nil, ErrTaskReopenEnded
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	bossOverride := role.Code == "boss" && q.AskedBy != accountId
	if q.AskedBy != accountId && !bossOverride {
		return nil, ErrReopenForbidden
	}

	reason := strings.TrimSpace(req.GetReason())
	tnow := now()
	tx := s.host.DB.WithContext(ctx).Begin()
	res := tx.Model(&model.Question{}).
		Where(model.QuestionColumnId, q.Id).
		Where(model.QuestionColumnStatus, model.QuestionStatusAnswered).
		Updates(map[string]any{
			model.QuestionColumnAnswer:      "",
			model.QuestionColumnPicked:      "",
			model.QuestionColumnEvidence:    "",
			model.QuestionColumnStatus:      model.QuestionStatusOpen,
			model.QuestionColumnAnsweredBy:  int64(0),
			model.QuestionColumnAnsweredAt:  nil,
			model.QuestionColumnSelfAnswer:  false,
			model.QuestionColumnReopenCount: q.ReopenCount + 1,
			model.QuestionColumnVersion:     q.Version + 1,
			model.QuestionColumnUpdatedBy:   cast.ToString(accountId),
			model.QuestionColumnUpdateTime:  &tnow,
		})
	if res.Error != nil {
		tx.Rollback()
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return nil, ErrQuestionNotAnswered
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionQuestionReopen, model.EntityQuestion, q.Id,
		map[string]any{
			"task_id":           q.TaskId,
			"reason":            reason,
			"previous_answer":   q.Answer,
			"previous_evidence": q.Evidence,
			"reopen_count":      q.ReopenCount + 1,
			"boss_override":     bossOverride,
		}); err != nil {
		tx.Rollback()
		return nil, err
	}
	// 子问题被打回时，父问题若已被“子问题闭环回填”自动闭环，也必须一并重开，
	// 否则任务会显示 ready 但实际链路还缺一个答案。
	if err := s.reopenParents(ctx, tx, q.ParentQuestionId, q.Id, accountId); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	fresh, err := s.host.questionByID(ctx, q.Id)
	if err != nil {
		return nil, err
	}
	roleNames, err := s.host.roleNameMap(ctx)
	if err != nil {
		return nil, err
	}
	taskTitles, err := s.host.taskTitleMap(ctx, []model.Question{*fresh})
	if err != nil {
		return nil, err
	}
	return questionToPb(fresh, taskTitles[fresh.TaskId], roleNames), nil
}

const autoCloseAnswerPrefix = "由子问题闭环回填："

// reopenParents 沿升级链向上，把“因所有子问题闭环而自动闭环”的父问题重新打开。
// 只动自动回填的父问题；人工给出的父问题答案不会被子问题重开推翻。
func (s *QuestionService) reopenParents(ctx context.Context, tx *gorm.DB, parentId, childId, accountId int64) error {
	for parentId > 0 {
		var parent model.Question
		err := tx.WithContext(ctx).Select(model.QuestionFields).
			Where(model.QuestionColumnId, parentId).First(&parent).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		if parent.Status != model.QuestionStatusAnswered || !strings.HasPrefix(parent.Answer, autoCloseAnswerPrefix) {
			return nil
		}
		tnow := now()
		res := tx.WithContext(ctx).Model(&model.Question{}).
			Where(model.QuestionColumnId, parent.Id).
			Where(model.QuestionColumnStatus, model.QuestionStatusAnswered).
			Updates(map[string]any{
				model.QuestionColumnAnswer:     "",
				model.QuestionColumnPicked:     "",
				model.QuestionColumnEvidence:   "",
				model.QuestionColumnStatus:     model.QuestionStatusOpen,
				model.QuestionColumnAnsweredBy: int64(0),
				model.QuestionColumnAnsweredAt: nil,
				model.QuestionColumnSelfAnswer: false,
				model.QuestionColumnVersion:    parent.Version + 1,
				model.QuestionColumnUpdatedBy:  cast.ToString(accountId),
				model.QuestionColumnUpdateTime: &tnow,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionQuestionReopenCascade, model.EntityQuestion, parent.Id,
			map[string]any{"child_question_id": childId, "reopened_by_child": childId}); err != nil {
			return err
		}
		parentId = parent.ParentQuestionId
	}
	return nil
}

// depthOf 返回某个问题在升级链上的层级（根问题 = 0）。
func (s *QuestionService) depthOf(ctx context.Context, id int64) (int, error) {
	depth := 0
	cur := id
	for cur > 0 && depth < 10 {
		q, err := s.host.questionByID(ctx, cur)
		if err != nil {
			return 0, err
		}
		cur = q.ParentQuestionId
		if cur > 0 {
			depth++
		}
	}
	return depth, nil
}

// cascadeUp 沿 parent_question_id 向上回填；任何一级还有其它 open 子问题就停止。
func (s *QuestionService) cascadeUp(ctx context.Context, tx *gorm.DB, parentId, closedChildId, accountId int64) error {
	for parentId > 0 {
		var parent model.Question
		err := tx.WithContext(ctx).Select(model.QuestionFields).
			Where(model.QuestionColumnId, parentId).First(&parent).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		if parent.Status != model.QuestionStatusOpen {
			return nil
		}
		var remaining int64
		if err := tx.WithContext(ctx).Model(&model.Question{}).
			Where(model.QuestionColumnParentQuestionId, parentId).
			Where(model.QuestionColumnStatus, model.QuestionStatusOpen).
			Where(model.QuestionColumnIsDeleted, false).
			Count(&remaining).Error; err != nil {
			return err
		}
		if remaining > 0 {
			return nil
		}
		t := now()
		res := tx.WithContext(ctx).Model(&model.Question{}).
			Where(model.QuestionColumnId, parentId).
			Where(model.QuestionColumnStatus, model.QuestionStatusOpen).
			Updates(map[string]any{
				model.QuestionColumnAnswer:     "由子问题闭环回填：" + parent.Body,
				model.QuestionColumnStatus:     model.QuestionStatusAnswered,
				model.QuestionColumnAnsweredBy: accountId,
				model.QuestionColumnAnsweredAt: &t,
				model.QuestionColumnSelfAnswer: parent.AskedBy != 0 && parent.AskedBy == accountId,
				model.QuestionColumnVersion:    parent.Version + 1,
				model.QuestionColumnUpdatedBy:  cast.ToString(accountId),
				model.QuestionColumnUpdateTime: &t,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionQuestionAutoClose, model.EntityQuestion, parentId,
			map[string]any{"child_question_id": closedChildId, "child_answered": true}); err != nil {
			return err
		}
		parentId = parent.ParentQuestionId
	}
	return nil
}

// ---- 共用读取 ----

func (h *Host) questionByID(ctx context.Context, id int64) (*model.Question, error) {
	if id <= 0 {
		return nil, ErrQuestionNotFound
	}
	var q model.Question
	err := h.DB.WithContext(ctx).Select(model.QuestionFields).
		Where(model.QuestionColumnId, id).
		Where(model.QuestionColumnApplicationId, h.AppId).
		Where(model.QuestionColumnIsDeleted, false).
		First(&q).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}
	return &q, nil
}

func (h *Host) taskTitleMap(ctx context.Context, qs []model.Question) (map[int64]string, error) {
	ids := make([]int64, 0, len(qs))
	seen := map[int64]struct{}{}
	for i := range qs {
		if _, ok := seen[qs[i].TaskId]; ok {
			continue
		}
		seen[qs[i].TaskId] = struct{}{}
		ids = append(ids, qs[i].TaskId)
	}
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var tasks []model.Task
	if err := h.DB.WithContext(ctx).Select(model.TaskColumnId, model.TaskColumnTitle).
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

func questionToPb(q *model.Question, taskTitle string, roleNames map[int64]string) *flow_v1.Question {
	opts := optionsFromJSON(q.Options)
	pbOpts := make([]*flow_v1.QuestionOption, 0, len(opts))
	for _, o := range opts {
		pbOpts = append(pbOpts, &flow_v1.QuestionOption{Id: o.Id, Label: o.Label, Rec: o.Rec})
	}
	return &flow_v1.Question{
		Id:               cast.ToString(q.Id),
		TaskId:           cast.ToString(q.TaskId),
		TaskTitle:        taskTitle,
		FromRoleId:       cast.ToString(q.FromRoleId),
		FromRoleName:     roleNames[q.FromRoleId],
		ToRoleId:         cast.ToString(q.ToRoleId),
		ToRoleName:       roleNames[q.ToRoleId],
		AskedBy:          cast.ToString(q.AskedBy),
		Body:             q.Body,
		Options:          pbOpts,
		Answer:           q.Answer,
		Picked:           q.Picked,
		AnswerKind:       q.AnswerKind,
		Evidence:         q.Evidence,
		Status:           q.Status,
		ParentQuestionId: cast.ToString(q.ParentQuestionId),
		ReopenCount:      int32(q.ReopenCount),
		SelfAnswer:       q.SelfAnswer,
		AnsweredBy:       cast.ToString(q.AnsweredBy),
		AnsweredAt:       tsPtr(q.AnsweredAt),
		CreateTime:       tsOf(q.CreateTime),
	}
}
