package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	flow_v1 "github.com/linjunhao1997/idl/pb/flow"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"flow-service/pkg/model"
)

const maxAncestorDepth = 32

var executionRoleCodes = map[string]bool{"dev": true, "qa": true, "ops": true}
var planningRoleCodes = map[string]bool{"analyst": true, "pm": true, "arch": true}

var defaultRequiredArtifacts = map[string][]string{
	"dev": {"requirement_spec", "feature_design", "tech_plan", "acceptance_criteria"},
	"qa":  {"requirement_spec", "feature_design", "acceptance_criteria", "code_change"},
	"ops": {"acceptance_criteria", "test_report", "release_plan"},
}

var defaultOutputArtifact = map[string]string{
	"dev": "code_change",
	"qa":  "test_report",
	"ops": "deployment_record",
}

type workArtifactSnapshot struct {
	Id      string `json:"id"`
	TaskId  string `json:"task_id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	Hash    string `json:"hash"`
	Content string `json:"content_md"`
}

type workPackageSnapshot struct {
	TaskId      string                 `json:"task_id"`
	TaskTitle   string                 `json:"task_title"`
	TaskBrief   string                 `json:"task_brief"`
	RoleCode    string                 `json:"role_code"`
	GeneratedAt string                 `json:"generated_at"`
	Artifacts   []workArtifactSnapshot `json:"artifacts"`
}

func splitTypes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func joinTypes(types []string) string { return strings.Join(types, ",") }

func requiredTypesFor(t *model.Task, roleCode string) []string {
	if types := splitTypes(t.RequiredArtifactTypes); len(types) > 0 {
		return types
	}
	return defaultRequiredArtifacts[roleCode]
}

func (s *TaskService) StartTask(ctx context.Context, req *common_v1.GetRequest) (*flow_v1.WorkPackage, error) {
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
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if t.OwnerId <= 0 || t.OwnerId != accountId || t.RoleId != role.Id {
		return nil, ErrTaskOwnerMismatch
	}
	if !executionRoleCodes[role.Code] {
		return nil, ErrTaskStateInvalid
	}
	if t.Status != model.TaskStatusOpen {
		return nil, ErrTaskEnded
	}
	openQ, err := s.host.openQuestionCount(ctx, 0, t.Id)
	if err != nil {
		return nil, err
	}
	if openQ > 0 {
		return nil, ErrTaskBlocked
	}
	if t.StartedAt != nil {
		return s.workPackageFromSnapshot(t)
	}

	arts, missing, err := s.workArtifacts(ctx, t, role.Code)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, ErrWorkPackageMissing
	}

	required := requiredTypesFor(t, role.Code)
	snapshot := workPackageSnapshot{
		TaskId: cast.ToString(t.Id), TaskTitle: t.Title, TaskBrief: t.Brief,
		RoleCode: role.Code, GeneratedAt: now().Format(time.RFC3339Nano),
	}
	for i := range arts {
		snapshot.Artifacts = append(snapshot.Artifacts, workArtifactSnapshot{
			Id: cast.ToString(arts[i].Id), TaskId: cast.ToString(arts[i].TaskId),
			Type: arts[i].Type, Title: arts[i].Title, Version: arts[i].Version,
			Status: arts[i].Status, Hash: arts[i].Hash, Content: arts[i].ContentMd,
		})
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(body)
	contextHash := hex.EncodeToString(hash[:])

	tx := s.host.DB.WithContext(ctx).Begin()
	// 实施开工即确认上游已提交的规划物料：下游真正开始消费，才算验收。
	for i := range arts {
		if err := s.acceptPlanningArtifactTask(ctx, tx, arts[i].TaskId, accountId, role); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	res := tx.Model(&model.Task{}).
		Where(model.TaskColumnId, t.Id).
		Where(model.TaskColumnStatus, model.TaskStatusOpen).
		Updates(map[string]any{
			model.TaskColumnStartedAt:             now(),
			model.TaskColumnRequiredArtifactTypes: joinTypes(required),
			model.TaskColumnContextHash:           contextHash,
			model.TaskColumnContextJson:           string(body),
			model.TaskColumnVersion:               t.Version + 1,
			model.TaskColumnUpdatedBy:             cast.ToString(accountId),
			model.TaskColumnUpdateTime:            now(),
		})
	if res.Error != nil {
		tx.Rollback()
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return nil, ErrTaskNotOpen
	}
	detail := map[string]any{"artifacts": len(arts), "context_hash": contextHash, "role": role.Code}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionTaskStart, model.EntityTask, t.Id, detail); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	t.StartedAt = ptrTime(now())
	t.ContextHash = contextHash
	wp := &flow_v1.WorkPackage{
		Task:        s.taskWithMeta(ctx, t, role.Name, accountId, m.Name),
		Artifacts:   artifactsToPb(arts),
		ContextHash: contextHash,
		GeneratedAt: tsOf(now()),
	}
	return wp, nil
}

func (s *TaskService) SubmitTask(ctx context.Context, req *flow_v1.SubmitTaskRequest) (*flow_v1.Artifact, error) {
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
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if t.OwnerId <= 0 || t.OwnerId != accountId || t.RoleId != role.Id {
		return nil, ErrTaskOwnerMismatch
	}
	if !planningRoleCodes[role.Code] {
		return nil, ErrTaskStateInvalid
	}
	if t.Status != model.TaskStatusOpen {
		return nil, ErrTaskEnded
	}
	openQ, err := s.host.openQuestionCount(ctx, 0, t.Id)
	if err != nil {
		return nil, err
	}
	if openQ > 0 {
		return nil, ErrTaskBlocked
	}
	content := strings.TrimSpace(req.GetContentMd())
	if content == "" {
		return nil, ErrArtifactEmpty
	}
	artType := strings.TrimSpace(req.GetArtifactType())
	if artType == "" {
		return nil, ErrArtifactEmpty
	}
	title := strings.TrimSpace(req.GetTitle())
	if title == "" {
		title = t.Title
	}
	hash := hashText(content)

	var maxVersion int
	if err := s.host.DB.WithContext(ctx).Model(&model.Artifact{}).
		Where(model.ArtifactColumnTaskId, t.Id).
		Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
		return nil, err
	}
	art := model.Artifact{
		ApplicationId: s.host.AppId, TaskId: t.Id, Type: artType, Title: title,
		ContentMd: content, Version: maxVersion + 1, Status: model.ArtifactStatusSubmitted,
		Hash: hash, CreatedBy: accountId, CreatedRoleId: role.Id,
		CreateTime: now(), UpdateTime: now(),
	}
	tx := s.host.DB.WithContext(ctx).Begin()
	if err := tx.Model(&model.Artifact{}).
		Where(model.ArtifactColumnTaskId, t.Id).
		Where(model.ArtifactColumnType, artType).
		Where(model.ArtifactColumnStatus+" IN ?", []string{model.ArtifactStatusSubmitted, model.ArtifactStatusAccepted}).
		Updates(map[string]any{
			model.ArtifactColumnStatus:     model.ArtifactStatusSuperseded,
			model.ArtifactColumnUpdateTime: now(),
		}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Create(&art).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Model(&model.Task{}).Where(model.TaskColumnId, t.Id).
		Updates(map[string]any{
			model.TaskColumnSubmittedAt: now(),
			model.TaskColumnVersion:     t.Version + 1,
			model.TaskColumnUpdatedBy:   cast.ToString(accountId),
			model.TaskColumnUpdateTime:  now(),
		}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	detail := map[string]any{"artifact_id": art.Id, "type": art.Type, "version": art.Version, "hash": art.Hash}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionTaskSubmit, model.EntityTask, t.Id, detail); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return artifactToPb(&art), nil
}

func (s *TaskService) AcceptTask(ctx context.Context, req *common_v1.GetRequest) (*emptypb.Empty, error) {
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
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if t.Status != model.TaskStatusOpen || t.SubmittedAt == nil {
		return nil, ErrTaskStateInvalid
	}
	taskRole, err := s.host.roleByID(ctx, t.RoleId)
	if err != nil {
		return nil, err
	}
	if t.RoleId == role.Id {
		return nil, ErrTaskAcceptForbidden
	}
	if role.Code != "boss" && role.Sort <= taskRole.Sort {
		return nil, ErrTaskAcceptForbidden
	}
	openQ, err := s.host.openQuestionCount(ctx, 0, t.Id)
	if err != nil {
		return nil, err
	}
	if openQ > 0 {
		return nil, ErrTaskBlocked
	}
	openChildren, err := s.openChildCount(ctx, t.Id)
	if err != nil {
		return nil, err
	}
	if openChildren > 0 {
		return nil, ErrChildTasksOpen
	}
	tx := s.host.DB.WithContext(ctx).Begin()
	if err := s.acceptTaskTx(ctx, tx, t, accountId); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionTaskAccept, model.EntityTask, t.Id, nil); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *TaskService) FinishTask(ctx context.Context, req *flow_v1.FinishTaskRequest) (*emptypb.Empty, error) {
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
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if t.OwnerId <= 0 || t.OwnerId != accountId || t.RoleId != role.Id {
		return nil, ErrTaskOwnerMismatch
	}
	if !executionRoleCodes[role.Code] {
		return nil, ErrTaskStateInvalid
	}
	if t.Status != model.TaskStatusOpen {
		return nil, ErrTaskEnded
	}
	if t.StartedAt == nil {
		return nil, ErrTaskNotStarted
	}
	openQ, err := s.host.openQuestionCount(ctx, 0, t.Id)
	if err != nil {
		return nil, err
	}
	if openQ > 0 {
		return nil, ErrTaskBlocked
	}
	openChildren, err := s.openChildCount(ctx, t.Id)
	if err != nil {
		return nil, err
	}
	if openChildren > 0 {
		return nil, ErrChildTasksOpen
	}
	content := strings.TrimSpace(req.GetContentMd())
	if content == "" {
		return nil, ErrArtifactEmpty
	}
	artType := strings.TrimSpace(req.GetArtifactType())
	if artType == "" {
		artType = defaultOutputArtifact[role.Code]
	}
	title := strings.TrimSpace(req.GetTitle())
	if title == "" {
		title = t.Title
	}
	art := model.Artifact{
		ApplicationId: s.host.AppId, TaskId: t.Id, Type: artType, Title: title,
		ContentMd: content, Version: 1, Status: model.ArtifactStatusAccepted,
		Hash: hashText(content), CreatedBy: accountId, CreatedRoleId: role.Id,
		AcceptedBy: accountId, CreateTime: now(), UpdateTime: now(),
	}
	acceptAt := now()
	art.AcceptedAt = &acceptAt

	tx := s.host.DB.WithContext(ctx).Begin()
	if err := tx.Create(&art).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
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
	detail := map[string]any{"artifact_id": art.Id, "type": art.Type, "hash": art.Hash}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionTaskFinish, model.EntityTask, t.Id, detail); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *TaskService) GetWorkPackage(ctx context.Context, req *common_v1.GetRequest) (*flow_v1.WorkPackage, error) {
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
	t, err := s.host.taskByID(ctx, cast.ToInt64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if t.OwnerId != accountId && role.Code != "boss" {
		return nil, ErrTaskOwnerMismatch
	}
	arts, missing, err := s.workArtifacts(ctx, t, role.Code)
	if err != nil {
		return nil, err
	}
	return &flow_v1.WorkPackage{
		Task:         s.taskWithMeta(ctx, t, role.Name, accountId, m.Name),
		Artifacts:    artifactsToPb(arts),
		MissingTypes: missing,
		ContextHash:  t.ContextHash,
		GeneratedAt:  tsOf(now()),
	}, nil
}

func (s *TaskService) ListTaskArtifacts(ctx context.Context, req *common_v1.GetRequest) (*flow_v1.ListArtifactsResponse, error) {
	if _, _, err := s.host.ensureMember(ctx); err != nil {
		return nil, err
	}
	var arts []model.Artifact
	if err := s.host.DB.WithContext(ctx).Select(model.ArtifactFields).
		Where(model.ArtifactColumnApplicationId, s.host.AppId).
		Where(model.ArtifactColumnTaskId, cast.ToInt64(req.GetId())).
		Where(model.ArtifactColumnIsDeleted, false).
		Order(model.ArtifactColumnId + " DESC").Find(&arts).Error; err != nil {
		return nil, err
	}
	return &flow_v1.ListArtifactsResponse{Artifacts: artifactsToPb(arts)}, nil
}

func (s *TaskService) workArtifacts(ctx context.Context, t *model.Task, roleCode string) ([]model.Artifact, []string, error) {
	required := requiredTypesFor(t, roleCode)
	byType := make(map[string]model.Artifact, len(required))
	seenTasks := make(map[int64]bool, maxAncestorDepth)
	chain := make([]int64, 0, maxAncestorDepth)
	cur := t.ParentTaskId
	for depth := 0; cur > 0 && depth < maxAncestorDepth; depth++ {
		if seenTasks[cur] {
			break
		}
		seenTasks[cur] = true
		chain = append(chain, cur)
		parent, err := s.host.taskByID(ctx, cur)
		if err != nil {
			break
		}
		cur = parent.ParentTaskId
	}
	for _, taskId := range chain {
		var arts []model.Artifact
		if err := s.host.DB.WithContext(ctx).Select(model.ArtifactFields).
			Where(model.ArtifactColumnApplicationId, s.host.AppId).
			Where(model.ArtifactColumnTaskId, taskId).
			Where(model.ArtifactColumnStatus+" IN ?", []string{model.ArtifactStatusSubmitted, model.ArtifactStatusAccepted}).
			Where(model.ArtifactColumnIsDeleted, false).
			Order(model.ArtifactColumnVersion + " DESC").Find(&arts).Error; err != nil {
			return nil, nil, err
		}
		for i := range arts {
			if _, ok := byType[arts[i].Type]; !ok {
				byType[arts[i].Type] = arts[i]
			}
		}
	}
	missing := make([]string, 0)
	for _, typ := range required {
		if _, ok := byType[typ]; !ok {
			missing = append(missing, typ)
		}
	}
	out := make([]model.Artifact, 0, len(byType))
	for _, art := range byType {
		out = append(out, art)
	}
	// 按类型稳定排序，便于上下文 hash 可复现。
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Type < out[i].Type {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, missing, nil
}

func (s *TaskService) workPackageFromSnapshot(t *model.Task) (*flow_v1.WorkPackage, error) {
	var snap workPackageSnapshot
	if strings.TrimSpace(t.ContextJson) == "" || json.Unmarshal([]byte(t.ContextJson), &snap) != nil {
		return nil, ErrTaskStateInvalid
	}
	arts := make([]model.Artifact, 0, len(snap.Artifacts))
	for _, a := range snap.Artifacts {
		arts = append(arts, model.Artifact{
			Id: cast.ToInt64(a.Id), TaskId: cast.ToInt64(a.TaskId), Type: a.Type,
			Title: a.Title, Version: a.Version, Status: a.Status, Hash: a.Hash, ContentMd: a.Content,
		})
	}
	role, err := s.host.roleByID(context.Background(), t.RoleId)
	if err != nil {
		return nil, err
	}
	return &flow_v1.WorkPackage{
		Task:        taskToPb(t, taskMeta{roleName: role.Name}, nil),
		Artifacts:   artifactsToPb(arts),
		ContextHash: t.ContextHash,
		GeneratedAt: tsOf(now()),
	}, nil
}

func (s *TaskService) acceptPlanningArtifactTask(ctx context.Context, tx *gorm.DB, taskId, accountId int64, role *model.WorkRole) error {
	t, err := s.host.taskByID(ctx, taskId)
	if err != nil {
		return err
	}
	tr, err := s.host.roleByID(ctx, t.RoleId)
	if err != nil {
		return err
	}
	if !planningRoleCodes[tr.Code] || t.Status != model.TaskStatusOpen || t.SubmittedAt == nil {
		return nil
	}
	if t.RoleId == role.Id {
		return nil
	}
	openChildren, err := s.openChildCount(ctx, t.Id)
	if err != nil {
		return err
	}
	if openChildren > 0 {
		// 下游已经开工，但父任务还有没完成的直接子任务；父任务继续停在已提交，等下级全部完成后再确认。
		return nil
	}
	return s.acceptTaskTx(ctx, tx, t, accountId)
}

func (s *TaskService) openChildCount(ctx context.Context, taskId int64) (int64, error) {
	var open int64
	err := s.host.DB.WithContext(ctx).Model(&model.Task{}).
		Where(model.TaskColumnParentTaskId, taskId).
		Where(model.TaskColumnStatus, model.TaskStatusOpen).
		Where(model.TaskColumnIsDeleted, false).
		Count(&open).Error
	return open, err
}

func (s *TaskService) acceptTaskTx(ctx context.Context, tx *gorm.DB, t *model.Task, accountId int64) error {
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
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}
	return tx.Model(&model.Artifact{}).
		Where(model.ArtifactColumnTaskId, t.Id).
		Where(model.ArtifactColumnStatus, model.ArtifactStatusSubmitted).
		Updates(map[string]any{
			model.ArtifactColumnStatus:     model.ArtifactStatusAccepted,
			model.ArtifactColumnAcceptedBy: accountId,
			model.ArtifactColumnAcceptedAt: now(),
			model.ArtifactColumnUpdateTime: now(),
		}).Error
}

func (s *TaskService) taskWithMeta(ctx context.Context, t *model.Task, roleName string, accountId int64, ownerName string) *flow_v1.Task {
	meta := taskMeta{roleName: roleName, ownerName: ownerName}
	var artCount int64
	_ = s.host.DB.WithContext(ctx).Model(&model.Artifact{}).
		Where(model.ArtifactColumnTaskId, t.Id).
		Where(model.ArtifactColumnIsDeleted, false).
		Count(&artCount).Error
	meta.artifactCount = int32(artCount)
	return taskToPb(t, meta, nil)
}

func artifactToPb(a *model.Artifact) *flow_v1.Artifact {
	out := &flow_v1.Artifact{
		Id: cast.ToString(a.Id), TaskId: cast.ToString(a.TaskId), Type: a.Type,
		Title: a.Title, ContentMd: a.ContentMd, Version: int32(a.Version),
		Status: a.Status, Hash: a.Hash, CreatedBy: cast.ToString(a.CreatedBy),
		CreatedRoleId: cast.ToString(a.CreatedRoleId), AcceptedBy: cast.ToString(a.AcceptedBy),
		CreateTime: tsOf(a.CreateTime),
	}
	if a.AcceptedAt != nil {
		out.AcceptedAt = tsOf(*a.AcceptedAt)
	}
	return out
}

func artifactsToPb(in []model.Artifact) []*flow_v1.Artifact {
	out := make([]*flow_v1.Artifact, 0, len(in))
	for i := range in {
		out = append(out, artifactToPb(&in[i]))
	}
	return out
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
