import { createContext } from 'react';
import { useContext } from 'react';

/**
 * 权限码判定上下文（fastarch_dev 重构 #4 裁决：权限码是唯一授权词汇）。
 *   - hasPermission  按钮/入口级判定，数据源 permissionCodeList（服务端快照），
 *                    驱动页面内操作入口显隐（宪法 XI 入口即行动）与路由直访守卫；
 *   - 菜单可见性不在此判定：由服务端 my-menu-tree 按角色菜单绑定过滤后下发。
 * loading 期间判定一律返回 false，路由守卫据此展示加载态而非误判重定向。
 */
export interface PermissionContextValue {
  permissionCodes: ReadonlySet<string>;
  loading: boolean;
  hasPermission: (code?: string) => boolean;
  reload: () => Promise<void>;
}

export const PermissionContext = createContext<PermissionContextValue | undefined>(undefined);

export function usePermissions(): PermissionContextValue {
  const context = useContext(PermissionContext);
  if (!context) throw new Error('usePermissions must be used within PermissionProvider');
  return context;
}
