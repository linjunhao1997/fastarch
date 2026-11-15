/**
 * 字典类型（t_dict_type）：code 唯一；状态 enabled/disabled。
 * id 一律 string 编码（雪花 ID 惯例），version 为乐观锁。
 */
export interface DictType {
  id: string;
  code: string;
  name: string;
  remark: string;
  status: string;
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

/** 字典条目（t_dict_item）：按 type_id 归属；sort 控制顺序。 */
export interface DictItem {
  id: string;
  typeId: string;
  label: string;
  value: string;
  sort: number;
  status: string;
  version: number;
  createdBy: string;
  updatedBy: string;
  createTime: string;
  updateTime: string;
}

export interface CreateDictTypeRequest {
  code: string;
  name: string;
  remark?: string;
  status?: string;
}

export interface UpdateDictTypeRequest {
  name?: string;
  remark?: string;
  status?: string;
  version: number;
}

export interface CreateDictItemRequest {
  typeId: string;
  label: string;
  value: string;
  sort?: number;
  status?: string;
}

export interface UpdateDictItemRequest {
  label?: string;
  value?: string;
  sort?: number;
  status?: string;
  version: number;
}

export interface ListDictTypesResponse {
  page: import('../api/request').Page;
  dictTypes: DictType[];
}

export interface ListDictItemsResponse {
  page: import('../api/request').Page;
  dictItems: DictItem[];
}
