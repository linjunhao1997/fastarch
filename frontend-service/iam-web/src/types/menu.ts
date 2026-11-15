/**
 * 菜单（t_menu 语义）：结构与展示均由服务端提供（path/icon/sort），
 * 可见性 = 角色菜单绑定（t_r_role_menu），服务端在 my-menu-tree 中已按登录用户过滤。
 * id/parentId 为 string 编码（雪花 ID 惯例）；permissionCode 可空 = 不绑定权限。
 */
export interface Menu {
  id: string;
  parentId: string;
  name: string;
  path: string;
  icon: string;
  sort: number;
  permissionCode: string;
  /** enabled 启用 / disabled 禁用 */
  status: string;
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface MenuTreeNode {
  menu: Menu;
  children: MenuTreeNode[];
}

export interface ListMenuTreeResponse {
  roots: MenuTreeNode[];
}

export interface CreateMenuRequest {
  parentId?: string;
  name: string;
  path?: string;
  icon?: string;
  sort?: number;
  permissionCode?: string;
  status?: string;
}

export interface UpdateMenuRequest {
  parentId?: string;
  name?: string;
  path?: string;
  icon?: string;
  sort?: number;
  permissionCode?: string;
  status?: string;
  version: number;
}

export interface UpdateRoleMenusRequest {
  roleId: string;
  menuIds: string[];
}
