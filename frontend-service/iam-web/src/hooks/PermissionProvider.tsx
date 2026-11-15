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
  const permissionCodes = useMemo(
    () => new Set(query.data?.permissionCodeList ?? []),
    [query.data],
  );

  const value = useMemo(
    () => ({
      permissionCodes,
      loading: query.isPending,
      // 权限码判定：快照驱动；未登记的码永不命中（fail-closed）。
      // 缺省（无码要求）放行由调用方显式传 undefined 表达。
      hasPermission: (code?: string) =>
        !query.isPending && (!code || permissionCodes.has(code)),
      reload: () => queryClient.invalidateQueries({ queryKey: ['permissions', 'my'] }),
    }),
    [permissionCodes, query.isPending, queryClient],
  );

  return <PermissionContext.Provider value={value}>{children}</PermissionContext.Provider>;
}
