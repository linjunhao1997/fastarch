package service

import (
	"context"
	"log/slog"
	"time"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"

	"github.com/go-dev-frame/sponge/pkg/krand"
	"github.com/redis/go-redis/v9"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type MenuService struct {
	db  *gorm.DB
	rdb *redis.Client
}

var _ iam_v1.MenuServiceServer = (*MenuService)(nil)

func NewMenuService(db *gorm.DB, rdb *redis.Client) *MenuService {
	return &MenuService{db: db, rdb: rdb}
}

// menuToPb 模型转 pb（t_menu 全列映射）。
func menuToPb(m model.Menu) *iam_v1.Menu {
	return &iam_v1.Menu{
		Id:             cast.ToString(m.Id),
		ParentId:       m.ParentId,
		Name:           m.Name,
		Path:           m.Path,
		Icon:           m.Icon,
		Sort:           m.Sort,
		PermissionCode: m.PermissionCode,
		Status:         m.Status,
		Version:        m.Version,
		CreatedBy:      m.CreatedBy,
		UpdatedBy:      m.UpdatedBy,
		CreateTime:     timestamppb.New(m.CreateTime),
		UpdateTime:     timestamppb.New(m.UpdateTime),
	}
}

// menuTreeNodes 按 parent_id 组装菜单树（纯内存回溯，禁止循环内 DB 调用）。
func menuTreeNodes(menus []model.Menu) []*iam_v1.MenuTreeNode {
	nodesById := make(map[int64]*iam_v1.MenuTreeNode, len(menus))
	for _, menu := range menus {
		nodesById[menu.Id] = &iam_v1.MenuTreeNode{
			Menu: menuToPb(menu),
		}
	}
	roots := make([]*iam_v1.MenuTreeNode, 0)
	for _, menu := range menus {
		node := nodesById[menu.Id]
		parent, hasParent := nodesById[menu.ParentId]
		if menu.ParentId != 0 && hasParent {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}

// loadVisibleMenus 按账号角色取可见菜单（011 模型：t_menu + t_r_role_menu）。
// 管理员取全量启用菜单；普通用户取角色绑定菜单并回补祖先路径，保证树结构完整。
func loadVisibleMenus(ctx context.Context, db *gorm.DB, accountId int64, isAdmin bool) []model.Menu {
	var menus []model.Menu
	query := db.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnStatus, model.StatusEnabled)
	if !isAdmin {
		var roleIds []int64
		err := db.WithContext(ctx).Model(model.RUserRole{}).
			Where(model.RUserRoleColumnUserId, accountId).
			Pluck(model.RUserRoleColumnRoleId, &roleIds).Error
		if err != nil {
			slog.Warn("load user role ids failed", "account_id", accountId, "error", err)
			return nil
		}
		if len(roleIds) == 0 {
			return nil
		}
		var menuIds []int64
		err = db.WithContext(ctx).Model(model.RRoleMenu{}).
			Where(model.RRoleMenuColumnRoleId+" IN ?", roleIds).
			Distinct().
			Pluck(model.RRoleMenuColumnMenuId, &menuIds).Error
		if err != nil {
			slog.Warn("load role menu ids failed", "role_ids", roleIds, "error", err)
			return nil
		}
		if len(menuIds) == 0 {
			return nil
		}
		query = query.Where(model.MenuColumnId+" IN ?", menuIds)
	}
	err := query.
		Order(model.MenuColumnSort + " ASC").
		Find(&menus).Error
	if err != nil {
		slog.Warn("load menus failed", "error", err)
		return nil
	}
	// 回补祖先路径：把选中菜单的各级父节点并入集合，保证树从根可达
	return fillMenuAncestors(ctx, db, menus)
}

// fillMenuAncestors 把菜单集合缺失的祖先菜单并入返回集合。
// 一次性载入全量启用菜单后纯内存回溯（菜单树规模有限），避免循环内点查。
func fillMenuAncestors(ctx context.Context, db *gorm.DB, menus []model.Menu) []model.Menu {
	included := make(map[int64]struct{}, len(menus))
	for _, menu := range menus {
		included[menu.Id] = struct{}{}
	}
	var allMenus []model.Menu
	err := db.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnStatus, model.StatusEnabled).
		Find(&allMenus).Error
	if err != nil {
		slog.Warn("load all menus for ancestor fill failed", "error", err)
		return menus
	}
	menuById := make(map[int64]model.Menu, len(allMenus))
	for _, menu := range allMenus {
		menuById[menu.Id] = menu
	}
	result := menus
	for _, menu := range menus {
		parentId := menu.ParentId
		for parentId != 0 {
			if _, ok := included[parentId]; ok {
				break
			}
			parent, exists := menuById[parentId]
			if !exists {
				break
			}
			included[parentId] = struct{}{}
			result = append(result, parent)
			parentId = parent.ParentId
		}
	}
	return result
}

// ListMenuTree 管理员视角全量菜单树（含停用菜单，前端置灰）。
func (s *MenuService) ListMenuTree(ctx context.Context, empty *emptypb.Empty) (*iam_v1.ListMenuTreeResponse, error) {
	var menus []model.Menu
	err := s.db.WithContext(ctx).
		Select(model.MenuFields).
		Order(model.MenuColumnSort + " ASC").
		Find(&menus).Error
	if err != nil {
		slog.Warn("list menu tree failed", "error", err)
		return nil, err
	}
	return &iam_v1.ListMenuTreeResponse{
		Roots: menuTreeNodes(menus),
	}, nil
}

// ListMyMenuTree 当前登录用户可见菜单树：管理员全量启用菜单，
// 普通用户按角色绑定过滤并回补祖先路径。
func (s *MenuService) ListMyMenuTree(ctx context.Context, empty *emptypb.Empty) (*iam_v1.ListMenuTreeResponse, error) {
	isAdmin := auth.IsAdminFromContext(ctx)
	accountId := auth.GetAccountId(ctx)
	menus := loadVisibleMenus(ctx, s.db, accountId, isAdmin)
	return &iam_v1.ListMenuTreeResponse{
		Roots: menuTreeNodes(menus),
	}, nil
}

// CreateMenu 创建菜单，返回创建出的菜单（含 id）。
func (s *MenuService) CreateMenu(ctx context.Context, request *iam_v1.CreateMenuRequest) (*iam_v1.Menu, error) {
	if len(request.PermissionCode) > 0 && !model.IsValidPermissionCode(request.PermissionCode) {
		return nil, ErrInvalidPermissionCode
	}
	pkId := krand.NewID()
	now := time.Now()
	operator := cast.ToString(auth.GetAccountId(ctx))
	status := request.Status
	if status == "" {
		status = model.StatusEnabled
	}
	menu := model.Menu{
		Id:             pkId,
		ParentId:       request.ParentId,
		Name:           request.Name,
		Path:           request.Path,
		Icon:           request.Icon,
		Sort:           request.Sort,
		PermissionCode: request.PermissionCode,
		Status:         status,
		Version:        1,
		CreatedBy:      operator,
		UpdatedBy:      operator,
		CreateTime:     now,
		UpdateTime:     now,
	}
	if err := s.db.WithContext(ctx).Select(model.MenuFields).Create(&menu).Error; err != nil {
		slog.Warn("create menu failed", "menu_name", request.Name, "error", err)
		return nil, err
	}
	return menuToPb(menu), nil
}

// GetMenu 获取菜单信息。
func (s *MenuService) GetMenu(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Menu, error) {
	var menu model.Menu
	err := s.db.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnId, request.Id).
		First(&menu).Error
	if err != nil {
		return nil, err
	}
	return menuToPb(menu), nil
}

