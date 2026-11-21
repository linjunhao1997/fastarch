package service

import (
	"context"
	"strconv"
	"strings"

	flow_v1 "github.com/linjunhao1997/idl/pb/flow"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"flow-service/pkg/model"
)

// MemberService 实现"我是谁 / 我能担任哪些角色 / 我现在用哪个角色"。
type MemberService struct {
	host *Host
}

func NewMemberService(host *Host) *MemberService { return &MemberService{host: host} }

func (s *MemberService) GetMe(ctx context.Context, _ *emptypb.Empty) (*flow_v1.Member, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	return s.host.memberToPb(ctx, m, roles)
}

func (s *MemberService) ListMyRoles(ctx context.Context, _ *emptypb.Empty) (*flow_v1.ListMyRolesResponse, error) {
	_, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*flow_v1.WorkRole, 0, len(roles))
	for i := range roles {
		out = append(out, workRoleToPb(&roles[i]))
	}
	return &flow_v1.ListMyRolesResponse{Roles: out}, nil
}

func (s *MemberService) SwitchRole(ctx context.Context, req *flow_v1.SwitchRoleRequest) (*flow_v1.Member, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(req.GetRoleCode())
	if key == "" {
		return nil, ErrRoleNotFound
	}
	var target *model.WorkRole
	for i := range roles {
		if roles[i].Code == key || roles[i].Name == key {
			target = &roles[i]
			break
		}
	}
	if target == nil {
		return nil, ErrRoleNotHeld
	}
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if m.CurrentRoleId != target.Id {
		t := now()
		res := s.host.DB.WithContext(ctx).Model(&model.Member{}).
			Where(model.MemberColumnId, m.Id).
			Where(model.MemberColumnVersion, m.Version).
			Updates(map[string]any{
				model.MemberColumnCurrentRoleId: target.Id,
				model.MemberColumnVersion:       m.Version + 1,
				model.MemberColumnUpdatedBy:     cast.ToString(accountId),
				model.MemberColumnUpdateTime:    &t,
			})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 0 {
			// 版本已经被别人改过：重读一次，若当前角色已是目标角色则视为幂等成功，
			// 否则才是真正的并发冲突——不能再报"未持有角色"，那会误导人。
			var fresh model.Member
			if rerr := s.host.DB.WithContext(ctx).Select(model.MemberFields).
				Where(model.MemberColumnId, m.Id).First(&fresh).Error; rerr != nil || fresh.CurrentRoleId != target.Id {
				return nil, ErrConcurrentUpdate
			}
			m = &fresh
		} else {
			m.CurrentRoleId = target.Id
			m.Version++
			if err := s.host.appendEvent(ctx, s.host.DB, accountId, model.EventActionRoleSwitch,
				model.EntityMember, m.Id, map[string]any{"role_code": target.Code, "role_name": target.Name}); err != nil {
				return nil, err
			}
		}
	}
	return s.host.memberToPb(ctx, m, roles)
}

// requireBoss 取当前身份并校验"当前角色是老板（boss）"。
// 判权分两层：网关拦截器只校验登录态与权限码（flow.member.get），
// "谁能改别人的角色"这种业务身份由这里强制——不依赖 IAM 新增权限码，
// 避免为一处能力去动权限种子数据。
func (s *MemberService) requireBoss(ctx context.Context) (*model.Member, *model.WorkRole, error) {
	m, roles, err := s.host.ensureMember(ctx)
	if err != nil {
		return nil, nil, err
	}
	role := currentRole(m, roles)
	if role == nil {
		return nil, nil, ErrRoleNotFound
	}
	if role.Code != model.RoleCodeBoss {
		return nil, nil, ErrNotBoss
	}
	return m, role, nil
}

