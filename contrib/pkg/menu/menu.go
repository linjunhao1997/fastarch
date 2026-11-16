package menu

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	common_menu_v1 "github.com/linjunhao1997/idl/pb/common/menu"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/linjunhao1997/contrib/model"
	"github.com/linjunhao1997/contrib/pkg/bizerr"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// 通用菜单能力（ADR 0001）：每服务嵌入自持，契约共享（iam.v1.MenuService）。
// 宿主差异经 Config 钩子注入：应用上下文、账号/管理员身份、角色绑定、
// 权限码校验（授权词汇规则归宿主）。菜单不参与授权（ADR 0002）：
// 角色绑定仅决定可见性，permission_code 为前端展示门槛，绑定变更不产码。

// 业务错误（业务码与既有客户端约定保持不变）。
// 权限码格式/服务段等授权词汇规则归宿主（ValidatePermissionCode 钩子），
// 本包不定义码校验错误。
var (
	ErrMenuNotFound    = bizerr.NewErrorWithCode("1001047", codes.NotFound, errors.New("菜单不存在"))
	ErrMenuHasChildren = bizerr.NewErrorWithCode("1001050", codes.FailedPrecondition, errors.New("菜单存在子菜单，请先删除子菜单"))
	ErrMenuReferenced  = bizerr.NewErrorWithCode("1001051", codes.FailedPrecondition, errors.New("菜单已被角色绑定，请先解除绑定"))
	ErrUpdateMenuBusy  = bizerr.NewErrorWithCode("1001052", codes.FailedPrecondition, errors.New("菜单已被他人修改，请刷新后重试"))
)

// Config 是菜单能力的宿主接入配置。必填钩子缺省时 NewMenuService 报错（fail-closed）。
type Config struct {
	DB *gorm.DB

	// ApplicationIdFromContext 返回当前请求的应用上下文 id；未选应用时返回错误。
	ApplicationIdFromContext func(ctx context.Context) (int64, error)
	// AccountIdFromContext 返回当前登录账号 id（无登录态返回 0）。
	AccountIdFromContext func(ctx context.Context) int64
	// IsAdminFromContext 返回当前登录者是否管理员（管理员取全量启用菜单）。
	IsAdminFromContext func(ctx context.Context) bool
	// RoleIdsForAccount 返回账号绑定的角色 id 集合（非管理员可见菜单过滤用）。
	RoleIdsForAccount func(ctx context.Context, accountId int64) ([]int64, error)
	// RoleExists 校验角色存在（角色绑定菜单前）；false = 不存在。
	RoleExists func(ctx context.Context, roleId int64) (bool, error)
	// ValidatePermissionCode 可选：校验菜单展示码（格式/服务段隔离等宿主规则）。
	// 码语义为前端展示门槛（用户持有才显示该菜单），不授予权限（ADR 0002）。
	ValidatePermissionCode func(ctx context.Context, appId int64, code string) error
}

type MenuService struct {
	cfg Config
}

var _ iam_v1.MenuServiceServer = (*MenuService)(nil)

// NewMenuService 构建通用菜单服务；必填钩子缺失时返回错误。
func NewMenuService(cfg Config) (*MenuService, error) {
	if cfg.DB == nil {
		return nil, errors.New("menu: DB is required")
	}
	for name, hook := range map[string]func(ctx context.Context) (int64, error){
		"ApplicationIdFromContext": cfg.ApplicationIdFromContext,
	} {
		if hook == nil {
			return nil, errors.New("menu: hook " + name + " is required")
		}
	}
	if cfg.AccountIdFromContext == nil || cfg.IsAdminFromContext == nil ||
		cfg.RoleIdsForAccount == nil || cfg.RoleExists == nil {
		return nil, errors.New("menu: hooks AccountIdFromContext/IsAdminFromContext/RoleIdsForAccount/RoleExists are required")
	}
	return &MenuService{cfg: cfg}, nil
}