// UpdateMenu 修改菜单：字段缺省 = 不修改；parent_id 默认不改，
// 仅在 parent_cleared = true 时按传入值整体移动（含挂到顶级）。
func (s *MenuService) UpdateMenu(ctx context.Context, request *iam_v1.UpdateMenuRequest) (*emptypb.Empty, error) {
	var menu model.Menu
	err := s.db.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnId, request.Id).
		First(&menu).Error
	if err != nil {
		return nil, err
	}
	if len(request.PermissionCode) > 0 && !model.IsValidPermissionCode(request.PermissionCode) {
		return nil, ErrInvalidPermissionCode
	}

	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.MenuColumnName] = request.Name
	}
	if len(request.Path) > 0 {
		updates[model.MenuColumnPath] = request.Path
	}
	if len(request.Icon) > 0 {
		updates[model.MenuColumnIcon] = request.Icon
	}
	if request.Sort != 0 {
		updates[model.MenuColumnSort] = request.Sort
	}
	if len(request.PermissionCode) > 0 {
		updates[model.MenuColumnPermissionCode] = request.PermissionCode
	}
	if len(request.Status) > 0 {
		updates[model.MenuColumnStatus] = request.Status
	}
	if request.ParentCleared {
		updates[model.MenuColumnParentId] = request.ParentId
	}
	updates[model.MenuColumnUpdateTime] = time.Now()
	// 审计字段服务端注入：仅更新修改人与修改时间（宪法 XII）
	updates[model.MenuColumnUpdatedBy] = cast.ToString(auth.GetAccountId(ctx))
	updates[model.MenuColumnVersion] = gorm.Expr(model.MenuColumnVersion + " + 1")

	result := s.db.WithContext(ctx).Model(model.Menu{}).
		Where(model.MenuColumnId, request.Id).
		Where(model.MenuColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		slog.Warn("update menu failed", "menu_id", request.Id, "version", request.Version, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("update menu rejected: version conflict", "menu_id", request.Id, "version", request.Version)
		return nil, ErrUpdateMenuBusy
	}
	return &emptypb.Empty{}, nil
}

