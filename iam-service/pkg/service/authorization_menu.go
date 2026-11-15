package service

import (
	"context"
	"log/slog"

	"iam-service/pkg/auth"
	"iam-service/pkg/model"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

var _ iam_v1.AuthorizationServiceServer = (*AuthorizationService)(nil)

func NewAuthorizationService(db *gorm.DB, rdb *redis.Client) *AuthorizationService {
	return &AuthorizationService{db: db, rdb: rdb}
}

// menuToResourcePb 把菜单模型映射到 proto Resource（组件资源模型已废弃，
// proto 的 ListMyMenuTree 仍复用 ResourceTreeNode 承载菜单树）。
func menuToResourcePb(m model.Menu) *iam_v1.Resource {
	permissionCodes := []string{}
	if len(m.PermissionCode) > 0 {
		permissionCodes = []string{m.PermissionCode}
	}
	return &iam_v1.Resource{
		Id:                 cast.ToString(m.Id),
		Kind:               "menu",
		Key:                m.Path,
		Name:               m.Name,
		EnabledStatus:      m.Status,
		CreateTime:         timestamppb.New(m.CreateTime),
		UpdateTime:         timestamppb.New(m.UpdateTime),
		ParentId:           m.ParentId,
		Version:            m.Version,
		DisplayOrder:       m.Sort,
		PermissionCodeList: permissionCodes,
	}
}

// menuTreeNodes 按 parent_id 组装 ResourceTreeNode 树，根节点为父节点不在
// 集合内的菜单（祖先已回补，根节点天然是顶级菜单）。
func menuTreeNodes(menus []model.Menu) []*iam_v1.ResourceTreeNode {
	nodesById := make(map[int64]*iam_v1.ResourceTreeNode, len(menus))
	for _, menu := range menus {
		nodesById[menu.Id] = &iam_v1.ResourceTreeNode{
			Resource: menuToResourcePb(menu),
		}
	}
	roots := make([]*iam_v1.ResourceTreeNode, 0)
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

// ListMyMenuTree 返回当前认证账号可见的菜单树（011 模型：t_menu + t_r_role_menu）。
// proto 复用 ResourceTreeNode 承载菜单树；ApplicationId 参数随应用归属模型删除，忽略。
func (a *AuthorizationService) ListMyMenuTree(ctx context.Context, request *iam_v1.ListMyMenuTreeRequest) (*iam_v1.ListResourceTreeResponse, error) {
	isAdmin := auth.IsAdminFromContext(ctx)
	accountId := auth.GetAccountId(ctx)
	menus := loadVisibleMenus(ctx, a.db, accountId, isAdmin)
	return &iam_v1.ListResourceTreeResponse{
		Roots: menuTreeNodes(menus),
	}, nil
}
