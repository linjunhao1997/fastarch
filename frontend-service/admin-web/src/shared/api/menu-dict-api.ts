/**
 * 菜单 / 字典 API（跨 services 共享，动态前缀）。
 *
 * 管理员切换应用后，菜单/字典配置写入对应服务的库：
 *   - 当前应用 iam    → /iam/menu/...、/iam/dict/...
 *   - 当前应用 sender → /sender/menu/...、/sender/dict/...
 * 前缀取自 shared/api/app-context，与当前应用绑定状态同步。
 */
import { request, type RequestOptions } from './request';
import { appApiPrefix } from './app-context';
import type {
  CreateDictItemRequest,
  CreateDictTypeRequest,
  CreateMenuRequest,
  DictItem,
  DictType,
  ListDictItemsResponse,
  ListDictTypesResponse,
  ListMenuTreeResponse,
  Menu,
  UpdateDictItemRequest,
  UpdateDictTypeRequest,
  UpdateMenuRequest,
} from '../types';

const buildListUrl = (base: string, currentPage: number, pageSize: number, keyword = '') => {
  const params = new URLSearchParams({
    current_page: String(currentPage),
    page_size: String(pageSize),
  });
  const trimmed = keyword.trim();
  if (trimmed) params.set('keyword', trimmed);
  return `${base}?${params.toString()}`;
};

/** 菜单（t_menu 语义，纯展示配置）：服务端下发结构与展示，前端按码门槛过滤可见性。 */
export const menuApi = {
  listMyTree: () => request<ListMenuTreeResponse>(`${appApiPrefix()}/menu/my-menu-tree`),
  /** 全量菜单树（管理员视角，含停用） */
  listTree: () => request<ListMenuTreeResponse>(`${appApiPrefix()}/menu/menu-tree`),
  create: (data: CreateMenuRequest) =>
    request<Menu>(`${appApiPrefix()}/menu/menus`, { method: 'POST', body: data }),
  get: (id: string) => request<Menu>(`${appApiPrefix()}/menu/menus/${id}`),
  update: (id: string, data: UpdateMenuRequest) =>
    request<void>(`${appApiPrefix()}/menu/menus/${id}`, {
      method: 'PATCH',
      body: data,
    } as RequestOptions),
  delete: (id: string) =>
    request<void>(`${appApiPrefix()}/menu/menus/${id}`, { method: 'DELETE' }),
};

/** 字典（t_dict_type / t_dict_item）：类型 code 唯一；条目按 type_id 归属。 */
export const dictApi = {
  createType: (data: CreateDictTypeRequest) =>
    request<DictType>(`${appApiPrefix()}/dict/types`, { method: 'POST', body: data }),
  getType: (id: string) => request<DictType>(`${appApiPrefix()}/dict/types/${id}`),
  updateType: (id: string, data: UpdateDictTypeRequest) =>
    request<void>(`${appApiPrefix()}/dict/types/${id}`, { method: 'PATCH', body: data }),
  deleteType: (id: string) =>
    request<void>(`${appApiPrefix()}/dict/types/${id}`, { method: 'DELETE' }),
  listTypes: (currentPage = 1, pageSize = 10, keyword = '') =>
    request<ListDictTypesResponse>(
      buildListUrl(`${appApiPrefix()}/dict/types`, currentPage, pageSize, keyword),
    ),
  createItem: (data: CreateDictItemRequest) =>
    request<DictItem>(`${appApiPrefix()}/dict/items`, { method: 'POST', body: data }),
  getItem: (id: string) => request<DictItem>(`${appApiPrefix()}/dict/items/${id}`),
  updateItem: (id: string, data: UpdateDictItemRequest) =>
    request<void>(`${appApiPrefix()}/dict/items/${id}`, { method: 'PATCH', body: data }),
  deleteItem: (id: string) =>
    request<void>(`${appApiPrefix()}/dict/items/${id}`, { method: 'DELETE' }),
  listItems: (typeId: string, currentPage = 1, pageSize = 50, keyword = '') =>
    request<ListDictItemsResponse>(
      `${buildListUrl(`${appApiPrefix()}/dict/items`, currentPage, pageSize, keyword)}&type_id=${typeId}`,
    ),
};
