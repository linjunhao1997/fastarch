import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { authApi } from '../api/iam';
import type { MyProfileApplication } from '../types';
import { createContext, useContext } from 'react';

const STORAGE_KEY = 'current_application_id';

/**
 * 应用上下文（多应用裁决 #8/#9）：
 *   - 可进入应用清单来自 ListMyApplications（admin 全量启用应用；其他按准入绑定）；
 *   - "当前应用"是服务端概念：token(jti) → 应用映射存 Redis，切换经
 *     SelectApplication 覆写映射、不重签令牌；本地仅缓存选择偏好（localStorage）；
 *   - 每次挂载都重新绑定（映射随令牌过期失效，页面刷新后必须重放）；
 *   - 切换成功后失效全部业务查询缓存，各页按新应用上下文重取。
 */
export interface ApplicationContextValue {
  applications: MyProfileApplication[];
  currentApplicationId: string | null;
  currentApplication: MyProfileApplication | null;
  /** 切换当前应用（服务端绑定成功后才生效并刷新数据） */
  setCurrentApplicationId: (id: string) => Promise<void>;
  /** 应用清单加载中 */
  loading: boolean;
  /** 应用清单加载失败 */
  error: unknown;
  refetch: () => Promise<unknown>;
}

export const ApplicationContext = createContext<ApplicationContextValue | undefined>(undefined);

export function ApplicationProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ['my-applications'],
    queryFn: authApi.listMyApplications,
    staleTime: 5 * 60_000,
    retry: 1,
  });

  const [currentApplicationId, setCurrentApplicationIdState] = useState<string | null>(() =>
    localStorage.getItem(STORAGE_KEY),
  );
  // 防止重复绑定：同一 (应用清单版本, 目标应用) 只发一次 SelectApplication
  const boundRef = useRef<string>('');

  const applications = useMemo(() => query.data?.applications ?? [], [query.data]);

  const bindOnServer = useCallback(
    async (app: MyProfileApplication) => {
      const fingerprint = `${app.id}:${applications.length}:${applications.map((a) => a.id).join(',')}`;
      if (boundRef.current === fingerprint) return;
      await authApi.selectApplication(app.code);
      boundRef.current = fingerprint;
      setCurrentApplicationIdState(app.id);
      localStorage.setItem(STORAGE_KEY, app.id);
      // 绑定成功后业务数据（菜单树等）都依赖新应用上下文，全量失效重取
      void queryClient.invalidateQueries();
    },
    [applications, queryClient],
  );

  // 清单到位后：恢复既有选择（仍有效则重放绑定），失效则回退第一个可用应用
  useEffect(() => {
    if (query.isPending || applications.length === 0) return;
    const stored = applications.find((app) => app.id === currentApplicationId && app.status === 'enabled');
    const target = stored ?? applications.find((app) => app.status === 'enabled') ?? applications[0];
    void bindOnServer(target).catch(() => {
      // 绑定失败（网络/准入变化）：清掉本地偏好，下次挂载重试
      boundRef.current = '';
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [applications, query.isPending]);

  const setCurrentApplicationId = useCallback(
    async (id: string) => {
      const app = applications.find((a) => a.id === id);
      if (!app) return;
      boundRef.current = '';
      await bindOnServer(app);
    },
    [applications, bindOnServer],
  );

  const value = useMemo<ApplicationContextValue>(
    () => ({
      applications,
      currentApplicationId,
      currentApplication: applications.find((app) => app.id === currentApplicationId) ?? null,
      setCurrentApplicationId,
      loading: query.isPending,
      error: query.error,
      refetch: () => query.refetch(),
    }),
    [applications, currentApplicationId, setCurrentApplicationId, query.isPending, query.error, query],
  );

  return <ApplicationContext.Provider value={value}>{children}</ApplicationContext.Provider>;
}

export function useApplicationContext(): ApplicationContextValue {
  const context = useContext(ApplicationContext);
  if (!context) throw new Error('useApplicationContext must be used within ApplicationProvider');
  return context;
}