// DeleteMenu 删除菜单：子菜单与角色绑定引用检查，物理删除（基础架构型，宪法 XIV）。
func (s *MenuService) DeleteMenu(ctx context.Context, request *iam_v1.DeleteMenuRequest) (*emptypb.Empty, error) {
	var childCount int64
	err := s.db.WithContext(ctx).Model(model.Menu{}).
		Where(model.MenuColumnParentId, request.Id).
		Count(&childCount).Error
	if err != nil {
		slog.Warn("check menu children failed", "menu_id", request.Id, "error", err)
		return nil, err
	}
	if childCount > 0 {
		slog.Warn("delete menu rejected: has child menus", "menu_id", request.Id, "child_count", childCount)
		return nil, ErrMenuHasChildren
	}
	var bindingCount int64
	err = s.db.WithContext(ctx).Model(model.RRoleMenu{}).
		Where(model.RRoleMenuColumnMenuId, request.Id).
		Count(&bindingCount).Error
	if err != nil {
		slog.Warn("check menu role bindings failed", "menu_id", request.Id, "error", err)
		return nil, err
	}
	if bindingCount > 0 {
		slog.Warn("delete menu rejected: bound by roles", "menu_id", request.Id, "binding_count", bindingCount)
		return nil, ErrMenuReferenced
	}

	result := s.db.WithContext(ctx).
		Where(model.MenuColumnId, request.Id).
		Delete(model.Menu{})
	if result.Error != nil {
		slog.Warn("delete menu failed", "menu_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete menu rejected: menu not found", "menu_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

// UpdateRoleMenus 角色绑定菜单全量覆盖：以 menu_ids 为目标集合做整体替换
// （事务内先清空再写入；与 UpdateAccountRoles 的增量语义不同）。
func (s *MenuService) UpdateRoleMenus(ctx context.Context, request *iam_v1.UpdateRoleMenusRequest) (*emptypb.Empty, error) {
	menuIds := make([]int64, 0, len(request.MenuIds))
	seen := make(map[int64]struct{}, len(request.MenuIds))
	for _, raw := range request.MenuIds {
		id := cast.ToInt64(raw)
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		menuIds = append(menuIds, id)
	}

	// 菜单必须全部存在（避免静默丢弃非法 id）
	if len(menuIds) > 0 {
		var count int64
		err := s.db.WithContext(ctx).Model(model.Menu{}).
			Where(model.MenuColumnId+" IN ?", menuIds).
			Count(&count).Error
		if err != nil {
			slog.Warn("check menus for role binding failed", "role_id", request.RoleId, "error", err)
			return nil, err
		}
		if count != int64(len(menuIds)) {
			slog.Warn("bind menus rejected: some menus not found", "role_id", request.RoleId)
			return nil, ErrMenuNotFound
		}
	}

	var role model.Role
	err := s.db.WithContext(ctx).
		Select(model.RoleFields).
		Where(model.RoleColumnId, request.RoleId).
		First(&role).Error
	if err != nil {
		slog.Warn("get role for menu binding failed", "role_id", request.RoleId, "error", err)
		return nil, err
	}
	if err := loadRoleMenus(ctx, s.db, &role); err != nil {
		return nil, err
	}

	// 与当前绑定求差集，只增删变化部分，避免清空重写引发无谓行变更
	boundMenuIds := make(map[int64]struct{}, len(role.MenuList))
	for _, menu := range role.MenuList {
		boundMenuIds[menu.Id] = struct{}{}
	}
	targetSet := make(map[int64]struct{}, len(menuIds))
	for _, id := range menuIds {
		targetSet[id] = struct{}{}
	}
	addedMenuIds := make([]int64, 0, len(menuIds))
	for _, id := range menuIds {
		if _, exists := boundMenuIds[id]; !exists {
			addedMenuIds = append(addedMenuIds, id)
		}
	}
	removedMenuIds := make([]int64, 0, len(role.MenuList))
	for _, menu := range role.MenuList {
		if _, exists := targetSet[menu.Id]; !exists {
			removedMenuIds = append(removedMenuIds, menu.Id)
		}
	}

	// 菜单绑定只写关联表 t_r_role_menu，绝不触碰 menu 行；
	// 不用 Association.Append——传入 stub model.Menu{Id} 会连带 upsert menu 行，
	// 零值 CreateTime/UpdateTime 触发 MySQL "Incorrect datetime value: '0000-00-00'"。
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(addedMenuIds) > 0 {
			rows := make([]model.RRoleMenu, 0, len(addedMenuIds))
			for _, id := range addedMenuIds {
				rows = append(rows, model.RRoleMenu{RoleId: role.Id, MenuId: id})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		if len(removedMenuIds) > 0 {
			if err := tx.Where(model.RRoleMenuColumnRoleId+" = ? AND "+model.RRoleMenuColumnMenuId+" IN ?",
				role.Id, removedMenuIds).Delete(&model.RRoleMenu{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("commit role menu binding failed",
			"role_id", request.RoleId,
			"menu_ids", request.MenuIds,
			"error", err)
		return nil, err
	}
	// 菜单绑定参与授权推导（#6 裁决）：绑定变更后立即失效持有该角色用户的权限快照
	s.invalidateRoleUsersSnapshot(ctx, role.Id)
	return &emptypb.Empty{}, nil
}

// invalidateRoleUsersSnapshot 删除持有指定角色的全部用户的权限快照键。
// 快照为派生数据：删除失败仅记日志，由短 TTL 兜底。
func (s *MenuService) invalidateRoleUsersSnapshot(ctx context.Context, roleId int64) {
	if s.rdb == nil {
		return
	}
	var userIds []int64
	err := s.db.Model(model.RUserRole{}).
		Where(model.RUserRoleColumnRoleId, roleId).
		Pluck(model.RUserRoleColumnUserId, &userIds).Error
	if err != nil {
		slog.Warn("pluck role user ids failed", "role_id", roleId, "error", err)
		return
	}
	for _, userId := range userIds {
		if err := s.rdb.Del(ctx, constant.GetAccountPermissionKey(userId)).Err(); err != nil {
			slog.Warn("invalidate user permission snapshot failed", "user_id", userId, "error", err)
		}
	}
}

// ListRoleMenus 查询角色绑定的菜单 id 集合（编辑角色页回显用）。
func (s *MenuService) ListRoleMenus(ctx context.Context, request *iam_v1.ListRoleMenusRequest) (*iam_v1.ListRoleMenusResponse, error) {
	var bindings []model.RRoleMenu
	err := s.db.WithContext(ctx).Model(model.RRoleMenu{}).
		Select(model.RRoleMenuColumnMenuId).
		Where(model.RRoleMenuColumnRoleId, request.RoleId).
		Find(&bindings).Error
	if err != nil {
		slog.Warn("list role menus failed", "role_id", request.RoleId, "error", err)
		return nil, err
	}
	menuIds := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		menuIds = append(menuIds, cast.ToString(binding.MenuId))
	}
	return &iam_v1.ListRoleMenusResponse{MenuIds: menuIds}, nil
}
