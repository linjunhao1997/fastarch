import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { profileApi } from '../api/iam';
import type { MyProfile, MyProfileApplication } from '../types';
import { createContext, useContext } from 'react';

const STORAGE_KEY = 'current_application_id';

/**
 * 应用上下文（specs/008-account-app-context US3/US4）：
 *   - 持有当前用户所属应用列表（数据源 GetMyProfile）与当前选中应用 id；
 *   - 选择持久化 localStorage，恢复前校验"仍在列表且未停用"，
 *     失效回退列表第一个可用应用；无归属时为空态（currentApplicationId=null）；
 *   - 菜单可见性与"创建默认应用"均以本上下文的 currentApplicationId 为准。
 */
export interface ApplicationContextValue {
  profile: MyProfile | null;
  applications: MyProfileApplication[];
  currentApplicationId: string | null;
  currentApplication: MyProfileApplication | null;
  /** 切换当前应用（持久化到 localStorage） */
  setCurrentApplicationId: (id: string) => void;
  /** 个人信息加载中（应用列表尚不可用） */
  loading: boolean;
  /** 个人信息加载失败（不静默进入无上下文状态） */
  error: unknown;
  refetch: () => Promise<unknown>;
}

export const ApplicationContext = createContext<ApplicationContextValue | undefined>(undefined);

export function ApplicationProvider({ children }: { children: ReactNode }) {
  const query = useQuery({
    queryKey: ['profile', 'my'],
    queryFn: profileApi.get,
    staleTime: 10 * 60_000,
    retry: 1,
  });

  const [currentApplicationId, setCurrentApplicationIdState] = useState<string | null>(() =>
    localStorage.getItem(STORAGE_KEY),
  );

  const applications = useMemo(() => query.data?.applications ?? [], [query.data]);

  // 恢复校验与回退：应用列表到位后校验既有选择，失效回退第一个可用应用
  useEffect(() => {
    if (query.isPending) return;
    if (applications.length === 0) {
      if (currentApplicationId !== null) {
        setCurrentApplicationIdState(null);
        localStorage.removeItem(STORAGE_KEY);
      }
      return;
    }
    const current = applications.find((app) => app.id === currentApplicationId);
    if (current && current.status === 'enabled') return;
    const fallback = applications.find((app) => app.status === 'enabled') ?? applications[0];
    setCurrentApplicationIdState(fallback.id);
    localStorage.setItem(STORAGE_KEY, fallback.id);
  }, [applications, query.isPending, currentApplicationId]);

  const setCurrentApplicationId = (id: string) => {
    setCurrentApplicationIdState(id);
    localStorage.setItem(STORAGE_KEY, id);
  };

  const value = useMemo<ApplicationContextValue>(
    () => ({
      profile: query.data ?? null,
      applications,
      currentApplicationId,
      currentApplication: applications.find((app) => app.id === currentApplicationId) ?? null,
      setCurrentApplicationId,
      loading: query.isPending,
      error: query.error,
      refetch: () => query.refetch(),
    }),
    // setCurrentApplicationId 稳定，不依赖 state
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [query.data, query.isPending, query.error, applications, currentApplicationId],
  );

  return <ApplicationContext.Provider value={value}>{children}</ApplicationContext.Provider>;
}

export function useApplicationContext(): ApplicationContextValue {
  const context = useContext(ApplicationContext);
  if (!context) throw new Error('useApplicationContext must be used within ApplicationProvider');
  return context;
}