// menuToPb 模型转 pb（t_menu 全列映射）。
func menuToPb(m model.Menu) *common_menu_v1.Menu {
	return &common_menu_v1.Menu{
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
func menuTreeNodes(menus []model.Menu) []*common_menu_v1.MenuTreeNode {
	nodesById := make(map[int64]*common_menu_v1.MenuTreeNode, len(menus))
	for _, menu := range menus {
		nodesById[menu.Id] = &common_menu_v1.MenuTreeNode{
			Menu: menuToPb(menu),
		}
	}
	roots := make([]*common_menu_v1.MenuTreeNode, 0)
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

// loadVisibleMenus 按账号角色取可见菜单。
// 管理员取全量启用菜单；普通用户取角色绑定菜单并回补祖先路径，保证树结构完整。
// appId 为当前应用上下文：仅返回该应用下的菜单。
func loadVisibleMenus(ctx context.Context, db *gorm.DB, cfg Config, accountId int64, isAdmin bool, appId int64) []model.Menu {
	var menus []model.Menu
	query := db.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnStatus, model.StatusEnabled).
		Where(model.MenuColumnApplicationId, appId)
	if !isAdmin {
		roleIds, err := cfg.RoleIdsForAccount(ctx, accountId)
		if err != nil {
			slog.Warn("load account role ids failed", "account_id", accountId, "error", err)
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
	return fillMenuAncestors(ctx, db, menus, appId)
}

// fillMenuAncestors 把菜单集合缺失的祖先菜单并入返回集合。
// 一次性载入全量启用菜单后纯内存回溯（菜单树规模有限），避免循环内点查。
func fillMenuAncestors(ctx context.Context, db *gorm.DB, menus []model.Menu, appId int64) []model.Menu {
	included := make(map[int64]struct{}, len(menus))
	for _, menu := range menus {
		included[menu.Id] = struct{}{}
	}
	var allMenus []model.Menu
	err := db.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnStatus, model.StatusEnabled).
		Where(model.MenuColumnApplicationId, appId).
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
// 数据范围 = 当前应用上下文（宿主 token 承载）。
func (s *MenuService) ListMenuTree(ctx context.Context, empty *emptypb.Empty) (*common_menu_v1.ListMenuTreeResponse, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var menus []model.Menu
	err = s.cfg.DB.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnApplicationId, appId).
		Order(model.MenuColumnSort + " ASC").
		Find(&menus).Error
	if err != nil {
		slog.Warn("list menu tree failed", "error", err)
		return nil, err
	}
	return &common_menu_v1.ListMenuTreeResponse{
		Roots: menuTreeNodes(menus),
	}, nil
}

// ListMyMenuTree 当前登录用户可见菜单树：管理员取当前应用全量启用菜单，
// 普通用户按角色绑定过滤并回补祖先路径（应用上下文由宿主 token 承载）。
func (s *MenuService) ListMyMenuTree(ctx context.Context, empty *emptypb.Empty) (*common_menu_v1.ListMenuTreeResponse, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	isAdmin := s.cfg.IsAdminFromContext(ctx)
	accountId := s.cfg.AccountIdFromContext(ctx)
	menus := loadVisibleMenus(ctx, s.cfg.DB, s.cfg, accountId, isAdmin, appId)
	return &common_menu_v1.ListMenuTreeResponse{
		Roots: menuTreeNodes(menus),
	}, nil
}

// validateMenuPermissionCode 菜单绑定码经宿主规则校验（钩子缺省时跳过）。
func (s *MenuService) validateMenuPermissionCode(ctx context.Context, appId int64, code string) error {
	if s.cfg.ValidatePermissionCode == nil {
		return nil
	}
	return s.cfg.ValidatePermissionCode(ctx, appId, code)
}

// CreateMenu 创建菜单，返回创建出的菜单（含 id）。归属应用取当前上下文；
// 绑定权限码时经宿主规则校验。
func (s *MenuService) CreateMenu(ctx context.Context, request *common_menu_v1.CreateMenuRequest) (*common_menu_v1.Menu, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if len(request.PermissionCode) > 0 {
		if err := s.validateMenuPermissionCode(ctx, appId, request.PermissionCode); err != nil {
			return nil, err
		}
	}
	pkId := krand.NewID()
	now := time.Now()
	operator := cast.ToString(s.cfg.AccountIdFromContext(ctx))
	status := request.Status
	if status == "" {
		status = model.StatusEnabled
	}
	menu := model.Menu{
		Id:             pkId,
		ParentId:       request.ParentId,
		ApplicationId:  appId,
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
	if err := s.cfg.DB.WithContext(ctx).Select(model.MenuFields).Create(&menu).Error; err != nil {
		slog.Warn("create menu failed", "menu_name", request.Name, "error", err)
		return nil, err
	}
	return menuToPb(menu), nil
}

// GetMenu 获取菜单信息（限当前应用上下文，跨应用按不存在处理）。
func (s *MenuService) GetMenu(ctx context.Context, request *common_v1.GetRequest) (*common_menu_v1.Menu, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var menu model.Menu
	err = s.cfg.DB.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnId, request.Id).
		Where(model.MenuColumnApplicationId, appId).
		First(&menu).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	return menuToPb(menu), nil
}

// UpdateMenu 修改菜单：字段缺省 = 不修改；parent_id 默认不改，
// 仅在 parent_cleared = true 时按传入值整体移动（含挂到顶级）。
func (s *MenuService) UpdateMenu(ctx context.Context, request *common_menu_v1.UpdateMenuRequest) (*emptypb.Empty, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var menu model.Menu
	err = s.cfg.DB.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnId, request.Id).
		Where(model.MenuColumnApplicationId, appId).
		First(&menu).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	if len(request.PermissionCode) > 0 {
		if err := s.validateMenuPermissionCode(ctx, appId, request.PermissionCode); err != nil {
			return nil, err
		}
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
	// 审计字段服务端注入：仅更新修改人与修改时间
	updates[model.MenuColumnUpdatedBy] = cast.ToString(s.cfg.AccountIdFromContext(ctx))
	updates[model.MenuColumnVersion] = gorm.Expr(model.MenuColumnVersion + " + 1")

	result := s.cfg.DB.WithContext(ctx).Model(model.Menu{}).
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

// DeleteMenu 删除菜单：限当前应用上下文；子菜单与角色绑定引用检查，
// 物理删除（基础架构型）。
func (s *MenuService) DeleteMenu(ctx context.Context, request *common_menu_v1.DeleteMenuRequest) (*emptypb.Empty, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var childCount int64
	err = s.cfg.DB.WithContext(ctx).Model(model.Menu{}).
		Where(model.MenuColumnParentId, request.Id).
		Where(model.MenuColumnApplicationId, appId).
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
	err = s.cfg.DB.WithContext(ctx).Model(model.RRoleMenu{}).
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

	result := s.cfg.DB.WithContext(ctx).
		Where(model.MenuColumnId, request.Id).
		Where(model.MenuColumnApplicationId, appId).
		Delete(model.Menu{})
	if result.Error != nil {
		slog.Warn("delete menu failed", "menu_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete menu rejected: menu not found", "menu_id", request.Id)
		return nil, ErrMenuNotFound
	}
	return &emptypb.Empty{}, nil
}

// UpdateRoleMenus 角色绑定菜单全量覆盖：以 menu_ids 为目标集合做整体替换
// （差集增量写，避免清空重写引发无谓行变更）。限当前应用上下文：目标菜单
// 必须全部归属当前应用（跨应用按不存在处理），差集只增删当前应用内的绑定，
// 其他应用的绑定不受影响。角色存在性经宿主钩子校验。
// 绑定仅决定菜单可见性，不授予权限（ADR 0002）。
func (s *MenuService) UpdateRoleMenus(ctx context.Context, request *common_menu_v1.UpdateRoleMenusRequest) (*emptypb.Empty, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
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

	// 菜单必须全部存在且归属当前应用（避免静默丢弃非法 id / 跨应用绑定）
	if len(menuIds) > 0 {
		var count int64
		err := s.cfg.DB.WithContext(ctx).Model(model.Menu{}).
			Where(model.MenuColumnId+" IN ?", menuIds).
			Where(model.MenuColumnApplicationId, appId).
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

	exists, err := s.cfg.RoleExists(ctx, request.RoleId)
	if err != nil {
		slog.Warn("check role for menu binding failed", "role_id", request.RoleId, "error", err)
		return nil, err
	}
	if !exists {
		return nil, ErrRoleNotFound
	}

	boundMenuIds, err := s.listRoleMenuIds(ctx, request.RoleId, appId)
	if err != nil {
		return nil, err
	}

	// 与当前绑定求差集，只增删变化部分
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
	removedMenuIds := make([]int64, 0, len(boundMenuIds))
	for id := range boundMenuIds {
		if _, exists := targetSet[id]; !exists {
			removedMenuIds = append(removedMenuIds, id)
		}
	}

	// 菜单绑定只写关联表 t_r_role_menu，绝不触碰 menu 行；
	// 不用 Association.Append——传入 stub model.Menu{Id} 会连带 upsert menu 行，
	// 零值 CreateTime/UpdateTime 触发 MySQL "Incorrect datetime value: '0000-00-00'"。
	err = s.cfg.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(addedMenuIds) > 0 {
			rows := make([]model.RRoleMenu, 0, len(addedMenuIds))
			for _, id := range addedMenuIds {
				rows = append(rows, model.RRoleMenu{RoleId: request.RoleId, MenuId: id})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		if len(removedMenuIds) > 0 {
			if err := tx.Where(model.RRoleMenuColumnRoleId+" = ? AND "+model.RRoleMenuColumnMenuId+" IN ?",
				request.RoleId, removedMenuIds).Delete(&model.RRoleMenu{}).Error; err != nil {
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
	return &emptypb.Empty{}, nil
}

// ErrRoleNotFound 角色不存在（宿主钩子判定）。
var ErrRoleNotFound = bizerr.NewErrorWithCode("1001010", codes.InvalidArgument, errors.New("角色不存在"))

// listRoleMenuIds 查询角色已绑定的菜单 id 集合，限当前应用上下文。
// 关联表无应用列，归属经菜单行的 application_id 过滤（绑定引用检查保证
// 被绑菜单必然存在）；跨应用绑定不进入集合，差集计算才不会误删其他应用绑定。
func (s *MenuService) listRoleMenuIds(ctx context.Context, roleId int64, appId int64) (map[int64]struct{}, error) {
	var menuIds []int64
	err := s.cfg.DB.WithContext(ctx).Model(model.RRoleMenu{}).
		Where(model.RRoleMenuColumnRoleId, roleId).
		Distinct().
		Pluck(model.RRoleMenuColumnMenuId, &menuIds).Error
	if err != nil {
		slog.Warn("list role menus failed", "role_id", roleId, "error", err)
		return nil, err
	}
	bound := make(map[int64]struct{}, len(menuIds))
	if len(menuIds) == 0 {
		return bound, nil
	}
	var appMenuIds []int64
	err = s.cfg.DB.WithContext(ctx).Model(model.Menu{}).
		Where(model.MenuColumnId+" IN ?", menuIds).
		Where(model.MenuColumnApplicationId, appId).
		Pluck(model.MenuColumnId, &appMenuIds).Error
	if err != nil {
		slog.Warn("filter role menus by application failed", "role_id", roleId, "application_id", appId, "error", err)
		return nil, err
	}
	for _, id := range appMenuIds {
		bound[id] = struct{}{}
	}
	return bound, nil
}

// ListRoleMenus 查询角色绑定的菜单 id 集合（编辑角色页回显用，限当前应用上下文）。
func (s *MenuService) ListRoleMenus(ctx context.Context, request *common_menu_v1.ListRoleMenusRequest) (*common_menu_v1.ListRoleMenusResponse, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	bound, err := s.listRoleMenuIds(ctx, request.RoleId, appId)
	if err != nil {
		return nil, err
	}
	menuIds := make([]string, 0, len(bound))
	for id := range bound {
		menuIds = append(menuIds, cast.ToString(id))
	}
	return &common_menu_v1.ListRoleMenusResponse{MenuIds: menuIds}, nil
}