// ListMembers 列出全部成员及其持有角色，并回传全部启用角色供前端勾选。
func (s *MemberService) ListMembers(ctx context.Context, req *flow_v1.ListMembersRequest) (*flow_v1.ListMembersResponse, error) {
	_, role, err := s.requireBoss(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := limitAndOffset(req.GetCurrentPage(), req.GetPageSize())

	q := s.host.DB.WithContext(ctx).Model(&model.Member{}).
		Where(model.MemberColumnIsDeleted, false)
	if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
		q = q.Where(model.MemberColumnName+" LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var ms []model.Member
	if err := q.Select(model.MemberFields).Order(model.MemberColumnId + " ASC").
		Limit(limit).Offset(offset).Find(&ms).Error; err != nil {
		return nil, err
	}

	all, err := s.host.enabledRoles(ctx)
	if err != nil {
		return nil, err
	}
	resp := &flow_v1.ListMembersResponse{
		Page:            pageOf(req.GetCurrentPage(), req.GetPageSize(), total),
		CurrentRoleCode: role.Code,
	}
	for i := range all {
		resp.AllRoles = append(resp.AllRoles, workRoleToPb(&all[i]))
	}
	for i := range ms {
		b := &flow_v1.MemberBrief{
			Id:            cast.ToString(ms[i].Id),
			AccountId:     cast.ToString(ms[i].AccountId),
			Name:          ms[i].Name,
			Title:         ms[i].Title,
			CurrentRoleId: cast.ToString(ms[i].CurrentRoleId),
		}
		mrs, rerr := s.host.rolesOfMember(ctx, ms[i].Id)
		if rerr != nil {
			return nil, rerr
		}
		for j := range mrs {
			b.Roles = append(b.Roles, workRoleToPb(&mrs[j]))
			if mrs[j].Id == ms[i].CurrentRoleId {
				b.CurrentRole = workRoleToPb(&mrs[j])
			}
		}
		resp.Members = append(resp.Members, b)
	}
	return resp, nil
}

// AssignMemberRoles 整体替换某个成员能担任的工作角色（boss 才能改）。
func (s *MemberService) AssignMemberRoles(ctx context.Context, req *flow_v1.AssignMemberRolesRequest) (*flow_v1.MemberBrief, error) {
	me, _, err := s.requireBoss(ctx)
	if err != nil {
		return nil, err
	}
	targetId := req.GetId()
	if targetId <= 0 {
		return nil, ErrMemberNotFound
	}
	var target model.Member
	if err := s.host.DB.WithContext(ctx).Select(model.MemberFields).
		Where(model.MemberColumnId, targetId).
		Where(model.MemberColumnIsDeleted, false).
		First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}

	codes := make([]string, 0, len(req.GetRoleCodes()))
	seen := make(map[string]struct{}, len(req.GetRoleCodes()))
	for _, c := range req.GetRoleCodes() {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		codes = append(codes, c)
	}
	if len(codes) == 0 {
		return nil, ErrRoleCodesEmpty
	}

	all, err := s.host.enabledRoles(ctx)
	if err != nil {
		return nil, err
	}
	picked := make([]model.WorkRole, 0, len(codes))
	for _, c := range codes {
		var hit *model.WorkRole
		for i := range all {
			if all[i].Code == c || all[i].Name == c {
				hit = &all[i]
				break
			}
		}
		if hit == nil {
			return nil, ErrRoleNotFound
		}
		picked = append(picked, *hit)
	}

	before, err := s.host.rolesOfMember(ctx, target.Id)
	if err != nil {
		return nil, err
	}
	// 自锁保护：改自己时不能把 boss 摘掉，否则以后没人能再分配角色。
	if target.AccountId == me.AccountId {
		had, keep := false, false
		for i := range before {
			if before[i].Code == model.RoleCodeBoss {
				had = true
			}
		}
		for i := range picked {
			if picked[i].Code == model.RoleCodeBoss {
				keep = true
			}
		}
		if had && !keep {
			return nil, ErrSelfBossLockout
		}
	}

	// 当前角色必须仍在持有集合内，否则重置为第一个，避免"当前角色 = 未持有角色"。
	currentId := target.CurrentRoleId
	keepCurrent := false
	for i := range picked {
		if picked[i].Id == currentId {
			keepCurrent = true
			break
		}
	}
	if !keepCurrent {
		currentId = picked[0].Id
	}

	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	t := now()
	tx := s.host.DB.WithContext(ctx).Begin()
	if err := tx.Where("member_id = ?", target.Id).Delete(&model.MemberRole{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	for i := range picked {
		if err := tx.Create(&model.MemberRole{MemberId: target.Id, RoleId: picked[i].Id, CreateTime: &t}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Model(&model.Member{}).Where(model.MemberColumnId, target.Id).
		Updates(map[string]any{
			model.MemberColumnCurrentRoleId: currentId,
			model.MemberColumnVersion:       target.Version + 1,
			model.MemberColumnUpdatedBy:     cast.ToString(accountId),
			model.MemberColumnUpdateTime:    &t,
		}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.host.appendEvent(ctx, tx, accountId, model.EventActionMemberRolesAssign, model.EntityMember, target.Id,
		map[string]any{"role_codes": codes, "before": roleCodes(before), "current_role_id": currentId}); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	after, err := s.host.rolesOfMember(ctx, target.Id)
	if err != nil {
		return nil, err
	}
	b := &flow_v1.MemberBrief{
		Id:            cast.ToString(target.Id),
		AccountId:     cast.ToString(target.AccountId),
		Name:          target.Name,
		Title:         target.Title,
		CurrentRoleId: cast.ToString(currentId),
	}
	for i := range after {
		b.Roles = append(b.Roles, workRoleToPb(&after[i]))
		if after[i].Id == currentId {
			b.CurrentRole = workRoleToPb(&after[i])
		}
	}
	return b, nil
}

func roleCodes(roles []model.WorkRole) []string {
	out := make([]string, 0, len(roles))
	for i := range roles {
		out = append(out, roles[i].Code)
	}
	return out
}

// ensureMember 按 JWT 账号取档案；首次调用自动建档（单人验证场景授予全部角色）。
func (h *Host) ensureMember(ctx context.Context) (*model.Member, []model.WorkRole, error) {
	accountId, err := accountIdFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	var m model.Member
	err = h.DB.WithContext(ctx).Select(model.MemberFields).
		Where(model.MemberColumnAccountId, accountId).
		Where(model.MemberColumnIsDeleted, false).
		First(&m).Error
	if err == nil {
		roles, rerr := h.rolesOfMember(ctx, m.Id)
		return &m, roles, rerr
	}
	if err != gorm.ErrRecordNotFound {
		return nil, nil, err
	}

	// 首次建档授予哪些角色：
	// 1) BootstrapAllRoles=true（单机验证模式）→ 全部角色；
	// 2) 应用里还没有任何成员（第一个进来自动建档的人）→ 全部角色，
	//    否则谁都没有 boss，"定角色"这件事就没人能启动（自举）；
	// 3) 其余情况只给默认角色，剩下的由老板在「成员与角色」里分配。
	grantAll := h.BootstrapAllRoles
	if !grantAll {
		var memberCount int64
		if cerr := h.DB.WithContext(ctx).Model(&model.Member{}).
			Where(model.MemberColumnIsDeleted, false).Count(&memberCount).Error; cerr != nil {
			return nil, nil, cerr
		}
		grantAll = memberCount == 0
	}
	var roles []model.WorkRole
	if grantAll {
		roles, err = h.enabledRoles(ctx)
	} else {
		var r *model.WorkRole
		r, err = h.roleByCode(ctx, h.DefaultRoleCode)
		if err == nil {
			roles = []model.WorkRole{*r}
		}
	}
	if err != nil {
		return nil, nil, err
	}

	currentRoleId := int64(0)
	for i := range roles {
		if roles[i].Code == h.DefaultRoleCode {
			currentRoleId = roles[i].Id
			break
		}
	}
	if currentRoleId == 0 && len(roles) > 0 {
		currentRoleId = roles[0].Id
	}

	name := strings.TrimSpace(usernameFromContext(ctx))
	if name == "" {
		name = "账号" + strconv.FormatInt(accountId, 10)
	}
	t := now()
	m = model.Member{
		AccountId:     accountId,
		Name:          name,
		Title:         "",
		CurrentRoleId: currentRoleId,
		Version:       0,
		IsDeleted:     false,
		CreatedBy:     cast.ToString(accountId),
		UpdatedBy:     cast.ToString(accountId),
		CreateTime:    &t,
		UpdateTime:    &t,
	}

	tx := h.DB.WithContext(ctx).Begin()
	if err := tx.Create(&m).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	for i := range roles {
		if err := tx.Create(&model.MemberRole{MemberId: m.Id, RoleId: roles[i].Id, CreateTime: &t}).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	}
	if err := h.appendEvent(ctx, tx, accountId, model.EventActionMemberBootstrap, model.EntityMember, m.Id,
		map[string]any{"roles": len(roles), "current_role_id": currentRoleId}); err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}
	return &m, roles, nil
}

// memberToPb 组装身份 + 当前角色 + 角色清单 + 当前角色的任务/问题概览。
func (h *Host) memberToPb(ctx context.Context, m *model.Member, roles []model.WorkRole) (*flow_v1.Member, error) {
	pb := &flow_v1.Member{
		Id:            cast.ToString(m.Id),
		AccountId:     cast.ToString(m.AccountId),
		Name:          m.Name,
		Title:         m.Title,
		CurrentRoleId: cast.ToString(m.CurrentRoleId),
	}
	for i := range roles {
		pb.Roles = append(pb.Roles, workRoleToPb(&roles[i]))
		if roles[i].Id == m.CurrentRoleId {
			pb.CurrentRole = workRoleToPb(&roles[i])
		}
	}
	if pb.CurrentRole == nil && len(roles) > 0 {
		pb.CurrentRole = workRoleToPb(&roles[0])
	}

	if m.CurrentRoleId > 0 {
		ready, blocked, err := h.taskStats(ctx, m.CurrentRoleId)
		if err != nil {
			return nil, err
		}
		pb.ReadyTasks = int32(ready)
		pb.BlockedTasks = int32(blocked)
		// 只在有当前角色时统计"待我闭环"；roleId<=0 在 openQuestionCount 里是"全应用"语义，
		// 直接传 0 会把别人的问题算到这个人头上。
		openQ, err := h.openQuestionCount(ctx, m.CurrentRoleId, 0)
		if err != nil {
			return nil, err
		}
		pb.OpenQuestions = int32(openQ)
	}
	return pb, nil
}

// taskStatsForOwner 返回“当前人 + 当前角色”下 open 任务的可开工数与被阻塞数。
func (h *Host) taskStatsForOwner(ctx context.Context, roleId, ownerId int64) (ready, blocked int64, err error) {
	base := h.DB.WithContext(ctx).Model(&model.Task{}).
		Where(model.TaskColumnRoleId, roleId).
		Where(model.TaskColumnOwnerId, ownerId).
		Where(model.TaskColumnStatus, model.TaskStatusOpen).
		Where(model.TaskColumnIsDeleted, false)

	var total int64
	if err = base.Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var blockedCount int64
	if err = base.Where("EXISTS (SELECT 1 FROM " + model.Question{}.TableName() +
		" q WHERE q.task_id = " + model.Task{}.TableName() + ".id AND q.status = 'open' AND q.is_deleted = 0)").
		Count(&blockedCount).Error; err != nil {
		return 0, 0, err
	}
	return total - blockedCount, blockedCount, nil
}

// taskStats 返回当前角色下 open 任务的可开工数与被阻塞数（派生，不落库）。
func (h *Host) taskStats(ctx context.Context, roleId int64) (ready, blocked int64, err error) {
	base := h.DB.WithContext(ctx).Model(&model.Task{}).
		Where(model.TaskColumnRoleId, roleId).
		Where(model.TaskColumnStatus, model.TaskStatusOpen).
		Where(model.TaskColumnIsDeleted, false)

	var total int64
	if err = base.Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var blockedCount int64
	if err = base.Where("EXISTS (SELECT 1 FROM " + model.Question{}.TableName() +
		" q WHERE q.task_id = " + model.Task{}.TableName() + ".id AND q.status = 'open' AND q.is_deleted = 0)").
		Count(&blockedCount).Error; err != nil {
		return 0, 0, err
	}
	return total - blockedCount, blockedCount, nil
}

// openQuestionCount 统计发往某角色的 open 问题；roleId<=0 时统计全应用。
func (h *Host) openQuestionCount(ctx context.Context, roleId int64, taskId int64) (int64, error) {
	q := h.DB.WithContext(ctx).Model(&model.Question{}).
		Where(model.QuestionColumnApplicationId, h.AppId).
		Where(model.QuestionColumnStatus, model.QuestionStatusOpen).
		Where(model.QuestionColumnIsDeleted, false)
	if roleId > 0 {
		q = q.Where(model.QuestionColumnToRoleId, roleId)
	}
	if taskId > 0 {
		q = q.Where(model.QuestionColumnTaskId, taskId)
	}
	var out int64
	err := q.Count(&out).Error
	return out, err
}

func workRoleToPb(r *model.WorkRole) *flow_v1.WorkRole {
	return &flow_v1.WorkRole{
		Id:    cast.ToString(r.Id),
		Code:  r.Code,
		Name:  r.Name,
		Stage: r.Stage,
		Sort:  int32(r.Sort),
	}
}
