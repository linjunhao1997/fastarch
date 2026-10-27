import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo } from 'react';
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

  // Set 与 context value 均稳定化，避免每次渲染令全部 usePermissions 消费者重渲染
  const menuKeys = useMemo(() => new Set(query.data?.menuKeyList ?? []), [query.data]);
  const httpFullMethods = useMemo(
    () => new Set(query.data?.httpFullMethodList ?? []),
    [query.data],
  );

  const value = useMemo(
    () => ({
      menuKeys,
      httpFullMethods,
      loading: query.isPending,
      // 菜单级判定：menu_key_list 驱动（宪法 XIII 无权限可见，而非空白）。
      // 加载中未持权限返回 false，由路由守卫展示加载态。
      hasMenuPermission: (menuKey?: string) =>
        !query.isPending && (!menuKey || menuKeys.has(menuKey)),
      // 按钮级判定：http_full_method_list 驱动；加载中同样不放行入口。
      hasHttpPermission: (fullMethod?: string) =>
        !query.isPending && (!fullMethod || httpFullMethods.has(fullMethod)),
      reload: () => queryClient.invalidateQueries({ queryKey: ['permissions', 'my'] }),
    }),
    [menuKeys, httpFullMethods, query.isPending, queryClient],
  );

  return <PermissionContext.Provider value={value}>{children}</PermissionContext.Provider>;
}
