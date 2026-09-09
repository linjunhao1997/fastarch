import { createContext } from 'react';
import { useContext } from 'react';

/**
 * 双判定权限上下文（specs/005-typed-permission-model T038）：
 *   - hasMenuPermission  菜单级判定，数据源 menuKeyList，供导航过滤与路由直访守卫；
 *   - hasHttpPermission  按钮级判定，数据源 httpFullMethodList，供 PERMISSIONS 常量
 *                        驱动页面内操作入口显隐（宪法 XI 入口即行动）。
 * loading 期间判定一律返回 false，路由守卫据此展示加载态而非误判重定向。
 */
export interface PermissionContextValue {
  menuKeys: ReadonlySet<string>;
  httpFullMethods: ReadonlySet<string>;
  loading: boolean;
  hasMenuPermission: (menuKey?: string) => boolean;
  hasHttpPermission: (fullMethod?: string) => boolean;
  reload: () => Promise<void>;
}

export const PermissionContext = createContext<PermissionContextValue | undefined>(undefined);

export function usePermissions(): PermissionContextValue {
  const context = useContext(PermissionContext);
  if (!context) throw new Error('usePermissions must be used within PermissionProvider');
  return context;
}
