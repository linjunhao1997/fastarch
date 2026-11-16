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

// 通用菜单能力（ADR 0001）：每服务嵌入自持，service 定义每服务自持、消息共享
// common.menu.v1（ADR 0003）。宿主差异经 Config 钩子注入：应用上下文、账号身份、
// 权限码校验（授权词汇规则归宿主）。
//
// 菜单为纯展示配置（ADR 0002 + 纯码门槛裁决）：permission_code 是前端展示门槛
// （用户持有才显示该菜单），不授予权限。菜单可见性由前端按用户快照码过滤，
// 服务端 my-menu-tree 只按应用上下文返回启用菜单，不做角色绑定过滤
// （t_r_role_menu 已退役）。

// 业务错误（业务码与既有客户端约定保持不变）。
// 权限码格式/服务段等授权词汇规则归宿主（ValidatePermissionCode 钩子），
// 本包不定义码校验错误。
var (
	ErrMenuNotFound    = bizerr.NewErrorWithCode("1001047", codes.NotFound, errors.New("菜单不存在"))
	ErrMenuHasChildren = bizerr.NewErrorWithCode("1001050", codes.FailedPrecondition, errors.New("菜单存在子菜单，请先删除子菜单"))
	ErrUpdateMenuBusy  = bizerr.NewErrorWithCode("1001052", codes.FailedPrecondition, errors.New("菜单已被他人修改，请刷新后重试"))
)

// Config 是菜单能力的宿主接入配置。必填钩子缺省时 NewMenuService 报错（fail-closed）。
type Config struct {
	DB *gorm.DB

	// ApplicationIdFromContext 返回当前请求的应用上下文 id；未选应用时返回错误。
	ApplicationIdFromContext func(ctx context.Context) (int64, error)
	// AccountIdFromContext 返回当前登录账号 id（无登录态返回 0；审计字段用）。
	AccountIdFromContext func(ctx context.Context) int64
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
	if cfg.ApplicationIdFromContext == nil {
		return nil, errors.New("menu: hook ApplicationIdFromContext is required")
	}
	if cfg.AccountIdFromContext == nil {
		return nil, errors.New("menu: hook AccountIdFromContext is required")
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

// ListMyMenuTree 当前应用的全量启用菜单树（纯码门槛裁决）：服务端不做角色过滤，
// 返回当前应用所有启用菜单；可见性由前端按菜单 permission_code 与用户快照码过滤。
// admin 与否服务端不区分（admin 的全量码由 GetMyPermissions 保证，前端门槛自然放行）。
func (s *MenuService) ListMyMenuTree(ctx context.Context, empty *emptypb.Empty) (*common_menu_v1.ListMenuTreeResponse, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var menus []model.Menu
	err = s.cfg.DB.WithContext(ctx).
		Select(model.MenuFields).
		Where(model.MenuColumnApplicationId, appId).
		Where(model.MenuColumnStatus, model.StatusEnabled).
		Order(model.MenuColumnSort + " ASC").
		Find(&menus).Error
	if err != nil {
		slog.Warn("list my menu tree failed", "error", err)
		return nil, err
	}
	return &common_menu_v1.ListMenuTreeResponse{
		Roots: menuTreeNodes(menus),
	}, nil
}

// validateMenuPermissionCode 菜单展示码经宿主规则校验（钩子缺省时跳过）。
func (s *MenuService) validateMenuPermissionCode(ctx context.Context, appId int64, code string) error {
	if s.cfg.ValidatePermissionCode == nil {
		return nil
	}
	return s.cfg.ValidatePermissionCode(ctx, appId, code)
}

// CreateMenu 创建菜单，返回创建出的菜单（含 id）。归属应用取当前上下文；
// 展示码经宿主规则校验。
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

// DeleteMenu 删除菜单：限当前应用上下文；子菜单引用检查，物理删除（基础架构型）。
// 角色绑定已退役（纯码门槛），不再检查绑定引用。
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
