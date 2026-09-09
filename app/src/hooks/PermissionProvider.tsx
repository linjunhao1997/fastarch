import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { getMyPermissions } from '../api/iam';
import { PermissionContext } from './permission-context';

export function PermissionProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ['permissions', 'my'],
    queryFn: getMyPermissions,
    staleTime: 30_000,
    retry: 1,
  });

  const menuKeys = new Set(query.data?.menuKeyList ?? []);
  const httpFullMethods = new Set(query.data?.httpFullMethodList ?? []);
  const loading = query.isPending;

  // 菜单级判定：menu_key_list 驱动（宪法 XIII 无权限可见，而非空白）。
  // 加载中未持权限返回 false，由路由守卫展示加载态。
  const hasMenuPermission = (menuKey?: string) => !loading && (!menuKey || menuKeys.has(menuKey));
  // 按钮级判定：http_full_method_list 驱动；加载中同样不放行入口。
  const hasHttpPermission = (fullMethod?: string) => !loading && (!fullMethod || httpFullMethods.has(fullMethod));

  const reload = async () => {
    await queryClient.invalidateQueries({ queryKey: ['permissions', 'my'] });
  };

  return (
    <PermissionContext.Provider value={{ menuKeys, httpFullMethods, loading, hasMenuPermission, hasHttpPermission, reload }}>
      {children}
    </PermissionContext.Provider>
  );
}
